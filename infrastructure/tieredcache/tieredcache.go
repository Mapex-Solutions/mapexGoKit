package tieredcache

import (
	"fmt"
	"os"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
	"github.com/Mapex-Solutions/mapexGoKit/infrastructure/httpclient"
)

// New creates a new TieredCache client with optional L0 (RAM), L1 (Disk), and L2 (Remote).
//
// The cache follows a tiered architecture:
//   - L0 (RAM): Ultra-fast in-memory cache using ristretto (~50µs)
//   - L1 (Disk): Fast local NVMe/SSD storage (~500µs)
//   - L2 (Remote): MinIO/S3 source of truth (configured via Config.L2Loader)
//
// Critical behavior:
//   - L0 is enabled by default but can be disabled via config.EnableL0
//   - L1 is enabled by default but can be disabled via config.EnableL1
//   - L2 loader is configured via Config.EnableL2 and Config.L2Loader
//   - At least one cache layer (L0, L1, or L2 loader) must be enabled
//   - Cache follows LRU eviction with TTL support
func New(config Config) (*TieredCacheClient, error) {
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	// Apply defaults
	applyDefaults(&config)

	client := &TieredCacheClient{
		config:    config,
		keyPrefix: config.KeyPrefix,
	}

	// Initialize L0 (RAM) cache with ristretto if enabled
	if config.EnableL0 {
		l0Cache, err := ristretto.NewCache(&ristretto.Config{
			NumCounters: RistrettoNumCounters,
			MaxCost:     config.L0MaxSize,
			BufferItems: L0BufferItems,
			Metrics:     config.EnableMetrics,
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrL0InitFailed, err)
		}
		client.l0Cache = l0Cache
	}

	// Initialize L1 (Disk) cache if enabled
	if config.EnableL1 {
		if err := client.initL1Cache(config.L1Dir); err != nil {
			if client.l0Cache != nil {
				client.l0Cache.Close()
			}
			return nil, fmt.Errorf("%w: %v", ErrL1InitFailed, err)
		}
		client.l1Dir = config.L1Dir
	}

	// Initialize L2 (Remote) loader if enabled
	if config.EnableL2 {
		if config.L2Loader == nil {
			return nil, fmt.Errorf("%w: L2Loader is required when EnableL2 is true", ErrInvalidConfig)
		}
		client.l2Loader = config.L2Loader
	}

	// Initialize Fallback HTTP client if configured
	if config.FallbackBaseURL != "" {
		fallbackTimeout := config.FallbackTimeout
		if fallbackTimeout == 0 {
			fallbackTimeout = 5 * time.Second
		}
		client.fallbackClient = httpclient.New(httpclient.Config{
			BaseURL: config.FallbackBaseURL,
			APIKey:  config.FallbackAPIKey,
			Timeout: fallbackTimeout,
		})
		client.fallbackEndpoint = config.FallbackEndpoint
	}

	logger.Info(fmt.Sprintf("[INFRA:CACHE] Initialized (L0: %v, L1: %v, L2: %v, Fallback: %v)",
		config.EnableL0,
		config.EnableL1,
		config.EnableL2,
		config.FallbackBaseURL != "",
	))

	return client, nil
}

// validateConfig validates the cache configuration.
func validateConfig(config *Config) error {
	if config.L0MaxSize < 0 {
		return fmt.Errorf("L0MaxSize cannot be negative")
	}
	if config.L0MaxItems < 0 {
		return fmt.Errorf("L0MaxItems cannot be negative")
	}
	if config.EnableL1 && config.L1MaxSize < 0 {
		return fmt.Errorf("L1MaxSize cannot be negative")
	}
	if config.EnableL2 && config.L2Loader == nil {
		return fmt.Errorf("L2Loader is required when EnableL2 is true")
	}
	return nil
}

// applyDefaults applies default values to unset configuration fields.
//
// Layer enabling behavior:
//   - EnableL0 and EnableL1 use Go's zero value (false) as default
//   - User must explicitly set EnableL0: true or EnableL1: true to enable layers
//   - Size/TTL defaults are applied regardless, so config is ready if layer is enabled later
//
// Example configs:
//   - Full cache (L0+L1): Config{EnableL0: true, EnableL1: true}
//   - L1+L2 only:         Config{EnableL0: false, EnableL1: true}  // Asset Template case
//   - L0+L2 only:         Config{EnableL0: true, EnableL1: false}
func applyDefaults(config *Config) {
	// Apply L0 size defaults (used if L0 is enabled)
	if config.L0MaxSize == 0 {
		config.L0MaxSize = DefaultL0MaxSize
	}
	if config.L0MaxItems == 0 {
		config.L0MaxItems = DefaultL0MaxItems
	}
	if config.L0DefaultTTL == 0 {
		config.L0DefaultTTL = DefaultL0TTL
	}

	// Apply L1 size defaults (used if L1 is enabled)
	if config.L1Dir == "" {
		config.L1Dir = DefaultL1Dir
	}
	if config.L1MaxSize == 0 {
		config.L1MaxSize = DefaultL1MaxSize
	}
	if config.L1DefaultTTL == 0 {
		config.L1DefaultTTL = DefaultL1TTL
	}
}

// initL1Cache initializes the L1 disk cache directory.
func (c *TieredCacheClient) initL1Cache(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create L1 directory: %w", err)
	}

	// Verify directory is writable
	testFile := dir + "/.write_test"
	f, err := os.Create(testFile)
	if err != nil {
		return ErrL1DirNotWritable
	}
	f.Close()
	os.Remove(testFile)

	return nil
}

// Close closes the cache and releases all resources.
func (c *TieredCacheClient) Close() {
	if c.l0Cache != nil {
		c.l0Cache.Close()
	}
	logger.Info("[INFRA:CACHE] Closed")
}
