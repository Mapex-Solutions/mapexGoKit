package nats

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
)

/**
 * Mock NATS Server
 *
 * Simulates a NATS server for testing NATS publisher executors.
 * Implements the minimum NATS protocol: INFO, CONNECT, PUB, HPUB (with headers), PING/PONG.
 * Captures all published messages into a channel for test assertions.
 *
 * Two entry points:
 *   StartServer(t)          — unit tests (random port, t.Fatal on error)
 *   ForBenchmark(port)      — benchmarks (fixed port, auto-drain channel)
 *
 * Both delegate to startServer(listener) which owns the accept loop.
 */

// Message holds a captured message from the mock NATS server.
type Message struct {
	Subject string
	Data    []byte
	Headers map[string]string
}

// StartServer starts a mock NATS server on a random port.
// Returns the port, a channel for captured messages, and a cleanup function.
func StartServer(t *testing.T) (int, chan Message, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start mock NATS server: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	messages, cleanup := startServer(listener)
	return port, messages, cleanup
}

// ForBenchmark starts a mock NATS server on a fixed port.
// Messages are automatically drained (not captured). Returns a cleanup function.
func ForBenchmark(port int) (func(), error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, fmt.Errorf("mock NATS server port %d: %w", port, err)
	}

	messages, cleanup := startServer(listener)
	go func() { for range messages {} }()
	return cleanup, nil
}

// startServer is the shared base that owns the accept loop.
// Both StartServer and ForBenchmark delegate here.
func startServer(listener net.Listener) (chan Message, func()) {
	messages := make(chan Message, 10)
	var wg sync.WaitGroup

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go handleConnection(conn, messages, &wg)
		}
	}()

	cleanup := func() {
		listener.Close()
		wg.Wait()
		close(messages)
	}

	return messages, cleanup
}

// handleConnection handles a single NATS client connection.
func handleConnection(conn net.Conn, messages chan<- Message, wg *sync.WaitGroup) {
	defer wg.Done()
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Send INFO (must include headers:true for HPUB support)
	info := `INFO {"server_id":"mock","server_name":"mock","version":"2.10.0","proto":1,"max_payload":1048576,"headers":true,"auth_required":false}` + "\r\n"
	conn.Write([]byte(info))

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")

		switch {
		case strings.HasPrefix(line, "CONNECT"):
			// No response in non-verbose mode

		case strings.HasPrefix(line, "PING"):
			conn.Write([]byte("PONG\r\n"))

		case strings.HasPrefix(line, "PUB "):
			// PUB <subject> [reply] <size>\r\n<payload>\r\n
			parts := strings.Fields(line)
			if len(parts) < 3 {
				continue
			}
			subject := parts[1]
			sizeStr := parts[len(parts)-1]
			size, _ := strconv.Atoi(sizeStr)

			payload := make([]byte, size)
			if size > 0 {
				io.ReadFull(reader, payload)
			}
			// Read trailing \r\n
			reader.ReadString('\n')

			messages <- Message{
				Subject: subject,
				Data:    payload,
			}

		case strings.HasPrefix(line, "HPUB "):
			// HPUB <subject> [reply] <header-size> <total-size>\r\n<header+payload>\r\n
			parts := strings.Fields(line)
			if len(parts) < 4 {
				continue
			}
			subject := parts[1]

			var headerSize, totalSize int
			fmt.Sscanf(parts[len(parts)-2], "%d", &headerSize)
			fmt.Sscanf(parts[len(parts)-1], "%d", &totalSize)

			totalPayload := make([]byte, totalSize)
			if totalSize > 0 {
				io.ReadFull(reader, totalPayload)
			}
			// Read trailing \r\n
			reader.ReadString('\n')

			msg := Message{
				Subject: subject,
				Headers: make(map[string]string),
			}

			if headerSize > 0 && headerSize <= len(totalPayload) {
				headerBytes := totalPayload[:headerSize]
				msg.Data = totalPayload[headerSize:]

				// Parse NATS headers (format: "NATS/1.0\r\nKey: Value\r\n...\r\n\r\n")
				headerStr := string(headerBytes)
				headerLines := strings.Split(headerStr, "\r\n")
				for _, hl := range headerLines[1:] { // Skip "NATS/1.0"
					if hl == "" {
						continue
					}
					k, v, found := strings.Cut(hl, ": ")
					if found {
						msg.Headers[k] = v
					}
				}
			} else {
				msg.Data = totalPayload
			}

			messages <- msg

		case line == "" || strings.HasPrefix(line, "SUB") || strings.HasPrefix(line, "UNSUB"):
			// Ignore subscriptions and empty lines
		}
	}
}
