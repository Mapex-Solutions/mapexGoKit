package mqtt

import (
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
)

/**
 * Mock MQTT 3.1.1 Broker
 *
 * Simulates an MQTT broker for testing MQTT publisher executors.
 * Implements the minimum MQTT 3.1.1 binary protocol: CONNECT/CONNACK, PUBLISH/PUBACK, PINGREQ/PINGRESP, DISCONNECT.
 * Captures all published messages into a channel for test assertions.
 *
 * Two entry points:
 *   StartServer(t)          — unit tests (random port, t.Fatal on error)
 *   ForBenchmark(port)      — benchmarks (fixed port, auto-drain channel)
 *
 * Both delegate to startServer(listener) which owns the accept loop.
 */

// Message holds a captured message from the mock MQTT broker.
type Message struct {
	Topic   string
	Payload []byte
	QoS     byte
}

// StartServer starts a mock MQTT 3.1.1 broker on a random port.
// Returns the port, a channel for captured messages, and a cleanup function.
func StartServer(t *testing.T) (int, chan Message, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start mock MQTT broker: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	messages, cleanup := startServer(listener)
	return port, messages, cleanup
}

// ForBenchmark starts a mock MQTT 3.1.1 broker on a fixed port.
// Messages are automatically drained (not captured). Returns a cleanup function.
func ForBenchmark(port int) (func(), error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, fmt.Errorf("mock MQTT broker port %d: %w", port, err)
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

// handleConnection handles a single MQTT client connection.
func handleConnection(conn net.Conn, messages chan<- Message, wg *sync.WaitGroup) {
	defer wg.Done()
	defer conn.Close()

	for {
		// Read fixed header byte 1
		header := make([]byte, 1)
		_, err := io.ReadFull(conn, header)
		if err != nil {
			return
		}

		packetType := (header[0] >> 4) & 0x0F
		flags := header[0] & 0x0F

		// Read remaining length (MQTT variable-length encoding)
		remainingLength, err := readVarInt(conn)
		if err != nil {
			return
		}

		// Read the rest of the packet
		payload := make([]byte, remainingLength)
		if remainingLength > 0 {
			_, err = io.ReadFull(conn, payload)
			if err != nil {
				return
			}
		}

		switch packetType {
		case 1: // CONNECT -> respond with CONNACK (accepted)
			conn.Write([]byte{0x20, 0x02, 0x00, 0x00})

		case 3: // PUBLISH -> capture message
			qos := (flags >> 1) & 0x03
			offset := 0

			// Read topic length (2 bytes big-endian)
			if offset+2 > len(payload) {
				return
			}
			topicLen := int(payload[offset])<<8 | int(payload[offset+1])
			offset += 2

			// Read topic string
			if offset+topicLen > len(payload) {
				return
			}
			topic := string(payload[offset : offset+topicLen])
			offset += topicLen

			// Read packet ID for QoS > 0
			var packetID uint16
			if qos > 0 {
				if offset+2 > len(payload) {
					return
				}
				packetID = uint16(payload[offset])<<8 | uint16(payload[offset+1])
				offset += 2
			}

			// Remaining bytes are the message payload
			msgPayload := payload[offset:]

			messages <- Message{
				Topic:   topic,
				Payload: msgPayload,
				QoS:     qos,
			}

			// Send PUBACK for QoS 1
			if qos == 1 {
				puback := []byte{0x40, 0x02, byte(packetID >> 8), byte(packetID & 0xFF)}
				conn.Write(puback)
			}

		case 12: // PINGREQ -> respond with PINGRESP
			conn.Write([]byte{0xD0, 0x00})

		case 14: // DISCONNECT
			return
		}
	}
}

// readVarInt reads an MQTT variable-length integer from the connection.
func readVarInt(conn net.Conn) (int, error) {
	multiplier := 1
	value := 0
	buf := make([]byte, 1)

	for {
		_, err := io.ReadFull(conn, buf)
		if err != nil {
			return 0, err
		}
		value += int(buf[0]&0x7F) * multiplier
		if buf[0]&0x80 == 0 {
			break
		}
		multiplier *= 128
	}

	return value, nil
}
