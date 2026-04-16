package natsModel

import (
	"errors"
	"time"

	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// New creates a new NATS client connection and initializes the JetStream context.
//
// It accepts a NATS server URL and optional nats.Option parameters for customizing
// the connection behavior (e.g., authentication, reconnect policy).
//
// Example:
//
//	client, err := natsModel.New("nats://localhost:4222", nats.Name("MyApp"))
//	if err != nil {
//	  log.Fatal(err)
//	}
//
// Returns a pointer to the initialized Client or an error if the connection or JetStream setup fails.
func New(c Config) (*Client, error) {
	nc, err := c.Options.Connect()
	if err != nil {
		return nil, err
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, err
	}

	client := &Client{nc: nc, js: js}

	// Call OnConnect after initial connection
	if c.OnConnect != nil {
		c.OnConnect(client)
	}

	// Register reconnect handler to call OnConnect again after reconnection
	if c.OnConnect != nil {
		onConnect := c.OnConnect
		nc.SetReconnectHandler(func(_ *nats.Conn) {
			logger.Info("[INFRA:NATS] Reconnected — calling OnConnect callback")
			onConnect(client)
		})
	}

	logger.Info("[INFRA:NATS] Connected to server")
	return client, nil
}

// Ping measures the round-trip time to the NATS server using the native PING/PONG protocol.
func (c *Client) Ping() (time.Duration, error) {
	if c.nc == nil || c.nc.IsClosed() {
		return 0, errors.New("nats: connection is closed")
	}
	return c.nc.RTT()
}

// IsConnected returns true if the NATS connection is active and not closed.
func (c *Client) IsConnected() bool {
	return c.nc != nil && !c.nc.IsClosed()
}

// Close gracefully closes the NATS connection if it is open.
//
// This method is safe to call multiple times. It ensures that resources
// used by the Client are properly released.
func (c *Client) Close() {
	if c.nc != nil && !c.nc.IsClosed() {
		c.nc.Close()
	}
}
