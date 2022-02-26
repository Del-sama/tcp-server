package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"regexp"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	connType              = "tcp"
	port                  = "4000"
	maxConnections        = 5
	logFile               = "numbers.log"
	InputLength           = 9
	DefaultReportDuration = 10
	DefaultFlushDuration  = 30
)

var (
	totalUnique, duplicates, unique int32
	invalidNumberRegex              = regexp.MustCompile(`[^0-9]`)
)

type (
	rwCache struct {
		sync.RWMutex
		m map[string]string
	}

	server struct {
		addr             string
		server           net.Listener
		maxConnections   int32
		connectionsCount int32
		isTerminated     bool
	}
)

func main() {
	file, err := initializeFile(logFile)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		err = file.Close()
		if err != nil {
			log.Fatal(err)
		}
	}()

	// TODO:
	// - make bufio.Writer object is not thread safe
	// - fix issue where the size of the buffered data is bigger than the buffer used to write to the file;
	// 	either check before writing if there is space available or check for the error and split the data into multiple pieces that fit.
	// - clean up the goroutines of the open connections during graceful termination
	writer := bufio.NewWriter(file)
	cache := &rwCache{m: make(map[string]string)}

	server := newServer(":"+port, maxConnections, 0)

	// log report of unique and duplicates every 10 seconds
	go func(n time.Duration) {
		for range time.Tick(n) {
			atomic.AddInt32(&totalUnique, unique)
			logReport(unique, totalUnique, duplicates)
			atomic.StoreInt32(&unique, 0)
			atomic.StoreInt32(&duplicates, 0)
		}
	}(DefaultReportDuration * time.Second)

	// flush buffered data to log file every 30 seconds to improve performance. Flushin data more frequently reduces speed.
	go func(n time.Duration) {
		for range time.Tick(n) {
			if err := flushWriteBuffer(writer); err != nil {
				fmt.Printf("got error while flushing buffer: %v", err)
			}
		}
	}(DefaultFlushDuration * time.Second)

	server.run(writer, cache)
	log.Fatal(err)
}

func initializeFile(filename string) (*os.File, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func newServer(addr string, maxConnections int32, connectionsCount int32) *server {
	return &server{
		addr:             addr,
		maxConnections:   maxConnections,
		connectionsCount: connectionsCount,
	}
}

func logReport(unique, totalUnique, duplicates int32) {
	fmt.Printf("Received %d unique numbers, %d duplicates. Unique total: %d \n",
		unique, duplicates, totalUnique)
}

func isInvalidInput(input string) bool {
	return len(input) != InputLength || invalidNumberRegex.MatchString(input)
}

func isDuplicate(cache *rwCache, value string) bool {
	if cache.get(value) != "" {
		return true
	}
	return false
}

func (s *server) run(writer *bufio.Writer, cache *rwCache) error {
	var err error
	s.server, err = net.Listen(connType, s.addr)
	if err != nil {
		return err
	}
	fmt.Println("started TCP server")

	defer func() {
		if err = s.close(writer); err != nil {
			fmt.Printf("got error while closing server connection: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigChan
		if err = s.close(writer); err != nil {
			fmt.Printf("got error while closing server connection: %v", err)
		}
	}()

	for {
		conn, err := s.server.Accept()
		if err != nil {
			return err
		}

		if s.connectionsCount >= s.maxConnections {
			fmt.Println("Max concurrent connections reached")
			if err = conn.Close(); err != nil {
				fmt.Printf("got error `%v` while closing connection for client: %s", err, conn.RemoteAddr().String())
			}
			continue
		}
		fmt.Println("successfully connected")
		// handle connections
		go func() {
			err = s.handleConn(conn, writer, cache)
			if err != nil {
				fmt.Println(err)
			}
		}()
	}
}

func (s *server) close(writer *bufio.Writer) error {
	fmt.Println("Gracefully shutting down server...")
	fmt.Println("Flushing data to write buffer...")
	if !s.isTerminated {
		s.isTerminated = true
		if err := s.server.Close(); err != nil {
			_ = writer.Flush()
			return err
		}
		if err := writer.Flush(); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) handleConn(conn net.Conn, writer *bufio.Writer, cache *rwCache) error {
	atomic.AddInt32(&s.connectionsCount, 1)
	fmt.Printf("Now listening: %s \n", conn.RemoteAddr().String())
	err := s.handleContentFromConnection(conn, writer, cache)
	if err != nil {
		return errors.New(err.Error())
	}

	// close connection
	defer func(conn net.Conn) {
		atomic.AddInt32(&s.connectionsCount, -1)
		fmt.Printf("Closing connection for : %s \n", conn.RemoteAddr().String())
		if err := conn.Close(); err != nil {
			fmt.Printf("got error `%v` while closing connection for client: %s", err, conn.RemoteAddr().String())
		}
	}(conn)
	return nil
}

func (s *server) handleContentFromConnection(conn net.Conn, writer *bufio.Writer, cache *rwCache) error {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()

		if line == "terminate" {
			conn.Close()
			err := s.close(writer)
			if err != nil {
				return err
			}
		}

		if isInvalidInput(line) {
			break
		}

		if !isDuplicate(cache, line) {
			atomic.AddInt32(&unique, 1)
			cache.set(line)
			err := writeToFile(writer, line)
			if err != nil {
				return err
			}
		} else {
			atomic.AddInt32(&duplicates, 1)
		}
	}
	return nil
}

func writeToFile(writer *bufio.Writer, value string) error {
	if _, err := writer.WriteString(value + "\n"); err != nil {
		return err
	}
	return nil
}

func flushWriteBuffer(writer *bufio.Writer) error {
	fmt.Println("Flushing buffer...")
	if err := writer.Flush(); err != nil {
		return err
	}
	return nil
}

func (r *rwCache) get(key string) string {
	r.RLock()
	defer r.RUnlock()
	return r.m[key]
}

func (r *rwCache) set(key string) {
	r.Lock()
	defer r.Unlock()
	r.m[key] = key
}
