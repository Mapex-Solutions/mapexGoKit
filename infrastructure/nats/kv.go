package natsModel

import (
	"errors"
	"fmt"

	"github.com/nats-io/nats.go"

	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

/**
Sentinel errors for KV operations.
Services should check these with errors.Is() for typed error handling.
*/

var (
	// ErrKVKeyNotFound is returned when a Get or Delete targets a non-existent key.
	ErrKVKeyNotFound = errors.New("kv: key not found")

	// ErrKVKeyExists is returned when Create targets a key that already exists.
	ErrKVKeyExists = errors.New("kv: key already exists")

	// ErrKVCASConflict is returned when Update detects a revision mismatch (CAS failure).
	// Another writer modified the key between your Get and Update.
	ErrKVCASConflict = errors.New("kv: CAS conflict (revision mismatch)")
)

/**
KVStore wraps a NATS KeyValue bucket with typed errors and CAS support.

Usage:

	store, err := client.CreateKeyValue(natsModel.KVConfig{
	    Bucket:   "WORKFLOW-INSTANCES",
	    Replicas: 3,
	})

	// Put (create or overwrite)
	rev, _ := store.Put("inst:123", data)

	// Get + CAS Update
	entry, _ := store.Get("inst:123")
	newRev, err := store.Update("inst:123", newData, entry.Revision)
	if errors.Is(err, natsModel.ErrKVCASConflict) {
	    // Another writer modified the key — re-read and retry
	}
*/
type KVStore struct {
	kv nats.KeyValue
}

/* Compile-time interface check */
var _ KeyValueStore = (*KVStore)(nil)

// CreateKeyValue creates a NATS KV bucket (or binds to an existing one) and returns a KVStore.
// The bucket is file-backed by default (survives NATS restarts).
func (c *Client) CreateKeyValue(cfg KVConfig) (*KVStore, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("kv: bucket name is required")
	}

	/* Defaults */
	replicas := cfg.Replicas
	if replicas <= 0 {
		replicas = 1
	}
	history := cfg.History
	if history == 0 {
		history = 1
	}
	storage := cfg.Storage
	if storage == 0 {
		storage = nats.FileStorage
	}

	kvCfg := &nats.KeyValueConfig{
		Bucket:       cfg.Bucket,
		Description:  cfg.Description,
		TTL:          cfg.TTL,
		MaxBytes:     cfg.MaxBytes,
		MaxValueSize: cfg.MaxValueSize,
		History:      history,
		Replicas:     replicas,
		Storage:      storage,
	}

	kv, err := c.js.CreateKeyValue(kvCfg)
	if err != nil {
		return nil, fmt.Errorf("kv: failed to create bucket %q: %w", cfg.Bucket, err)
	}

	logger.Info(fmt.Sprintf("[INFRA:NATS:KV] Bucket %q ready (replicas=%d, storage=%v)", cfg.Bucket, replicas, storage))
	return &KVStore{kv: kv}, nil
}

// Get retrieves a value by key. Returns ErrKVKeyNotFound if the key doesn't exist.
func (s *KVStore) Get(key string) (*KVEntry, error) {
	entry, err := s.kv.Get(key)
	if err != nil {
		if errors.Is(err, nats.ErrKeyNotFound) {
			return nil, ErrKVKeyNotFound
		}
		return nil, fmt.Errorf("kv: get %q: %w", key, err)
	}
	return &KVEntry{
		Key:      entry.Key(),
		Value:    entry.Value(),
		Revision: entry.Revision(),
		Created:  entry.Created(),
	}, nil
}

// Put stores a value, creating or overwriting the key. Returns the new revision.
func (s *KVStore) Put(key string, value []byte) (uint64, error) {
	rev, err := s.kv.Put(key, value)
	if err != nil {
		return 0, fmt.Errorf("kv: put %q: %w", key, err)
	}
	return rev, nil
}

// Create stores a value ONLY IF the key doesn't exist.
// Returns ErrKVKeyExists if the key already exists.
func (s *KVStore) Create(key string, value []byte) (uint64, error) {
	rev, err := s.kv.Create(key, value)
	if err != nil {
		if errors.Is(err, nats.ErrKeyExists) {
			return 0, ErrKVKeyExists
		}
		return 0, fmt.Errorf("kv: create %q: %w", key, err)
	}
	return rev, nil
}

// Update stores a value ONLY IF the current revision matches expectedRevision (CAS).
// Returns ErrKVCASConflict if the revision doesn't match (another writer modified the key).
func (s *KVStore) Update(key string, value []byte, expectedRevision uint64) (uint64, error) {
	rev, err := s.kv.Update(key, value, expectedRevision)
	if err != nil {
		/* NATS KV returns a JetStream API error (code 10071: wrong last sequence)
		   when the expected revision doesn't match. We also check for ErrKeyExists
		   which some nats.go versions return for CAS failures. */
		var apiErr *nats.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode == 10071 {
			return 0, ErrKVCASConflict
		}
		if errors.Is(err, nats.ErrKeyExists) {
			return 0, ErrKVCASConflict
		}
		return 0, fmt.Errorf("kv: update %q (rev=%d): %w", key, expectedRevision, err)
	}
	return rev, nil
}

// Delete soft-deletes a key (preservable in history). Returns ErrKVKeyNotFound if not found.
func (s *KVStore) Delete(key string) error {
	err := s.kv.Delete(key)
	if err != nil {
		if errors.Is(err, nats.ErrKeyNotFound) {
			return ErrKVKeyNotFound
		}
		return fmt.Errorf("kv: delete %q: %w", key, err)
	}
	return nil
}

// Purge removes a key and all its historical revisions.
func (s *KVStore) Purge(key string) error {
	err := s.kv.Purge(key)
	if err != nil {
		return fmt.Errorf("kv: purge %q: %w", key, err)
	}
	return nil
}

// Keys returns all keys in the bucket. Use sparingly — scans entire bucket.
func (s *KVStore) Keys() ([]string, error) {
	keys, err := s.kv.Keys()
	if err != nil {
		if errors.Is(err, nats.ErrNoKeysFound) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("kv: keys: %w", err)
	}
	return keys, nil
}

// Bucket returns the name of the underlying KV bucket.
func (s *KVStore) Bucket() string {
	return s.kv.Bucket()
}
