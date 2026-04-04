package smtp

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
)

/**
 * Mock SMTP Server
 *
 * Simulates an SMTP server for testing email executors.
 * Implements the minimum SMTP protocol: EHLO, AUTH PLAIN, MAIL FROM, RCPT TO, DATA, QUIT.
 * Captures all received messages into a channel for test assertions.
 *
 * Two entry points:
 *   StartServer(t)          — unit tests (random port, t.Fatal on error)
 *   ForBenchmark(port)      — benchmarks (fixed port, auto-drain channel)
 *
 * Both delegate to startServer(listener) which owns the accept loop.
 */

// Message holds a captured email from the mock SMTP server.
type Message struct {
	From       string
	Recipients []string
	Data       string
}

// StartServer starts a mock SMTP server on a random port.
// Returns the port, a channel for captured messages, and a cleanup function.
func StartServer(t *testing.T) (int, chan Message, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start mock SMTP server: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	messages, cleanup := startServer(listener)
	return port, messages, cleanup
}

// ForBenchmark starts a mock SMTP server on a fixed port.
// Messages are automatically drained (not captured). Returns a cleanup function.
func ForBenchmark(port int) (func(), error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, fmt.Errorf("mock SMTP server port %d: %w", port, err)
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

// handleConnection handles a single SMTP conversation.
func handleConnection(conn net.Conn, messages chan<- Message, wg *sync.WaitGroup) {
	defer wg.Done()
	defer conn.Close()

	reader := bufio.NewReader(conn)

	// Send greeting
	fmt.Fprintf(conn, "220 mock ESMTP ready\r\n")

	var msg Message
	var inData bool
	var dataBuilder strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")

		if inData {
			if line == "." {
				inData = false
				msg.Data = dataBuilder.String()
				fmt.Fprintf(conn, "250 2.0.0 OK\r\n")
				messages <- msg
				msg = Message{}
				dataBuilder.Reset()
				continue
			}
			dataBuilder.WriteString(line + "\r\n")
			continue
		}

		upper := strings.ToUpper(line)

		switch {
		case strings.HasPrefix(upper, "EHLO") || strings.HasPrefix(upper, "HELO"):
			fmt.Fprintf(conn, "250-mock Hello\r\n250-AUTH PLAIN LOGIN\r\n250 OK\r\n")
		case strings.HasPrefix(upper, "AUTH"):
			fmt.Fprintf(conn, "235 2.7.0 Authentication successful\r\n")
		case strings.HasPrefix(upper, "MAIL FROM:"):
			msg.From = extractAngleBracketAddr(line)
			fmt.Fprintf(conn, "250 2.1.0 OK\r\n")
		case strings.HasPrefix(upper, "RCPT TO:"):
			msg.Recipients = append(msg.Recipients, extractAngleBracketAddr(line))
			fmt.Fprintf(conn, "250 2.1.5 OK\r\n")
		case strings.HasPrefix(upper, "DATA"):
			inData = true
			fmt.Fprintf(conn, "354 Start mail input; end with <CRLF>.<CRLF>\r\n")
		case strings.HasPrefix(upper, "QUIT"):
			fmt.Fprintf(conn, "221 2.0.0 Bye\r\n")
			return
		default:
			fmt.Fprintf(conn, "250 OK\r\n")
		}
	}
}

// extractAngleBracketAddr extracts the email address from angle brackets (e.g., "MAIL FROM:<user@host>").
func extractAngleBracketAddr(line string) string {
	start := strings.Index(line, "<")
	end := strings.Index(line, ">")
	if start >= 0 && end > start {
		return line[start+1 : end]
	}
	return line
}
