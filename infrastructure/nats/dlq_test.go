package natsModel

import (
	"encoding/json"
	"testing"
	"time"
)

/** DefaultDLQPolicy */

func TestDefaultDLQPolicy(t *testing.T) {
	policy := DefaultDLQPolicy("my-service")

	if policy == nil {
		t.Fatal("DefaultDLQPolicy() returned nil")
	}
	if policy.Stream != "MAPEXOS-DLQ" {
		t.Errorf("expected Stream 'MAPEXOS-DLQ', got %q", policy.Stream)
	}
	if policy.Subject != "dlq.mapexos" {
		t.Errorf("expected Subject 'dlq.mapexos', got %q", policy.Subject)
	}
	if policy.ServiceName != "my-service" {
		t.Errorf("expected ServiceName 'my-service', got %q", policy.ServiceName)
	}
	if policy.ServiceType != "unknown" {
		t.Errorf("expected ServiceType 'unknown', got %q", policy.ServiceType)
	}
	if policy.EventType != "unknown" {
		t.Errorf("expected EventType 'unknown', got %q", policy.EventType)
	}
}

/** GetStream */

func TestGetStream_Custom(t *testing.T) {
	policy := &DLQPolicy{Stream: "CUSTOM-DLQ"}
	if policy.GetStream() != "CUSTOM-DLQ" {
		t.Errorf("expected 'CUSTOM-DLQ', got %q", policy.GetStream())
	}
}

func TestGetStream_Default(t *testing.T) {
	policy := &DLQPolicy{}
	if policy.GetStream() != "MAPEXOS-DLQ" {
		t.Errorf("expected default 'MAPEXOS-DLQ', got %q", policy.GetStream())
	}
}

/** GetSubject */

func TestGetSubject_Custom(t *testing.T) {
	policy := &DLQPolicy{Subject: "dlq.events"}
	if policy.GetSubject() != "dlq.events" {
		t.Errorf("expected 'dlq.events', got %q", policy.GetSubject())
	}
}

func TestGetSubject_Default(t *testing.T) {
	policy := &DLQPolicy{}
	if policy.GetSubject() != "dlq.mapexos" {
		t.Errorf("expected default 'dlq.mapexos', got %q", policy.GetSubject())
	}
}

/** NewDLQMessage */

func TestNewDLQMessage_AllFields(t *testing.T) {
	policy := &DLQPolicy{
		ServiceName: "events-service",
		ServiceType: "processor",
		EventType:   "raw-events",
	}

	firstDelivery := time.Now().Add(-10 * time.Minute)
	msg := NewDLQMessage(
		policy,
		"events-consumer",
		"tracker-123",
		"org-abc",
		"path-xyz",
		"events.raw",
		"EVENTS-RAW",
		[]byte(`{"sensor":"temp","value":42}`),
		map[string]string{"X-Custom": "header"},
		"connection timeout",
		5,
		firstDelivery,
	)

	if msg == nil {
		t.Fatal("NewDLQMessage returned nil")
	}
	if msg.ID == "" {
		t.Error("expected non-empty UUID")
	}
	if msg.EventTrackerId != "tracker-123" {
		t.Errorf("expected EventTrackerId 'tracker-123', got %q", msg.EventTrackerId)
	}
	if msg.OrgId != "org-abc" {
		t.Errorf("expected OrgId 'org-abc', got %q", msg.OrgId)
	}
	if msg.PathKey != "path-xyz" {
		t.Errorf("expected PathKey 'path-xyz', got %q", msg.PathKey)
	}
	if msg.ServiceName != "events-service" {
		t.Errorf("expected ServiceName 'events-service', got %q", msg.ServiceName)
	}
	if msg.ServiceType != "processor" {
		t.Errorf("expected ServiceType 'processor', got %q", msg.ServiceType)
	}
	if msg.EventType != "raw-events" {
		t.Errorf("expected EventType 'raw-events', got %q", msg.EventType)
	}
	if msg.OriginalSubject != "events.raw" {
		t.Errorf("expected OriginalSubject 'events.raw', got %q", msg.OriginalSubject)
	}
	if msg.OriginalStream != "EVENTS-RAW" {
		t.Errorf("expected OriginalStream 'EVENTS-RAW', got %q", msg.OriginalStream)
	}
	if string(msg.OriginalData) != `{"sensor":"temp","value":42}` {
		t.Errorf("expected OriginalData preserved, got %q", string(msg.OriginalData))
	}
	if msg.OriginalHeaders["X-Custom"] != "header" {
		t.Error("expected OriginalHeaders preserved")
	}
	if msg.LastError != "connection timeout" {
		t.Errorf("expected LastError 'connection timeout', got %q", msg.LastError)
	}
	if msg.ErrorCount != 5 {
		t.Errorf("expected ErrorCount 5, got %d", msg.ErrorCount)
	}
	if msg.TotalDeliveries != 5 {
		t.Errorf("expected TotalDeliveries 5, got %d", msg.TotalDeliveries)
	}
	if msg.ConsumerName != "events-consumer" {
		t.Errorf("expected ConsumerName 'events-consumer', got %q", msg.ConsumerName)
	}
	if msg.FirstDelivery != firstDelivery {
		t.Error("expected FirstDelivery preserved")
	}
	if msg.SentToDLQAt.IsZero() {
		t.Error("expected SentToDLQAt to be set")
	}
}

func TestNewDLQMessage_UniqueIDs(t *testing.T) {
	policy := DefaultDLQPolicy("test")
	now := time.Now()

	msg1 := NewDLQMessage(policy, "c1", "", "", "", "s1", "S1", nil, nil, "", 1, now)
	msg2 := NewDLQMessage(policy, "c2", "", "", "", "s2", "S2", nil, nil, "", 1, now)

	if msg1.ID == msg2.ID {
		t.Error("expected unique IDs for different messages")
	}
}

/** ToJSON */

func TestToJSON_ValidOutput(t *testing.T) {
	msg := &DLQMessage{
		ID:              "test-id",
		ServiceName:     "test-service",
		OriginalSubject: "test.subject",
		OriginalData:    []byte(`{"key":"value"}`),
		LastError:       "test error",
		ErrorCount:      3,
	}

	data, err := msg.ToJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty JSON output")
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("ToJSON output is not valid JSON: %v", err)
	}

	if parsed["id"] != "test-id" {
		t.Errorf("expected id 'test-id', got %v", parsed["id"])
	}
	if parsed["serviceName"] != "test-service" {
		t.Errorf("expected serviceName 'test-service', got %v", parsed["serviceName"])
	}
	if parsed["lastError"] != "test error" {
		t.Errorf("expected lastError 'test error', got %v", parsed["lastError"])
	}
}

func TestToJSON_PreservesOriginalData(t *testing.T) {
	originalJSON := `{"sensor":"temperature","value":42.5}`
	msg := &DLQMessage{
		ID:           "test",
		OriginalData: json.RawMessage(originalJSON),
	}

	data, err := msg.ToJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]json.RawMessage
	json.Unmarshal(data, &parsed)

	if string(parsed["originalData"]) != originalJSON {
		t.Errorf("expected originalData to be raw JSON, got %s", string(parsed["originalData"]))
	}
}

func TestToJSON_EmptyMessage(t *testing.T) {
	msg := &DLQMessage{}
	data, err := msg.ToJSON()
	if err != nil {
		t.Fatalf("unexpected error for empty message: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON even for empty message")
	}
}
