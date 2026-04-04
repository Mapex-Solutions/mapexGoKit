package tieredcache

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	common "github.com/Mapex-Solutions/mapexGoKit/infrastructure/common/ports"
)

// =============================================================================
// Test Helpers
// =============================================================================

// createTestCache creates a cache for testing with a temporary L1 directory.
func createTestCache(t *testing.T, opts ...func(*Config)) (*TieredCacheClient, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "tieredcache-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	config := Config{
		EnableL0:     true, // Explicitly enable L0 for tests
		L0MaxSize:    64 * 1024 * 1024, // 64MB for tests
		L0MaxItems:   10000,
		L0DefaultTTL: 1 * time.Minute,
		EnableL1:     true,
		L1Dir:        tmpDir,
		L1MaxSize:    128 * 1024 * 1024, // 128MB for tests
		L1DefaultTTL: 5 * time.Minute,
	}

	// Apply optional config modifications
	for _, opt := range opts {
		opt(&config)
	}

	cache, err := New(config)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create cache: %v", err)
	}

	cleanup := func() {
		cache.Close()
		os.RemoveAll(tmpDir)
	}

	return cache, cleanup
}

// mockLoader creates a mock L2 loader for testing.
func mockLoader(data map[string][]byte) common.LocalCacheLoader {
	return func(ctx context.Context, key string) ([]byte, error) {
		if d, ok := data[key]; ok {
			return d, nil
		}
		return nil, ErrCacheMiss
	}
}

// =============================================================================
// Unit Tests - Configuration
// =============================================================================

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:    "valid default config",
			config:  Config{},
			wantErr: false,
		},
		{
			name: "negative L0MaxSize",
			config: Config{
				L0MaxSize: -1,
			},
			wantErr: true,
		},
		{
			name: "negative L0MaxItems",
			config: Config{
				L0MaxItems: -1,
			},
			wantErr: true,
		},
		{
			name: "negative L1MaxSize with L1 enabled",
			config: Config{
				EnableL1:  true,
				L1MaxSize: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(&tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	config := Config{}
	applyDefaults(&config)

	if config.L0MaxSize != DefaultL0MaxSize {
		t.Errorf("L0MaxSize = %d, want %d", config.L0MaxSize, DefaultL0MaxSize)
	}
	if config.L0MaxItems != DefaultL0MaxItems {
		t.Errorf("L0MaxItems = %d, want %d", config.L0MaxItems, DefaultL0MaxItems)
	}
	if config.L0DefaultTTL != DefaultL0TTL {
		t.Errorf("L0DefaultTTL = %v, want %v", config.L0DefaultTTL, DefaultL0TTL)
	}
	if config.L1Dir != DefaultL1Dir {
		t.Errorf("L1Dir = %s, want %s", config.L1Dir, DefaultL1Dir)
	}
	if config.L1MaxSize != DefaultL1MaxSize {
		t.Errorf("L1MaxSize = %d, want %d", config.L1MaxSize, DefaultL1MaxSize)
	}
	if config.L1DefaultTTL != DefaultL1TTL {
		t.Errorf("L1DefaultTTL = %v, want %v", config.L1DefaultTTL, DefaultL1TTL)
	}
}

// =============================================================================
// Unit Tests - Key Operations
// =============================================================================

func TestPrefixKey(t *testing.T) {
	tests := []struct {
		name      string
		keyPrefix string
		key       string
		want      string
	}{
		{
			name:      "with prefix",
			keyPrefix: "assets",
			key:       "123",
			want:      "assets:123",
		},
		{
			name:      "without prefix",
			keyPrefix: "",
			key:       "123",
			want:      "123",
		},
		{
			name:      "complex key",
			keyPrefix: "templates",
			key:       "user/script.js",
			want:      "templates:user/script.js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := &TieredCacheClient{keyPrefix: tt.keyPrefix}
			got := cache.prefixKey(tt.key)
			if got != tt.want {
				t.Errorf("prefixKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHashKey(t *testing.T) {
	t.Run("consistent hashing", func(t *testing.T) {
		key := "test-key"
		hash1 := hashKey(key)
		hash2 := hashKey(key)

		if hash1 != hash2 {
			t.Error("hashKey should return consistent results")
		}
	})

	t.Run("different keys produce different hashes", func(t *testing.T) {
		hash1 := hashKey("key1")
		hash2 := hashKey("key2")

		if hash1 == hash2 {
			t.Error("different keys should produce different hashes")
		}
	})

	t.Run("hash length is 32 chars", func(t *testing.T) {
		hash := hashKey("test")
		if len(hash) != 32 {
			t.Errorf("hash length = %d, want 32", len(hash))
		}
	})
}

// =============================================================================
// Unit Tests - CacheTier
// =============================================================================

func TestCacheTierString(t *testing.T) {
	tests := []struct {
		tier CacheTier
		want string
	}{
		{TierMiss, "MISS"},
		{TierL0, "L0-RAM"},
		{TierL1, "L1-Disk"},
		{TierL2, "L2-Remote"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.tier.String(); got != tt.want {
				t.Errorf("CacheTier.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Unit Tests - Error Types
// =============================================================================

func TestErrorTypes(t *testing.T) {
	errors := []error{
		ErrCacheMiss,
		ErrL0InitFailed,
		ErrL1InitFailed,
		ErrL1DirNotWritable,
		ErrNoLoader,
		ErrEntryExpired,
		ErrInvalidConfig,
		ErrNilData,
		ErrEmptyKey,
	}

	for _, err := range errors {
		if err == nil {
			t.Error("Error should not be nil")
		}
		if err.Error() == "" {
			t.Error("Error message should not be empty")
		}
	}
}

// =============================================================================
// Integration Tests - Basic Operations
// =============================================================================

func TestNew(t *testing.T) {
	cache, cleanup := createTestCache(t)
	defer cleanup()

	if cache == nil {
		t.Fatal("New() returned nil cache")
	}

	if cache.l0Cache == nil {
		t.Error("L0 cache should be initialized")
	}
}

func TestNew_DisabledL1(t *testing.T) {
	cache, cleanup := createTestCache(t, func(c *Config) {
		c.EnableL1 = false
	})
	defer cleanup()

	if cache.isL1Enabled() {
		t.Error("L1 should be disabled")
	}
}

func TestSetAndGet_L0Only(t *testing.T) {
	cache, cleanup := createTestCache(t, func(c *Config) {
		c.EnableL1 = false
	})
	defer cleanup()

	ctx := context.Background()
	key := "test-key"
	value := []byte("test-value")

	// Set
	err := cache.Set(ctx, key, value, 1*time.Minute)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Get - should hit L0
	data, tier, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if tier != int(TierL0) {
		t.Errorf("Get() tier = %d, want %d (L0)", tier, TierL0)
	}

	if string(data) != string(value) {
		t.Errorf("Get() data = %s, want %s", data, value)
	}
}

func TestSetAndGet_WithL1(t *testing.T) {
	cache, cleanup := createTestCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "test-key-l1"
	value := []byte("test-value-for-l1-cache")

	// Set
	err := cache.Set(ctx, key, value, 1*time.Minute)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Verify L1 file exists
	prefixed := cache.prefixKey(key)
	l1Path := cache.l1FilePath(prefixed)
	if _, err := os.Stat(l1Path); os.IsNotExist(err) {
		t.Error("L1 cache file should exist")
	}

	// Clear L0 and verify L1 fallback
	cache.l0Cache.Del(prefixed)
	cache.l0Cache.Wait()

	// Get - should hit L1
	data, tier, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() from L1 error = %v", err)
	}

	if tier != int(TierL1) {
		t.Errorf("Get() tier = %d, want %d (L1)", tier, TierL1)
	}

	if string(data) != string(value) {
		t.Errorf("Get() data = %s, want %s", data, value)
	}
}

func TestSetAndGet_WithL2Loader(t *testing.T) {
	key := "remote-key"
	value := []byte("remote-value")

	cache, cleanup := createTestCache(t, func(c *Config) {
		c.EnableL1 = false // Disable L1 to test L2 directly
		c.EnableL2 = true
		c.L2Loader = mockLoader(map[string][]byte{key: value})
	})
	defer cleanup()

	ctx := context.Background()

	// Get - should hit L2 (via loader)
	data, tier, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if tier != int(TierL2) {
		t.Errorf("Get() tier = %d, want %d (L2)", tier, TierL2)
	}

	if string(data) != string(value) {
		t.Errorf("Get() data = %s, want %s", data, value)
	}

	// Second Get should hit L0 (promoted)
	data, tier, err = cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if tier != int(TierL0) {
		t.Errorf("Second Get() tier = %d, want %d (L0)", tier, TierL0)
	}
}

func TestGet_CacheMiss(t *testing.T) {
	cache, cleanup := createTestCache(t)
	defer cleanup()

	ctx := context.Background()
	_, tier, err := cache.Get(ctx, "nonexistent-key")

	if err != ErrCacheMiss {
		t.Errorf("Get() error = %v, want ErrCacheMiss", err)
	}

	if tier != int(TierMiss) {
		t.Errorf("Get() tier = %d, want %d (TierMiss)", tier, TierMiss)
	}
}

func TestGet_EmptyKey(t *testing.T) {
	cache, cleanup := createTestCache(t)
	defer cleanup()

	ctx := context.Background()
	_, _, err := cache.Get(ctx, "")

	if err != ErrEmptyKey {
		t.Errorf("Get() error = %v, want ErrEmptyKey", err)
	}
}

func TestSet_EmptyKey(t *testing.T) {
	cache, cleanup := createTestCache(t)
	defer cleanup()

	ctx := context.Background()
	err := cache.Set(ctx, "", []byte("data"), 1*time.Minute)

	if err != ErrEmptyKey {
		t.Errorf("Set() error = %v, want ErrEmptyKey", err)
	}
}

func TestSet_NilData(t *testing.T) {
	cache, cleanup := createTestCache(t)
	defer cleanup()

	ctx := context.Background()
	err := cache.Set(ctx, "key", nil, 1*time.Minute)

	if err != ErrNilData {
		t.Errorf("Set() error = %v, want ErrNilData", err)
	}
}

func TestDelete(t *testing.T) {
	cache, cleanup := createTestCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "delete-test"
	value := []byte("to-be-deleted")

	// Set
	err := cache.Set(ctx, key, value, 1*time.Minute)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Delete
	err = cache.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deleted
	_, _, err = cache.Get(ctx, key)
	if err != ErrCacheMiss {
		t.Errorf("Get() after delete error = %v, want ErrCacheMiss", err)
	}

	// Verify L1 file removed
	prefixed := cache.prefixKey(key)
	l1Path := cache.l1FilePath(prefixed)
	if _, err := os.Stat(l1Path); !os.IsNotExist(err) {
		t.Error("L1 cache file should not exist after delete")
	}
}

func TestInvalidate(t *testing.T) {
	cache, cleanup := createTestCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "invalidate-test"
	value := []byte("to-be-invalidated")

	// Set
	_ = cache.Set(ctx, key, value, 1*time.Minute)

	// Invalidate (should be same as Delete)
	err := cache.Invalidate(ctx, key)
	if err != nil {
		t.Fatalf("Invalidate() error = %v", err)
	}

	// Verify invalidated
	_, _, err = cache.Get(ctx, key)
	if err != ErrCacheMiss {
		t.Errorf("Get() after invalidate error = %v, want ErrCacheMiss", err)
	}
}

// =============================================================================
// Integration Tests - Statistics
// =============================================================================

func TestStats(t *testing.T) {
	cache, cleanup := createTestCache(t)
	defer cleanup()

	ctx := context.Background()

	// Set a value
	_ = cache.Set(ctx, "stats-key", []byte("value"), 1*time.Minute)

	// Get it (L0 hit)
	_, _, _ = cache.Get(ctx, "stats-key")

	// Get nonexistent (miss)
	_, _, _ = cache.Get(ctx, "nonexistent")

	stats := cache.Stats()

	if stats.L0Hits < 1 {
		t.Errorf("Stats().L0Hits = %d, want >= 1", stats.L0Hits)
	}

	if stats.L0Misses < 1 {
		t.Errorf("Stats().L0Misses = %d, want >= 1", stats.L0Misses)
	}
}

// =============================================================================
// Integration Tests - Direct Tier Access
// =============================================================================

func TestGetFromL0(t *testing.T) {
	cache, cleanup := createTestCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "l0-direct"
	value := []byte("l0-value")

	_ = cache.Set(ctx, key, value, 1*time.Minute)

	prefixed := cache.prefixKey(key)
	data, found := cache.GetFromL0(prefixed)

	if !found {
		t.Error("GetFromL0() should find the key")
	}

	if string(data) != string(value) {
		t.Errorf("GetFromL0() data = %s, want %s", data, value)
	}
}

func TestGetFromL1(t *testing.T) {
	cache, cleanup := createTestCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "l1-direct"
	value := []byte("l1-value")

	_ = cache.Set(ctx, key, value, 1*time.Minute)

	prefixed := cache.prefixKey(key)
	data, err := cache.GetFromL1(ctx, prefixed)

	if err != nil {
		t.Fatalf("GetFromL1() error = %v", err)
	}

	if string(data) != string(value) {
		t.Errorf("GetFromL1() data = %s, want %s", data, value)
	}
}

func TestGetFromL1_Disabled(t *testing.T) {
	cache, cleanup := createTestCache(t, func(c *Config) {
		c.EnableL1 = false
	})
	defer cleanup()

	ctx := context.Background()
	_, err := cache.GetFromL1(ctx, "any-key")

	if err != ErrCacheMiss {
		t.Errorf("GetFromL1() with disabled L1 error = %v, want ErrCacheMiss", err)
	}
}

// =============================================================================
// Integration Tests - Warmup
// =============================================================================

func TestWarmup(t *testing.T) {
	// Set up L2 loader with test data
	testData := map[string][]byte{
		"warmup-1": []byte("value-1"),
		"warmup-2": []byte("value-2"),
		"warmup-3": []byte("value-3"),
	}

	cache, cleanup := createTestCache(t, func(c *Config) {
		c.EnableL2 = true
		c.L2Loader = mockLoader(testData)
	})
	defer cleanup()

	ctx := context.Background()

	// Warmup
	err := cache.Warmup(ctx, []string{"warmup-1", "warmup-2", "warmup-3"})
	if err != nil {
		t.Fatalf("Warmup() error = %v", err)
	}

	// Verify all keys are now in L0
	for key := range testData {
		_, tier, err := cache.Get(ctx, key)
		if err != nil {
			t.Errorf("Get(%s) error = %v", key, err)
		}
		if tier != int(TierL0) {
			t.Errorf("Get(%s) tier = %d, want L0", key, tier)
		}
	}
}

func TestWarmup_NoLoader(t *testing.T) {
	cache, cleanup := createTestCache(t)
	defer cleanup()

	ctx := context.Background()
	err := cache.Warmup(ctx, []string{"key"})

	if err != ErrNoLoader {
		t.Errorf("Warmup() without loader error = %v, want ErrNoLoader", err)
	}
}

// =============================================================================
// Integration Tests - GetOrLoad
// =============================================================================

func TestGetOrLoad(t *testing.T) {
	cache, cleanup := createTestCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "load-test"
	value := []byte("loaded-value")

	// Use inline loader
	data, err := cache.GetOrLoad(ctx, key, func(ctx context.Context, k string) ([]byte, error) {
		return value, nil
	})

	if err != nil {
		t.Fatalf("GetOrLoad() error = %v", err)
	}

	if string(data) != string(value) {
		t.Errorf("GetOrLoad() data = %s, want %s", data, value)
	}

	// Second call should hit cache
	data2, err := cache.GetOrLoad(ctx, key, nil)
	if err != nil {
		t.Fatalf("GetOrLoad() second call error = %v", err)
	}

	if string(data2) != string(value) {
		t.Errorf("GetOrLoad() cached data = %s, want %s", data2, value)
	}
}

func TestGetOrLoad_NoLoader(t *testing.T) {
	cache, cleanup := createTestCache(t)
	defer cleanup()

	ctx := context.Background()
	_, err := cache.GetOrLoad(ctx, "key", nil)

	if err != ErrNoLoader {
		t.Errorf("GetOrLoad() without loader error = %v, want ErrNoLoader", err)
	}
}

// =============================================================================
// Integration Tests - L1 Expiration
// =============================================================================

func TestL1Expiration(t *testing.T) {
	cache, cleanup := createTestCache(t, func(c *Config) {
		c.L1DefaultTTL = 100 * time.Millisecond // Very short TTL for test
	})
	defer cleanup()

	ctx := context.Background()
	key := "expire-test"
	value := []byte("expiring-value")

	// Set with short TTL
	_ = cache.Set(ctx, key, value, 100*time.Millisecond)

	// Clear L0
	prefixed := cache.prefixKey(key)
	cache.l0Cache.Del(prefixed)
	cache.l0Cache.Wait()

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Try to read from L1 - should fail due to expiration
	_, err := cache.readL1(prefixed)
	if err != ErrEntryExpired && err != ErrCacheMiss {
		t.Errorf("readL1() after expiration error = %v, want ErrEntryExpired or ErrCacheMiss", err)
	}
}

// =============================================================================
// Integration Tests - Concurrent Access
// =============================================================================

func TestConcurrentAccess(t *testing.T) {
	cache, cleanup := createTestCache(t)
	defer cleanup()

	ctx := context.Background()
	const numGoroutines = 100
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := "concurrent-key"
				value := []byte("concurrent-value")

				// Mix of operations
				switch j % 3 {
				case 0:
					_ = cache.Set(ctx, key, value, 1*time.Minute)
				case 1:
					_, _, _ = cache.Get(ctx, key)
				case 2:
					_ = cache.Delete(ctx, key)
				}
			}
		}(i)
	}

	wg.Wait()
	// If we get here without panic/deadlock, concurrent access is safe
}

// =============================================================================
// Integration Tests - L1 Directory Structure
// =============================================================================

func TestL1DirectoryStructure(t *testing.T) {
	cache, cleanup := createTestCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "dir-test"
	value := []byte("value")

	_ = cache.Set(ctx, key, value, 1*time.Minute)

	prefixed := cache.prefixKey(key)
	l1Path := cache.l1FilePath(prefixed)

	// Verify 2-level subdirectory structure
	// Path should be: /cache/a1/b2/hash.cache
	dir := filepath.Dir(l1Path)           // /cache/a1/b2
	level2 := filepath.Base(dir)          // b2
	level1 := filepath.Base(filepath.Dir(dir)) // a1

	if len(level1) != 2 {
		t.Errorf("L1 level1 subdirectory name length = %d, want 2", len(level1))
	}
	if len(level2) != 2 {
		t.Errorf("L1 level2 subdirectory name length = %d, want 2", len(level2))
	}

	// Verify file extension
	if filepath.Ext(l1Path) != L1FileExtension {
		t.Errorf("L1 file extension = %s, want %s", filepath.Ext(l1Path), L1FileExtension)
	}

	t.Logf("L1 path structure: .../%s/%s/%s", level1, level2, filepath.Base(l1Path))
}

// =============================================================================
// Integration Tests - Lazy Cleanup (on-read expiration)
// =============================================================================

func TestLazyCleanup(t *testing.T) {
	cache, cleanup := createTestCache(t)
	defer cleanup()

	ctx := context.Background()

	// Write entry with very short TTL
	key := "lazy-cleanup-test"
	_ = cache.Set(ctx, key, []byte("data"), 50*time.Millisecond)

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Clear L0 to force L1 read
	prefixed := cache.prefixKey(key)
	cache.l0Cache.Del(prefixed)
	cache.l0Cache.Wait()

	// Try to read - should trigger lazy cleanup
	_, _, err := cache.Get(ctx, key)
	if err != ErrCacheMiss && err != ErrEntryExpired {
		t.Errorf("Expected ErrCacheMiss or ErrEntryExpired, got %v", err)
	}

	stats := cache.Stats()
	if stats.L1LazyExpired < 1 {
		t.Errorf("Expected at least 1 lazy expired file, got %d", stats.L1LazyExpired)
	}

	t.Logf("✓ Lazy cleanup: %d expired files removed on read", stats.L1LazyExpired)
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkSet(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "bench-cache-*")
	defer os.RemoveAll(tmpDir)

	cache, _ := New(Config{
		L0MaxSize: 64 * 1024 * 1024,
		L1Dir:     tmpDir,
		EnableL1:  true,
	})
	defer cache.Close()

	ctx := context.Background()
	value := []byte("benchmark-value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.Set(ctx, "bench-key", value, 1*time.Minute)
	}
}

func BenchmarkGet_L0Hit(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "bench-cache-*")
	defer os.RemoveAll(tmpDir)

	cache, _ := New(Config{
		L0MaxSize: 64 * 1024 * 1024,
		L1Dir:     tmpDir,
		EnableL1:  true,
	})
	defer cache.Close()

	ctx := context.Background()
	_ = cache.Set(ctx, "bench-key", []byte("value"), 1*time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = cache.Get(ctx, "bench-key")
	}
}

func BenchmarkGet_L1Hit(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "bench-cache-*")
	defer os.RemoveAll(tmpDir)

	cache, _ := New(Config{
		L0MaxSize: 64 * 1024 * 1024,
		L1Dir:     tmpDir,
		EnableL1:  true,
	})
	defer cache.Close()

	ctx := context.Background()
	key := "bench-key-l1"
	_ = cache.Set(ctx, key, []byte("value"), 1*time.Minute)

	// Clear L0 to force L1 reads
	prefixed := cache.prefixKey(key)
	cache.l0Cache.Del(prefixed)
	cache.l0Cache.Wait()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = cache.Get(ctx, key)
		// Clear L0 again for next iteration
		cache.l0Cache.Del(prefixed)
	}
}
