package common

import "time"

// HealthStatus represents the health state of an infrastructure dependency.
type HealthStatus struct {
	Connected    bool      `json:"connected"`
	Service      string    `json:"service"`
	LatencyMs    int64     `json:"latencyMs,omitempty"`
	LastCheckAt  time.Time `json:"lastCheckAt"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
}
