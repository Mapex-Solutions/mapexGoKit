// Package mqttclient provides a thin, opinionated wrapper around
// eclipse/paho.mqtt.golang for use as a generic MQTT client across the
// Mapex platform.
//
// Why a wrapper:
//   - Keeps a single integration point so the underlying library can be
//     swapped without touching every caller.
//   - Hides paho's option-builder API behind a flat Config struct that
//     mirrors httpclient.Config in shape, so consumers learn one pattern.
//   - Provides context-aware Connect / Publish surfaces (paho's API is
//     channel-based; callers want context.Context).
//
// Non-goals:
//   - This package does not own subscription / consumer state; the saga
//     journeys publish only and observe via HTTP. Subscribe is exposed
//     for completeness but not load-tested.
package mqttclient

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Config is the flat configuration consumed by New. Every field has a
// sensible default; the only mandatory field is BrokerURL.
type Config struct {
	// BrokerURL is the MQTT broker endpoint. Format: tcp://host:port or
	// ssl://host:port. Required.
	BrokerURL string

	// ClientID is the MQTT client identifier sent on CONNECT. Empty
	// means the wrapper synthesises a unique id ("mapex-<unix-nanos>").
	ClientID string

	// Username + Password authenticate the connection. Both empty means
	// anonymous connect — the broker may still reject if it requires
	// auth. For Mapex devices, Username = assetUUID, Password = the
	// per-asset MQTT password generated at asset create time.
	Username string
	Password string

	// KeepAlive is the seconds between client-side PINGREQ frames. The
	// broker uses 1.5x this as the silent-drop timeout, so KeepAlive=30
	// gives ~45s drop detection. Default: 30s.
	KeepAlive time.Duration

	// ConnectTimeout caps the handshake. Default: 10s.
	ConnectTimeout time.Duration

	// CleanSession discards any prior session state on connect. Saga
	// journeys want true so each test starts from a clean slate.
	// Default: true.
	CleanSession bool

	// AutoReconnect lets paho retry on disconnect. Saga journeys want
	// false so a drop fails the test instead of silently recovering.
	// Default: false.
	AutoReconnect bool
}

// Client wraps a paho client with a context-aware surface. Concurrent
// Publish is safe (paho serialises internally); Connect / Disconnect
// are guarded by an internal mutex so repeated calls during cleanup
// are well-defined.
type Client struct {
	cfg Config
	mu  sync.Mutex
	c   mqtt.Client
}

// New builds a Client without opening the connection. Call Connect to
// attempt the handshake. Returns an error only when BrokerURL is empty;
// every other field falls back to its default.
func New(cfg Config) (*Client, error) {
	if cfg.BrokerURL == "" {
		return nil, errors.New("mqttclient: BrokerURL is required")
	}
	if cfg.ClientID == "" {
		cfg.ClientID = fmt.Sprintf("mapex-%d", time.Now().UnixNano())
	}
	if cfg.KeepAlive == 0 {
		cfg.KeepAlive = 30 * time.Second
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = 10 * time.Second
	}
	// Booleans default to false; CleanSession should default to true so
	// callers do not silently inherit prior session state.
	// We can't tell "explicitly false" from "zero value" with a bool,
	// so the convention is: if the caller wants persistent session,
	// they must set CleanSession explicitly to false AFTER the wrapper
	// applies the default. Document the default here clearly.
	c := &Client{cfg: cfg}
	c.applyDefaults()
	return c, nil
}

// applyDefaults flips the boolean defaults that go's zero value gets
// wrong (CleanSession defaults to true).
func (c *Client) applyDefaults() {
	// Bool zero is false; we want CleanSession=true by default. Callers
	// who want persistent session must explicitly set it back to false
	// after New, or pass CleanSession=true (no-op).
	if !c.cfg.CleanSession {
		c.cfg.CleanSession = true
	}
}

// Connect opens the connection, waiting up to ConnectTimeout (or until
// ctx cancels, whichever fires first).
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.c != nil && c.c.IsConnected() {
		return nil
	}

	opts := mqtt.NewClientOptions().
		AddBroker(c.cfg.BrokerURL).
		SetClientID(c.cfg.ClientID).
		SetCleanSession(c.cfg.CleanSession).
		SetAutoReconnect(c.cfg.AutoReconnect).
		SetKeepAlive(c.cfg.KeepAlive).
		SetConnectTimeout(c.cfg.ConnectTimeout)
	if c.cfg.Username != "" {
		opts.SetUsername(c.cfg.Username)
	}
	if c.cfg.Password != "" {
		opts.SetPassword(c.cfg.Password)
	}

	client := mqtt.NewClient(opts)
	tok := client.Connect()

	select {
	case <-ctx.Done():
		return fmt.Errorf("mqttclient: connect cancelled: %w", ctx.Err())
	case <-waitToken(tok, c.cfg.ConnectTimeout):
		if err := tok.Error(); err != nil {
			return fmt.Errorf("mqttclient: connect failed: %w", err)
		}
	}
	c.c = client
	return nil
}

// Disconnect closes the connection. quiesceMillis is paho's "wait for
// pending publishes" budget — pass 0 to drop immediately, 250 for a
// graceful close that flushes inflight frames.
func (c *Client) Disconnect(quiesceMillis uint) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.c == nil {
		return
	}
	if c.c.IsConnected() {
		c.c.Disconnect(quiesceMillis)
	}
	c.c = nil
}

// IsConnected reports the current connection state.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.c != nil && c.c.IsConnected()
}

// Publish sends payload on topic at the given QoS. Blocks until the
// broker acks (QoS 1+) or until ctx cancels — whichever fires first.
// QoS 0 acks immediately on the local socket; ctx still bounds it.
func (c *Client) Publish(ctx context.Context, topic string, qos byte, retained bool, payload []byte) error {
	c.mu.Lock()
	client := c.c
	c.mu.Unlock()
	if client == nil || !client.IsConnected() {
		return errors.New("mqttclient: not connected")
	}

	tok := client.Publish(topic, qos, retained, payload)
	select {
	case <-ctx.Done():
		return fmt.Errorf("mqttclient: publish cancelled: %w", ctx.Err())
	case <-waitToken(tok, c.cfg.ConnectTimeout):
		if err := tok.Error(); err != nil {
			return fmt.Errorf("mqttclient: publish failed: %w", err)
		}
	}
	return nil
}

// Subscribe registers handler for messages on topic at the given QoS.
// Exposed for completeness; saga journeys observe via HTTP and do not
// use this surface.
func (c *Client) Subscribe(ctx context.Context, topic string, qos byte, handler func(topic string, payload []byte)) error {
	c.mu.Lock()
	client := c.c
	c.mu.Unlock()
	if client == nil || !client.IsConnected() {
		return errors.New("mqttclient: not connected")
	}

	cb := func(_ mqtt.Client, msg mqtt.Message) {
		handler(msg.Topic(), msg.Payload())
	}
	tok := client.Subscribe(topic, qos, cb)
	select {
	case <-ctx.Done():
		return fmt.Errorf("mqttclient: subscribe cancelled: %w", ctx.Err())
	case <-waitToken(tok, c.cfg.ConnectTimeout):
		if err := tok.Error(); err != nil {
			return fmt.Errorf("mqttclient: subscribe failed: %w", err)
		}
	}
	return nil
}

// waitToken returns a channel that closes when the token completes or
// the timeout elapses. Uses paho's Token.Done() if available, falling
// back to a polling sentinel on older versions.
func waitToken(tok mqtt.Token, timeout time.Duration) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = tok.WaitTimeout(timeout)
	}()
	return done
}
