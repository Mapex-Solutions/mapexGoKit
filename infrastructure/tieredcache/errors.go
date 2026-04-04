package tieredcache

import "errors"

var (
	// ErrCacheMiss is returned when the key is not found in any cache tier.
	ErrCacheMiss = errors.New("cache miss: key not found")

	// ErrL0InitFailed is returned when L0 (RAM) cache initialization fails.
	ErrL0InitFailed = errors.New("failed to initialize L0 RAM cache")

	// ErrL1InitFailed is returned when L1 (Disk) cache initialization fails.
	ErrL1InitFailed = errors.New("failed to initialize L1 disk cache")

	// ErrL1DirNotWritable is returned when L1 directory is not writable.
	ErrL1DirNotWritable = errors.New("L1 cache directory is not writable")

	// ErrNoLoader is returned when trying to load from L2 without a loader.
	ErrNoLoader = errors.New("no L2 loader configured")

	// ErrEntryExpired is returned when the cached entry has expired.
	ErrEntryExpired = errors.New("cache entry expired")

	// ErrInvalidConfig is returned when the configuration is invalid.
	ErrInvalidConfig = errors.New("invalid cache configuration")

	// ErrNilData is returned when attempting to cache nil data.
	ErrNilData = errors.New("cannot cache nil data")

	// ErrEmptyKey is returned when the cache key is empty.
	ErrEmptyKey = errors.New("cache key cannot be empty")
)
