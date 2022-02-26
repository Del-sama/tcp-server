# tcp-server
A TCP server that opens a socket and restricts input to at most 5 concurrent clients. Clients will connect to the server. The server allows at most 5 clients at a time write one or more numbers of 9 digit numbers, each number followed by a newline sequence, and then close the connection. 
The server writes a de-duplicated list of the numbers to a log file in no particular order.
