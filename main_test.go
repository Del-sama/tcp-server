package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

func init() {
	f, _ := initializeFile("test.log")
	writer := bufio.NewWriter(f)
	cache := &rwCache{m: make(map[string]string)}
	s := newServer("localhost:2030", 10, 0)
	go func() {
		s.run(writer, cache)
	}()

	err := os.Remove("test.log")
	if err != nil {
		fmt.Println(err)
	}
}

func ExampleLogReport() {
	logReport(5, 50, 10)
	// Output:
	// Received 5 unique numbers, 10 duplicates. Unique total: 50
}

func TestFlushWriteBuffer(t *testing.T) {
	file, err := os.Create("temp.log")
	if err != nil {
		t.Error(err)
	}
	writer := bufio.NewWriter(file)
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
		}
	}(file)

	if err != nil {
		t.Error(err)
	}

	err = writeToFile(writer, "123456789")
	if err != nil {
		t.Error(err)
	}
	data, err := os.ReadFile("temp.log")
	if err != nil {
		t.Error(err)
	}
	if string(data) != "" {
		t.Errorf("Expected empty file, got %s", string(data))
	}

	err = flushWriteBuffer(writer)
	if err != nil {
		t.Error(err)
	}
	data, err = os.ReadFile("temp.log")
	if string(data) != "123456789\n" {
		t.Errorf("Expected 123456789, got %s", string(data))
	}
	err = os.Remove("temp.log")
	if err != nil {
		t.Error(err)
	}
}

func TestInvalidInput(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{
			input:    "123456789",
			expected: false,
		},
		{
			input:    "1234567a9",
			expected: true,
		},
		{
			input:    "12345678",
			expected: true,
		},
		{
			input:    "abcdefghij",
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			r := isInvalidInput(test.input)
			if r != test.expected {
				t.Errorf("expected %t got %t", test.expected, r)
			}
		})
	}
}

func TestIsDuplicate(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{
			input:    "123456789",
			expected: true,
		},
		{
			input:    "234567890",
			expected: false,
		},
	}
	cache := &rwCache{m: make(map[string]string)}
	cache.set("123456789")
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			r := isDuplicate(cache, test.input)
			if r != test.expected {
				t.Errorf("expected %t got %t", test.expected, r)
			}
		})
	}
}

func TestClientConnection(t *testing.T) {
	err := makeTcpClient("000000001", "localhost:2030")
	if err != nil {
		t.Error("got error: ", err)
	}
	time.Sleep(3 * time.Second)
	data, err := os.ReadFile("test.log")
	if err != nil {
		t.Error("got error: ", err)
	}
	if string(data) != "000000001\n" {
		t.Errorf("expected `000000001` got %s", string(data))
	}
}

func makeTcpClient(input string, addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}

	defer func(conn net.Conn) {
		err = conn.Close()
		if err != nil {

		}
	}(conn)

	if _, err := conn.Write([]byte(input + "\n")); err != nil {
		return err
	}
	return nil
}
