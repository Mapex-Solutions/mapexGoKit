package natsModel

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

/**
DLQPolicy Methods
*/

// DefaultDLQPolicy returns a default DLQ policy
func DefaultDLQPolicy(serviceName string) *DLQPolicy {
	return &DLQPolicy{
		Stream:      "MAPEXOS-DLQ",
		Subject:     "dlq.mapexos",
		ServiceName: serviceName,
		ServiceType: "unknown",
		EventType:   "unknown",
	}
}

// GetStream returns the DLQ stream name or default
func (p *DLQPolicy) GetStream() string {
	if p.Stream == "" {
		return "MAPEXOS-DLQ"
	}
	return p.Stream
}

// GetSubject returns the DLQ subject or default
func (p *DLQPolicy) GetSubject() string {
	if p.Subject != "" {
		return p.Subject
	}
	return "dlq.mapexos"
}

/**
DLQMessage Methods
*/

// NewDLQMessage creates a new DLQ message with generated UUID.
// orgId, pathKey and eventTrackerId are mandatory for pipeline tracing and multi-tenant filtering.
func NewDLQMessage(
	dlqPolicy *DLQPolicy,
	consumerName string,
	eventTrackerId string,
	orgId string,
	pathKey string,
	originalSubject string,
	originalStream string,
	originalData []byte,
	originalHeaders map[string]string,
	lastError string,
	deliveryCount int,
	firstDelivery time.Time,
) *DLQMessage {
	return &DLQMessage{
		ID:              uuid.New().String(),
		EventTrackerId:  eventTrackerId,
		OrgId:           orgId,
		PathKey:         pathKey,
		ServiceName:     dlqPolicy.ServiceName,
		ServiceType:     dlqPolicy.ServiceType,
		EventType:       dlqPolicy.EventType,
		OriginalSubject: originalSubject,
		OriginalStream:  originalStream,
		OriginalData:    originalData,
		OriginalHeaders: originalHeaders,
		LastError:       lastError,
		ErrorCount:      deliveryCount,
		FirstDelivery:   firstDelivery,
		LastDelivery:    time.Now(),
		TotalDeliveries: deliveryCount,
		ConsumerName:    consumerName,
		SentToDLQAt:     time.Now(),
	}
}

// ToJSON marshals the DLQ message to JSON bytes
func (m *DLQMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}
