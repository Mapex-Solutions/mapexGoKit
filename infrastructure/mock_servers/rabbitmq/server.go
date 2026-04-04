package rabbitmq

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
)

/**
 * Mock AMQP 0-9-1 Server
 *
 * Simulates a RabbitMQ server for testing AMQP publisher executors.
 * Implements the full AMQP 0-9-1 connection handshake:
 *   Protocol Header -> Connection.Start -> StartOk -> Tune -> TuneOk -> Open -> OpenOk
 *   -> Channel.Open -> OpenOk -> (optional Exchange.Declare -> DeclareOk)
 *   -> Basic.Publish + Content Header + Content Body
 *   -> Channel.Close -> CloseOk -> Connection.Close -> CloseOk
 *
 * Captures all published messages into a channel for test assertions.
 *
 * Two entry points:
 *   StartServer(t)          — unit tests (random port, t.Fatal on error)
 *   ForBenchmark(port)      — benchmarks (fixed port, auto-drain channel)
 *
 * Both delegate to startServer(listener) which owns the accept loop.
 */

// Message holds a captured message from the mock AMQP server.
type Message struct {
	Exchange   string
	RoutingKey string
	Body       []byte
}

// StartServer starts a mock AMQP 0-9-1 server on a random port.
// Returns the port, a channel for captured messages, and a cleanup function.
func StartServer(t *testing.T) (int, chan Message, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start mock AMQP server: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	messages, cleanup := startServer(listener)
	return port, messages, cleanup
}

// ForBenchmark starts a mock AMQP 0-9-1 server on a fixed port.
// Messages are automatically drained (not captured). Returns a cleanup function.
func ForBenchmark(port int) (func(), error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, fmt.Errorf("mock AMQP server port %d: %w", port, err)
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

// handleConnection handles a single AMQP 0-9-1 client connection.
func handleConnection(conn net.Conn, messages chan<- Message, wg *sync.WaitGroup) {
	defer wg.Done()
	defer conn.Close()

	// Step 1: Read protocol header "AMQP\x00\x00\x09\x01"
	protoHeader := make([]byte, 8)
	if _, err := io.ReadFull(conn, protoHeader); err != nil {
		return
	}

	// Step 2: Send Connection.Start (class=10, method=10)
	startArgs := buildConnectionStartArgs()
	startPayload := buildMethodPayload(10, 10, startArgs)
	conn.Write(buildFrame(1, 0, startPayload))

	// Step 3: Read frames and respond to handshake
	var currentMsg Message
	var contentBodySize uint64

	for {
		frameType, channel, payload, err := readFrame(conn)
		if err != nil {
			return
		}

		switch frameType {
		case 1: // Method frame
			if len(payload) < 4 {
				return
			}
			classID := binary.BigEndian.Uint16(payload[0:2])
			methodID := binary.BigEndian.Uint16(payload[2:4])

			switch {
			case classID == 10 && methodID == 11: // Connection.StartOk
				// Send Connection.Tune (channel-max=2047, frame-max=131072, heartbeat=0)
				tuneArgs := buildConnectionTuneArgs()
				tunePayload := buildMethodPayload(10, 30, tuneArgs)
				conn.Write(buildFrame(1, 0, tunePayload))

			case classID == 10 && methodID == 31: // Connection.TuneOk
				// Nothing to send, wait for Connection.Open

			case classID == 10 && methodID == 40: // Connection.Open
				// Send Connection.OpenOk (reserved-1: shortstr "")
				openOkArgs := []byte{0} // empty shortstr
				openOkPayload := buildMethodPayload(10, 41, openOkArgs)
				conn.Write(buildFrame(1, 0, openOkPayload))

			case classID == 20 && methodID == 10: // Channel.Open
				// Send Channel.OpenOk (reserved-1: longstr "")
				openOkArgs := make([]byte, 4) // empty longstr (4-byte length = 0)
				openOkPayload := buildMethodPayload(20, 11, openOkArgs)
				conn.Write(buildFrame(1, channel, openOkPayload))

			case classID == 40 && methodID == 10: // Exchange.Declare
				// Send Exchange.DeclareOk
				declareOkPayload := buildMethodPayload(40, 11, nil)
				conn.Write(buildFrame(1, channel, declareOkPayload))

			case classID == 60 && methodID == 40: // Basic.Publish
				// Extract exchange and routing key
				currentMsg = extractPublishArgs(payload[4:])

			case classID == 20 && methodID == 40: // Channel.Close
				// Send Channel.CloseOk
				closeOkPayload := buildMethodPayload(20, 41, nil)
				conn.Write(buildFrame(1, channel, closeOkPayload))

			case classID == 10 && methodID == 50: // Connection.Close
				// Send Connection.CloseOk
				closeOkPayload := buildMethodPayload(10, 51, nil)
				conn.Write(buildFrame(1, 0, closeOkPayload))
				return
			}

		case 2: // Content header frame
			if len(payload) >= 12 {
				contentBodySize = binary.BigEndian.Uint64(payload[4:12])
			}
			if contentBodySize == 0 {
				// Empty body, emit message now
				messages <- currentMsg
				currentMsg = Message{}
			}

		case 3: // Content body frame
			currentMsg.Body = payload
			messages <- currentMsg
			currentMsg = Message{}
			contentBodySize = 0
		}
	}
}

/**
 * AMQP Frame Builders
 */

// buildFrame builds an AMQP 0-9-1 frame.
func buildFrame(frameType byte, channel uint16, payload []byte) []byte {
	frame := make([]byte, 7+len(payload)+1)
	frame[0] = frameType
	binary.BigEndian.PutUint16(frame[1:3], channel)
	binary.BigEndian.PutUint32(frame[3:7], uint32(len(payload)))
	copy(frame[7:], payload)
	frame[len(frame)-1] = 0xCE // frame end marker
	return frame
}

// buildMethodPayload builds a method frame payload with class/method IDs.
func buildMethodPayload(classID, methodID uint16, args []byte) []byte {
	payload := make([]byte, 4+len(args))
	binary.BigEndian.PutUint16(payload[0:2], classID)
	binary.BigEndian.PutUint16(payload[2:4], methodID)
	copy(payload[4:], args)
	return payload
}

// buildConnectionStartArgs builds Connection.Start method arguments.
func buildConnectionStartArgs() []byte {
	var buf bytes.Buffer

	// version-major (octet)
	buf.WriteByte(0)
	// version-minor (octet)
	buf.WriteByte(9)

	// server-properties (table) with "product" = "mock"
	buf.Write(buildTable(map[string]string{"product": "mock"}))

	// mechanisms (longstr) = "PLAIN"
	mechanisms := "PLAIN"
	binary.Write(&buf, binary.BigEndian, uint32(len(mechanisms)))
	buf.WriteString(mechanisms)

	// locales (longstr) = "en_US"
	locales := "en_US"
	binary.Write(&buf, binary.BigEndian, uint32(len(locales)))
	buf.WriteString(locales)

	return buf.Bytes()
}

// buildConnectionTuneArgs builds Connection.Tune method arguments.
func buildConnectionTuneArgs() []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint16(2047))   // channel-max
	binary.Write(&buf, binary.BigEndian, uint32(131072)) // frame-max
	binary.Write(&buf, binary.BigEndian, uint16(0))      // heartbeat
	return buf.Bytes()
}

// buildTable builds an AMQP table from a string map.
func buildTable(entries map[string]string) []byte {
	var content bytes.Buffer
	for k, v := range entries {
		// field name (shortstr: 1-byte length + data)
		content.WriteByte(byte(len(k)))
		content.WriteString(k)
		// field type 'S' = longstr
		content.WriteByte('S')
		// longstr value (4-byte length + data)
		binary.Write(&content, binary.BigEndian, uint32(len(v)))
		content.WriteString(v)
	}

	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint32(content.Len()))
	buf.Write(content.Bytes())
	return buf.Bytes()
}

/**
 * AMQP Frame Readers
 */

// readFrame reads a single AMQP frame from the connection.
func readFrame(conn net.Conn) (byte, uint16, []byte, error) {
	header := make([]byte, 7)
	if _, err := io.ReadFull(conn, header); err != nil {
		return 0, 0, nil, err
	}

	frameType := header[0]
	channel := binary.BigEndian.Uint16(header[1:3])
	size := binary.BigEndian.Uint32(header[3:7])

	payload := make([]byte, size)
	if size > 0 {
		if _, err := io.ReadFull(conn, payload); err != nil {
			return 0, 0, nil, err
		}
	}

	// Read frame-end marker (0xCE)
	end := make([]byte, 1)
	if _, err := io.ReadFull(conn, end); err != nil {
		return 0, 0, nil, err
	}
	if end[0] != 0xCE {
		return 0, 0, nil, fmt.Errorf("invalid AMQP frame end: 0x%02x", end[0])
	}

	return frameType, channel, payload, nil
}

// extractPublishArgs extracts exchange and routing key from Basic.Publish args.
func extractPublishArgs(args []byte) Message {
	offset := 0

	// reserved-1 (short = 2 bytes)
	if len(args) < 2 {
		return Message{}
	}
	offset += 2

	// exchange (shortstr: 1-byte length + data)
	if offset >= len(args) {
		return Message{}
	}
	exchangeLen := int(args[offset])
	offset++
	exchange := ""
	if exchangeLen > 0 && offset+exchangeLen <= len(args) {
		exchange = string(args[offset : offset+exchangeLen])
	}
	offset += exchangeLen

	// routing-key (shortstr: 1-byte length + data)
	if offset >= len(args) {
		return Message{Exchange: exchange}
	}
	routingKeyLen := int(args[offset])
	offset++
	routingKey := ""
	if routingKeyLen > 0 && offset+routingKeyLen <= len(args) {
		routingKey = string(args[offset : offset+routingKeyLen])
	}

	return Message{Exchange: exchange, RoutingKey: routingKey}
}
