package mqttclient

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNew_RequiresBrokerURL(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error when BrokerURL is empty")
	}
	if !strings.Contains(err.Error(), "BrokerURL") {
		t.Errorf("expected error mentioning BrokerURL, got %v", err)
	}
}

func TestNew_AppliesDefaults(t *testing.T) {
	c, err := New(Config{BrokerURL: "tcp://localhost:1883"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.cfg.ClientID == "" {
		t.Error("expected ClientID to default to a synthesised value")
	}
	if !strings.HasPrefix(c.cfg.ClientID, "mapex-") {
		t.Errorf("expected ClientID to start with mapex-, got %q", c.cfg.ClientID)
	}
	if c.cfg.KeepAlive != 30*time.Second {
		t.Errorf("expected KeepAlive default 30s, got %v", c.cfg.KeepAlive)
	}
	if c.cfg.ConnectTimeout != 10*time.Second {
		t.Errorf("expected ConnectTimeout default 10s, got %v", c.cfg.ConnectTimeout)
	}
	if !c.cfg.CleanSession {
		t.Error("expected CleanSession to default to true")
	}
}

func TestNew_PreservesProvidedClientID(t *testing.T) {
	c, err := New(Config{BrokerURL: "tcp://localhost:1883", ClientID: "my-client"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.cfg.ClientID != "my-client" {
		t.Errorf("expected ClientID 'my-client', got %q", c.cfg.ClientID)
	}
}

func TestPublish_WithoutConnect_ReturnsError(t *testing.T) {
	c, err := New(Config{BrokerURL: "tcp://localhost:1883"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = c.Publish(context.Background(), "test/topic", 0, false, []byte("x"))
	if err == nil {
		t.Fatal("expected error publishing on unconnected client")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got %v", err)
	}
}

func TestSubscribe_WithoutConnect_ReturnsError(t *testing.T) {
	c, err := New(Config{BrokerURL: "tcp://localhost:1883"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = c.Subscribe(context.Background(), "test/topic", 0, func(_ string, _ []byte) {})
	if err == nil {
		t.Fatal("expected error subscribing on unconnected client")
	}
}

func TestDisconnect_Idempotent(t *testing.T) {
	c, err := New(Config{BrokerURL: "tcp://localhost:1883"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Disconnect on a never-connected client must be a no-op, not panic.
	c.Disconnect(0)
	c.Disconnect(0)
	if c.IsConnected() {
		t.Error("expected IsConnected=false after Disconnect on never-connected client")
	}
}

func TestConnect_RespectsContextCancel(t *testing.T) {
	// Use an unreachable broker port so Connect blocks until ctx fires.
	c, err := New(Config{
		BrokerURL:      "tcp://127.0.0.1:1",
		ConnectTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err = c.Connect(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled connect")
	}
}
