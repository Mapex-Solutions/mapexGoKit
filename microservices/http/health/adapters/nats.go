package adapters

import (
	"context"
	"time"

	common "github.com/Mapex-Solutions/mapexGoKit/infrastructure/common/ports"
	natsModel "github.com/Mapex-Solutions/mapexGoKit/infrastructure/nats"
)

// NATSAdapter checks NATS health using the native PING/PONG protocol.
type NATSAdapter struct {
	client *natsModel.Client
	name   string
}

// NewNATSAdapter creates a new NATS health adapter with the given instance name.
func NewNATSAdapter(client *natsModel.Client, name string) *NATSAdapter {
	return &NATSAdapter{client: client, name: name}
}

// Name returns the adapter identifier for this NATS instance.
func (a *NATSAdapter) Name() string { return "nats:" + a.name }

// Check pings NATS using the native PING/PONG protocol and returns the health status.
func (a *NATSAdapter) Check(_ context.Context) common.HealthStatus {
	rtt, err := a.client.Ping()

	status := common.HealthStatus{
		Service:     "nats:" + a.name,
		Connected:   err == nil,
		LatencyMs:   rtt.Milliseconds(),
		LastCheckAt: time.Now(),
	}
	if err != nil {
		status.ErrorMessage = err.Error()
	}

	return status
}
