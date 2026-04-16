package natsModel

import (
	"context"
	"errors"
	"testing"
	"time"
)

/**
Unit Tests - KV Types
*/

func TestKVConfig(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		cfg := KVConfig{
			Bucket: "test-bucket",
		}

		if cfg.Bucket != "test-bucket" {
			t.Errorf("Bucket = %s, want test-bucket", cfg.Bucket)
		}
		if cfg.Description != "" {
			t.Error("Description should be empty by default")
		}
		if cfg.TTL != 0 {
			t.Errorf("TTL = %v, want 0", cfg.TTL)
		}
		if cfg.MaxBytes != 0 {
			t.Errorf("MaxBytes = %d, want 0", cfg.MaxBytes)
		}
		if cfg.History != 0 {
			t.Errorf("History = %d, want 0", cfg.History)
		}
		if cfg.Replicas != 0 {
			t.Errorf("Replicas = %d, want 0", cfg.Replicas)
		}
	})

	t.Run("with all fields", func(t *testing.T) {
		cfg := KVConfig{
			Bucket:       "WORKFLOW-INSTANCES",
			Description:  "Workflow instance hot state",
			TTL:          24 * time.Hour,
			MaxBytes:     10 * 1024 * 1024 * 1024, // 10GB
			MaxValueSize: 1024 * 1024,              // 1MB
			History:      5,
			Replicas:     3,
		}

		if cfg.Bucket != "WORKFLOW-INSTANCES" {
			t.Errorf("Bucket = %s, want WORKFLOW-INSTANCES", cfg.Bucket)
		}
		if cfg.TTL != 24*time.Hour {
			t.Errorf("TTL = %v, want 24h", cfg.TTL)
		}
		if cfg.Replicas != 3 {
			t.Errorf("Replicas = %d, want 3", cfg.Replicas)
		}
		if cfg.History != 5 {
			t.Errorf("History = %d, want 5", cfg.History)
		}
		if cfg.MaxValueSize != 1024*1024 {
			t.Errorf("MaxValueSize = %d, want 1048576", cfg.MaxValueSize)
		}
	})
}

func TestKVEntry(t *testing.T) {
	t.Run("all fields", func(t *testing.T) {
		now := time.Now()
		entry := KVEntry{
			Key:      "inst.123",
			Value:    []byte(`{"state":{"counter":5}}`),
			Revision: 42,
			Created:  now,
		}

		if entry.Key != "inst.123" {
			t.Errorf("Key = %s, want inst.123", entry.Key)
		}
		if string(entry.Value) != `{"state":{"counter":5}}` {
			t.Errorf("Value = %s, want JSON", string(entry.Value))
		}
		if entry.Revision != 42 {
			t.Errorf("Revision = %d, want 42", entry.Revision)
		}
		if !entry.Created.Equal(now) {
			t.Errorf("Created = %v, want %v", entry.Created, now)
		}
	})

	t.Run("zero revision", func(t *testing.T) {
		entry := KVEntry{Key: "new-key"}
		if entry.Revision != 0 {
			t.Errorf("Revision = %d, want 0", entry.Revision)
		}
	})
}

/**
Unit Tests - KV Sentinel Errors
*/

func TestKVErrors(t *testing.T) {
	t.Run("errors are distinct", func(t *testing.T) {
		if errors.Is(ErrKVKeyNotFound, ErrKVKeyExists) {
			t.Error("ErrKVKeyNotFound should not match ErrKVKeyExists")
		}
		if errors.Is(ErrKVKeyNotFound, ErrKVCASConflict) {
			t.Error("ErrKVKeyNotFound should not match ErrKVCASConflict")
		}
		if errors.Is(ErrKVKeyExists, ErrKVCASConflict) {
			t.Error("ErrKVKeyExists should not match ErrKVCASConflict")
		}
	})

	t.Run("errors have messages", func(t *testing.T) {
		if ErrKVKeyNotFound.Error() == "" {
			t.Error("ErrKVKeyNotFound should have a message")
		}
		if ErrKVKeyExists.Error() == "" {
			t.Error("ErrKVKeyExists should have a message")
		}
		if ErrKVCASConflict.Error() == "" {
			t.Error("ErrKVCASConflict should have a message")
		}
	})

	t.Run("errors are matchable with errors.Is", func(t *testing.T) {
		wrapped := errors.Join(ErrKVKeyNotFound, errors.New("extra context"))
		if !errors.Is(wrapped, ErrKVKeyNotFound) {
			t.Error("wrapped error should match ErrKVKeyNotFound")
		}
	})
}

/**
Unit Tests - CreateKeyValue validation
*/

func TestCreateKeyValue_EmptyBucket(t *testing.T) {
	client := &Client{} // nil connection — we only test bucket validation
	_, err := client.CreateKeyValue(KVConfig{Bucket: ""})
	if err == nil {
		t.Error("CreateKeyValue should return error for empty bucket name")
	}
}

/**
Integration Tests (require NATS server)
*/

func skipIfNoNATSClient(t *testing.T) *Client {
	t.Helper()
	config := getTestConfig()
	client, err := New(config)
	if err != nil {
		t.Skipf("NATS not available: %v", err)
		return nil
	}
	return client
}

// cleanupKVBucket deletes a KV bucket if it exists, ensuring a clean state for tests.
func cleanupKVBucket(t *testing.T, client *Client, bucket string) {
	t.Helper()
	_ = client.js.DeleteKeyValue(context.Background(), bucket)
}

func TestCreateKeyValue_Integration(t *testing.T) {
	client := skipIfNoNATSClient(t)
	if client == nil {
		return
	}

	t.Run("create new bucket", func(t *testing.T) {
		store, err := client.CreateKeyValue(KVConfig{
			Bucket:      "TEST-KV-CREATE",
			Description: "Integration test bucket",
			Replicas:    1,
		})
		if err != nil {
			t.Fatalf("CreateKeyValue() error = %v", err)
		}
		if store == nil {
			t.Fatal("CreateKeyValue() returned nil store")
		}
		if store.Bucket() != "TEST-KV-CREATE" {
			t.Errorf("Bucket() = %s, want TEST-KV-CREATE", store.Bucket())
		}
	})

	t.Run("create idempotent", func(t *testing.T) {
		store1, err := client.CreateKeyValue(KVConfig{Bucket: "TEST-KV-IDEMPOTENT", Replicas: 1})
		if err != nil {
			t.Fatalf("first CreateKeyValue() error = %v", err)
		}

		store2, err := client.CreateKeyValue(KVConfig{Bucket: "TEST-KV-IDEMPOTENT", Replicas: 1})
		if err != nil {
			t.Fatalf("second CreateKeyValue() error = %v", err)
		}

		if store1.Bucket() != store2.Bucket() {
			t.Error("idempotent create should return same bucket")
		}
	})
}

func TestKVPutGet_Integration(t *testing.T) {
	client := skipIfNoNATSClient(t)
	if client == nil {
		return
	}

	store, err := client.CreateKeyValue(KVConfig{Bucket: "TEST-KV-PUTGET", Replicas: 1})
	if err != nil {
		t.Fatalf("CreateKeyValue() error = %v", err)
	}

	t.Run("put and get", func(t *testing.T) {
		data := []byte(`{"state":{"counter":1}}`)
		rev, err := store.Put("inst.100", data)
		if err != nil {
			t.Fatalf("Put() error = %v", err)
		}
		if rev == 0 {
			t.Error("Put() revision should be > 0")
		}

		entry, err := store.Get("inst.100")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if entry.Key != "inst.100" {
			t.Errorf("Key = %s, want inst.100", entry.Key)
		}
		if string(entry.Value) != string(data) {
			t.Errorf("Value = %s, want %s", string(entry.Value), string(data))
		}
		if entry.Revision != rev {
			t.Errorf("Revision = %d, want %d", entry.Revision, rev)
		}
		if entry.Created.IsZero() {
			t.Error("Created should not be zero")
		}
	})

	t.Run("put overwrite advances revision", func(t *testing.T) {
		rev1, _ := store.Put("inst.200", []byte("v1"))
		rev2, _ := store.Put("inst.200", []byte("v2"))
		if rev2 <= rev1 {
			t.Errorf("second Put revision (%d) should be > first (%d)", rev2, rev1)
		}

		entry, _ := store.Get("inst.200")
		if string(entry.Value) != "v2" {
			t.Errorf("Value = %s, want v2", string(entry.Value))
		}
	})

	t.Run("get non-existent key", func(t *testing.T) {
		_, err := store.Get("inst.nonexistent")
		if !errors.Is(err, ErrKVKeyNotFound) {
			t.Errorf("Get(nonexistent) error = %v, want ErrKVKeyNotFound", err)
		}
	})
}

func TestKVCreate_Integration(t *testing.T) {
	client := skipIfNoNATSClient(t)
	if client == nil {
		return
	}

	cleanupKVBucket(t, client, "TEST-KV-CREATE-OP")
	store, err := client.CreateKeyValue(KVConfig{Bucket: "TEST-KV-CREATE-OP", Replicas: 1})
	if err != nil {
		t.Fatalf("CreateKeyValue() error = %v", err)
	}

	t.Run("create new key", func(t *testing.T) {
		rev, err := store.Create("inst.300", []byte("initial"))
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if rev == 0 {
			t.Error("Create() revision should be > 0")
		}
	})

	t.Run("create duplicate key returns ErrKVKeyExists", func(t *testing.T) {
		_, _ = store.Create("inst.301", []byte("first"))
		_, err := store.Create("inst.301", []byte("second"))
		if !errors.Is(err, ErrKVKeyExists) {
			t.Errorf("Create(duplicate) error = %v, want ErrKVKeyExists", err)
		}
	})
}

func TestKVUpdate_CAS_Integration(t *testing.T) {
	client := skipIfNoNATSClient(t)
	if client == nil {
		return
	}

	store, err := client.CreateKeyValue(KVConfig{Bucket: "TEST-KV-CAS", Replicas: 1})
	if err != nil {
		t.Fatalf("CreateKeyValue() error = %v", err)
	}

	t.Run("update with correct revision succeeds", func(t *testing.T) {
		rev1, _ := store.Put("inst.400", []byte("v1"))

		rev2, err := store.Update("inst.400", []byte("v2"), rev1)
		if err != nil {
			t.Fatalf("Update(correct rev) error = %v", err)
		}
		if rev2 <= rev1 {
			t.Errorf("new revision (%d) should be > old (%d)", rev2, rev1)
		}

		entry, _ := store.Get("inst.400")
		if string(entry.Value) != "v2" {
			t.Errorf("Value = %s, want v2", string(entry.Value))
		}
	})

	t.Run("update with wrong revision returns ErrKVCASConflict", func(t *testing.T) {
		store.Put("inst.401", []byte("v1"))

		_, err := store.Update("inst.401", []byte("v2"), 99999)
		if !errors.Is(err, ErrKVCASConflict) {
			t.Errorf("Update(wrong rev) error = %v, want ErrKVCASConflict", err)
		}
	})

	t.Run("concurrent CAS - only one wins", func(t *testing.T) {
		rev, _ := store.Put("inst.402", []byte("v0"))

		/* Simulate two workers reading the same revision */
		workerAEntry, _ := store.Get("inst.402")
		workerBEntry, _ := store.Get("inst.402")

		if workerAEntry.Revision != rev || workerBEntry.Revision != rev {
			t.Fatal("both workers should read same revision")
		}

		/* Worker A updates first — succeeds */
		_, err := store.Update("inst.402", []byte("workerA"), workerAEntry.Revision)
		if err != nil {
			t.Fatalf("Worker A Update() error = %v", err)
		}

		/* Worker B tries same revision — CAS conflict */
		_, err = store.Update("inst.402", []byte("workerB"), workerBEntry.Revision)
		if !errors.Is(err, ErrKVCASConflict) {
			t.Errorf("Worker B Update() error = %v, want ErrKVCASConflict", err)
		}

		/* Verify Worker A's value persisted */
		entry, _ := store.Get("inst.402")
		if string(entry.Value) != "workerA" {
			t.Errorf("Value = %s, want workerA (winner of CAS)", string(entry.Value))
		}
	})
}

func TestKVDelete_Integration(t *testing.T) {
	client := skipIfNoNATSClient(t)
	if client == nil {
		return
	}

	store, err := client.CreateKeyValue(KVConfig{Bucket: "TEST-KV-DELETE", Replicas: 1})
	if err != nil {
		t.Fatalf("CreateKeyValue() error = %v", err)
	}

	t.Run("delete existing key", func(t *testing.T) {
		store.Put("inst.500", []byte("data"))

		err := store.Delete("inst.500")
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		_, err = store.Get("inst.500")
		if !errors.Is(err, ErrKVKeyNotFound) {
			t.Errorf("Get(deleted) error = %v, want ErrKVKeyNotFound", err)
		}
	})

	t.Run("purge key", func(t *testing.T) {
		store.Put("inst.501", []byte("v1"))
		store.Put("inst.501", []byte("v2"))

		err := store.Purge("inst.501")
		if err != nil {
			t.Fatalf("Purge() error = %v", err)
		}

		_, err = store.Get("inst.501")
		if !errors.Is(err, ErrKVKeyNotFound) {
			t.Errorf("Get(purged) error = %v, want ErrKVKeyNotFound", err)
		}
	})
}

func TestKVKeys_Integration(t *testing.T) {
	client := skipIfNoNATSClient(t)
	if client == nil {
		return
	}

	cleanupKVBucket(t, client, "TEST-KV-KEYS")
	store, err := client.CreateKeyValue(KVConfig{Bucket: "TEST-KV-KEYS", Replicas: 1})
	if err != nil {
		t.Fatalf("CreateKeyValue() error = %v", err)
	}

	t.Run("empty bucket returns empty slice", func(t *testing.T) {
		keys, err := store.Keys()
		if err != nil {
			t.Fatalf("Keys() error = %v", err)
		}
		if len(keys) != 0 {
			t.Errorf("Keys() = %v, want empty", keys)
		}
	})

	t.Run("returns all keys", func(t *testing.T) {
		store.Put("inst.600", []byte("a"))
		store.Put("inst.601", []byte("b"))
		store.Put("inst.602", []byte("c"))

		/* Give NATS a moment to index */
		time.Sleep(100 * time.Millisecond)

		keys, err := store.Keys()
		if err != nil {
			t.Fatalf("Keys() error = %v", err)
		}
		if len(keys) != 3 {
			t.Errorf("Keys() count = %d, want 3", len(keys))
		}
	})
}

func TestKVPerStepPattern_Integration(t *testing.T) {
	client := skipIfNoNATSClient(t)
	if client == nil {
		return
	}

	cleanupKVBucket(t, client, "TEST-KV-PERSTEP")
	store, err := client.CreateKeyValue(KVConfig{Bucket: "TEST-KV-PERSTEP", Replicas: 1})
	if err != nil {
		t.Fatalf("CreateKeyValue() error = %v", err)
	}

	/* Simulates the per-step KV checkpoint pattern used by the workflow runtime:
	   1. Create instance (first step)
	   2. Put after each inline step (state advances)
	   3. After crash, Get returns last completed step
	   4. Recovery continues from next step */
	t.Run("per-step checkpoint simulation", func(t *testing.T) {
		key := "inst.perstep-1"

		/* Step 0: Create instance */
		rev0, err := store.Create(key, []byte(`{"step":0,"state":{}}`))
		if err != nil {
			t.Fatalf("step 0 Create() error = %v", err)
		}

		/* Step 1: set_state → KV Put */
		rev1, err := store.Update(key, []byte(`{"step":1,"state":{"counter":1}}`), rev0)
		if err != nil {
			t.Fatalf("step 1 Update() error = %v", err)
		}
		if rev1 <= rev0 {
			t.Errorf("rev1 (%d) should be > rev0 (%d)", rev1, rev0)
		}

		/* Step 2: condition → KV Put */
		rev2, err := store.Update(key, []byte(`{"step":2,"state":{"counter":1}}`), rev1)
		if err != nil {
			t.Fatalf("step 2 Update() error = %v", err)
		}

		/* Step 3: set_state(+1) → KV Put */
		rev3, err := store.Update(key, []byte(`{"step":3,"state":{"counter":2}}`), rev2)
		if err != nil {
			t.Fatalf("step 3 Update() error = %v", err)
		}

		/* === CRASH HERE === */
		/* Worker B picks up the instance from KV */
		entry, err := store.Get(key)
		if err != nil {
			t.Fatalf("recovery Get() error = %v", err)
		}

		/* Verify state is from step 3 (not step 0) */
		if string(entry.Value) != `{"step":3,"state":{"counter":2}}` {
			t.Errorf("recovery Value = %s, want step 3 state", string(entry.Value))
		}
		if entry.Revision != rev3 {
			t.Errorf("recovery Revision = %d, want %d (last step)", entry.Revision, rev3)
		}

		/* Worker B continues from step 4 using recovered revision */
		rev4, err := store.Update(key, []byte(`{"step":4,"state":{"counter":3}}`), entry.Revision)
		if err != nil {
			t.Fatalf("step 4 (recovery) Update() error = %v", err)
		}
		if rev4 <= rev3 {
			t.Errorf("rev4 (%d) should be > rev3 (%d)", rev4, rev3)
		}
	})
}
