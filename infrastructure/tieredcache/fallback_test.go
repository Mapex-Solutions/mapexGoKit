package tieredcache

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// Fallback Tests - Simulating L0 → L1 → L2 Chain
// =============================================================================

// TestFallback_L0Miss_L1Hit simulates:
// 1. Data exists in L1 (disk) but NOT in L0 (RAM)
// 2. Get should fallback to L1 and promote to L0
func TestFallback_L0Miss_L1Hit(t *testing.T) {
	cache, cleanup := createTestCache(t)
	defer cleanup()

	ctx := context.Background()
	key := "fallback-l1-test"
	value := []byte(`{"asset_id": "123", "name": "Test Asset", "template": "sensor"}`)

	// Step 1: Write directly to L1 (simulating data that was persisted but L0 evicted)
	prefixed := cache.prefixKey(key)
	err := cache.writeL1(prefixed, value, 5*time.Minute)
	if err != nil {
		t.Fatalf("writeL1() error = %v", err)
	}

	// Step 2: Verify L0 is empty
	_, found := cache.GetFromL0(prefixed)
	if found {
		t.Fatal("L0 should be empty before test")
	}

	// Step 3: Get - should fallback to L1
	data, tier, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// Verify L1 hit
	if tier != int(TierL1) {
		t.Errorf("Expected tier L1 (%d), got %d (%s)", TierL1, tier, CacheTier(tier).String())
	}

	// Verify data
	if string(data) != string(value) {
		t.Errorf("Data mismatch: got %s, want %s", data, value)
	}

	// Step 4: Verify promotion to L0
	time.Sleep(10 * time.Millisecond) // Give ristretto time to process
	data2, found := cache.GetFromL0(prefixed)
	if !found {
		t.Error("Data should be promoted to L0 after L1 hit")
	}
	if string(data2) != string(value) {
		t.Error("Promoted data should match original")
	}

	// Step 5: Second Get should hit L0
	_, tier2, _ := cache.Get(ctx, key)
	if tier2 != int(TierL0) {
		t.Errorf("Second Get should hit L0, got tier %d", tier2)
	}

	t.Logf("✓ Fallback L0→L1 working: First hit L1, second hit L0 (promoted)")
}

// TestFallback_L0Miss_L1Miss_L2Hit simulates:
// 1. Data exists ONLY in L2 (MinIO/S3)
// 2. Get should fallback through L0 → L1 → L2
// 3. Data should be promoted to L0 and L1
func TestFallback_L0Miss_L1Miss_L2Hit(t *testing.T) {
	ctx := context.Background()
	key := "fallback-l2-test"
	value := []byte(`{"template_id": "456", "script": "function process(data) { return data; }"}`)

	// Track L2 loader calls
	var l2CallCount atomic.Int32

	cache, cleanup := createTestCache(t, func(c *Config) {
		c.EnableL2 = true
		c.L2Loader = func(ctx context.Context, k string) ([]byte, error) {
			l2CallCount.Add(1)
			t.Logf("  → L2 Loader called for key: %s (call #%d)", k, l2CallCount.Load())

			if k == key {
				// Simulate network latency
				time.Sleep(5 * time.Millisecond)
				return value, nil
			}
			return nil, ErrCacheMiss
		}
	})
	defer cleanup()

	// Step 2: Verify L0 and L1 are empty
	prefixed := cache.prefixKey(key)
	_, foundL0 := cache.GetFromL0(prefixed)
	if foundL0 {
		t.Fatal("L0 should be empty before test")
	}

	_, errL1 := cache.GetFromL1(ctx, prefixed)
	if errL1 == nil {
		t.Fatal("L1 should be empty before test")
	}

	// Step 3: Get - should fallback to L2
	t.Log("First Get - expecting L2 hit...")
	data, tier, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// Verify L2 hit
	if tier != int(TierL2) {
		t.Errorf("Expected tier L2 (%d), got %d (%s)", TierL2, tier, CacheTier(tier).String())
	}

	// Verify data
	if string(data) != string(value) {
		t.Errorf("Data mismatch: got %s, want %s", data, value)
	}

	// Verify L2 was called exactly once
	if l2CallCount.Load() != 1 {
		t.Errorf("L2 loader should be called once, got %d", l2CallCount.Load())
	}

	// Step 4: Verify promotion to L0 and L1
	time.Sleep(10 * time.Millisecond)

	// Check L0
	dataL0, foundL0 := cache.GetFromL0(prefixed)
	if !foundL0 {
		t.Error("Data should be promoted to L0 after L2 hit")
	} else if string(dataL0) != string(value) {
		t.Error("L0 data should match original")
	}

	// Check L1
	dataL1, errL1 := cache.GetFromL1(ctx, prefixed)
	if errL1 != nil {
		t.Errorf("Data should be promoted to L1 after L2 hit: %v", errL1)
	} else if string(dataL1) != string(value) {
		t.Error("L1 data should match original")
	}

	// Step 5: Second Get should hit L0 (NOT call L2 again)
	t.Log("Second Get - expecting L0 hit (no L2 call)...")
	_, tier2, _ := cache.Get(ctx, key)
	if tier2 != int(TierL0) {
		t.Errorf("Second Get should hit L0, got tier %d", tier2)
	}

	if l2CallCount.Load() != 1 {
		t.Errorf("L2 loader should NOT be called again, total calls: %d", l2CallCount.Load())
	}

	t.Logf("✓ Fallback L0→L1→L2 working: First hit L2, promoted to L0+L1, second hit L0")
}

// TestFallback_L0Eviction_L1Survives simulates:
// 1. Data is in L0 and L1
// 2. L0 evicts the data (memory pressure)
// 3. Get should fallback to L1
func TestFallback_L0Eviction_L1Survives(t *testing.T) {
	// Create cache with very small L0 to force eviction
	tmpDir, err := os.MkdirTemp("", "eviction-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cache, err := New(Config{
		L0MaxSize:    1024,        // Very small: 1KB
		L0MaxItems:   10,          // Very few items
		L0DefaultTTL: time.Minute,
		L1Dir:        tmpDir,
		EnableL1:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Step 1: Store initial data
	key := "eviction-test"
	value := []byte(`{"important": "data that should survive L0 eviction"}`)
	_ = cache.Set(ctx, key, value, time.Minute)

	// Step 2: Fill L0 with other data to force eviction
	t.Log("Filling L0 to force eviction...")
	for i := 0; i < 100; i++ {
		largeValue := make([]byte, 200) // 200 bytes each
		_ = cache.Set(ctx, fmt.Sprintf("filler-%d", i), largeValue, time.Minute)
	}

	// Give ristretto time to evict
	time.Sleep(100 * time.Millisecond)

	// Step 3: Original key might be evicted from L0, but should be in L1
	prefixed := cache.prefixKey(key)
	dataL1, err := cache.GetFromL1(ctx, prefixed)
	if err != nil {
		t.Fatalf("L1 should still have the data: %v", err)
	}
	if string(dataL1) != string(value) {
		t.Error("L1 data should match original")
	}

	// Step 4: Get should work (either L0 or L1)
	data, tier, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() should succeed: %v", err)
	}
	if string(data) != string(value) {
		t.Error("Data should match original")
	}

	t.Logf("✓ After L0 pressure, data retrieved from tier: %s", CacheTier(tier).String())
}

// TestFallback_L1Expiration_L2Reload simulates:
// 1. Data is in L1 but expired
// 2. Get should detect expiration and fallback to L2
func TestFallback_L1Expiration_L2Reload(t *testing.T) {
	ctx := context.Background()
	key := "expiration-test"
	originalValue := []byte(`{"version": 1}`)
	updatedValue := []byte(`{"version": 2}`)

	// Track L2 calls
	var l2CallCount atomic.Int32

	cache, cleanup := createTestCache(t, func(c *Config) {
		c.EnableL2 = true
		c.L2Loader = func(ctx context.Context, k string) ([]byte, error) {
			l2CallCount.Add(1)
			if k == key {
				return updatedValue, nil
			}
			return nil, ErrCacheMiss
		}
	})
	defer cleanup()

	// Step 1: Write to L1 with very short TTL
	prefixed := cache.prefixKey(key)
	err := cache.writeL1(prefixed, originalValue, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	// Verify L1 has data
	dataL1, err := cache.readL1(prefixed)
	if err != nil {
		t.Fatalf("L1 should have data: %v", err)
	}
	if string(dataL1) != string(originalValue) {
		t.Error("L1 should have original value")
	}

	// Step 2: Wait for L1 expiration
	t.Log("Waiting for L1 expiration...")
	time.Sleep(100 * time.Millisecond)

	// Step 3: Get should detect expired L1 and fallback to L2
	data, tier, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// Should hit L2 because L1 expired
	if tier != int(TierL2) {
		t.Errorf("Expected tier L2 after L1 expiration, got %d (%s)", tier, CacheTier(tier).String())
	}

	// Should get updated value from L2
	if string(data) != string(updatedValue) {
		t.Errorf("Should get updated value from L2: got %s, want %s", data, updatedValue)
	}

	if l2CallCount.Load() != 1 {
		t.Errorf("L2 should be called once after L1 expiration, got %d", l2CallCount.Load())
	}

	t.Logf("✓ L1 expiration triggers L2 reload correctly")
}

// TestFallback_CompleteChain tests the complete fallback chain with stats
func TestFallback_CompleteChain(t *testing.T) {
	ctx := context.Background()

	// Set up test data in L2
	testData := map[string][]byte{
		"asset-1":    []byte(`{"id": "1", "type": "sensor"}`),
		"asset-2":    []byte(`{"id": "2", "type": "actuator"}`),
		"template-1": []byte(`function process(d) { return d.value * 2; }`),
	}

	var l2Calls atomic.Int32
	cache, cleanup := createTestCache(t, func(c *Config) {
		c.EnableL2 = true
		c.L2Loader = func(ctx context.Context, key string) ([]byte, error) {
			l2Calls.Add(1)
			if data, ok := testData[key]; ok {
				time.Sleep(2 * time.Millisecond) // Simulate network
				return data, nil
			}
			return nil, ErrCacheMiss
		}
	})
	defer cleanup()

	// Reset stats
	cache.stats = CacheStats{}

	t.Log("=== Testing Complete Fallback Chain ===")

	// First access - all should hit L2
	t.Log("\n1. First access (cold cache - expecting L2 hits):")
	for key := range testData {
		data, tier, err := cache.Get(ctx, key)
		if err != nil {
			t.Errorf("Get(%s) error: %v", key, err)
			continue
		}
		t.Logf("   %s: tier=%s, size=%d bytes", key, CacheTier(tier).String(), len(data))
	}

	stats := cache.Stats()
	t.Logf("   Stats: L0 hits=%d, L1 hits=%d, L2 hits=%d", stats.L0Hits, stats.L1Hits, stats.L2Hits)

	if stats.L2Hits != 3 {
		t.Errorf("Expected 3 L2 hits, got %d", stats.L2Hits)
	}

	// Second access - all should hit L0
	t.Log("\n2. Second access (warm cache - expecting L0 hits):")
	for key := range testData {
		_, tier, _ := cache.Get(ctx, key)
		t.Logf("   %s: tier=%s", key, CacheTier(tier).String())
	}

	stats = cache.Stats()
	t.Logf("   Stats: L0 hits=%d, L1 hits=%d, L2 hits=%d", stats.L0Hits, stats.L1Hits, stats.L2Hits)

	if stats.L0Hits != 3 {
		t.Errorf("Expected 3 L0 hits on second access, got %d", stats.L0Hits)
	}

	// Clear L0, access again - should hit L1
	t.Log("\n3. After L0 clear (expecting L1 hits):")
	for key := range testData {
		prefixed := cache.prefixKey(key)
		cache.l0Cache.Del(prefixed)
	}
	cache.l0Cache.Wait()

	for key := range testData {
		_, tier, _ := cache.Get(ctx, key)
		t.Logf("   %s: tier=%s", key, CacheTier(tier).String())
	}

	stats = cache.Stats()
	t.Logf("   Stats: L0 hits=%d, L1 hits=%d, L2 hits=%d", stats.L0Hits, stats.L1Hits, stats.L2Hits)

	if stats.L1Hits != 3 {
		t.Errorf("Expected 3 L1 hits after L0 clear, got %d", stats.L1Hits)
	}

	// Verify L2 was only called initially (3 times total)
	if l2Calls.Load() != 3 {
		t.Errorf("L2 should only be called 3 times total, got %d", l2Calls.Load())
	}

	t.Log("\n✓ Complete fallback chain working correctly!")
}

// TestFallback_ConcurrentAccess tests concurrent fallback behavior
func TestFallback_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	key := "concurrent-key"
	value := []byte(`{"concurrent": "test"}`)

	var l2Calls atomic.Int32

	cache, cleanup := createTestCache(t, func(c *Config) {
		c.EnableL2 = true
		c.L2Loader = func(ctx context.Context, k string) ([]byte, error) {
			l2Calls.Add(1)
			time.Sleep(10 * time.Millisecond) // Simulate slow network
			if k == key {
				return value, nil
			}
			return nil, ErrCacheMiss
		}
	})
	defer cleanup()

	// Launch many concurrent Gets
	const numGoroutines = 50
	results := make(chan int, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			_, tier, err := cache.Get(ctx, key)
			if err != nil {
				results <- -1
				return
			}
			results <- tier
		}()
	}

	// Collect results
	tierCounts := make(map[int]int)
	for i := 0; i < numGoroutines; i++ {
		tier := <-results
		tierCounts[tier]++
	}

	t.Logf("Concurrent access results (50 goroutines):")
	t.Logf("  L0 hits: %d", tierCounts[int(TierL0)])
	t.Logf("  L1 hits: %d", tierCounts[int(TierL1)])
	t.Logf("  L2 hits: %d", tierCounts[int(TierL2)])
	t.Logf("  L2 loader calls: %d", l2Calls.Load())

	// Most should hit L0 after first load
	// L2 might be called multiple times due to race, but should be limited
	if tierCounts[-1] > 0 {
		t.Errorf("%d requests failed", tierCounts[-1])
	}

	t.Log("✓ Concurrent fallback access completed without errors")
}

// =============================================================================
// Benchmark - Fallback Performance
// =============================================================================

func BenchmarkFallback_L1Hit(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "bench-fallback-*")
	defer os.RemoveAll(tmpDir)

	cache, _ := New(Config{
		L0MaxSize: 64 * 1024 * 1024,
		L1Dir:     tmpDir,
		EnableL1:  true,
	})
	defer cache.Close()

	ctx := context.Background()
	key := "bench-fallback"
	value := []byte(`{"benchmark": "fallback test data"}`)

	// Write to L1 only
	prefixed := cache.prefixKey(key)
	_ = cache.writeL1(prefixed, value, time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clear L0 to force L1 fallback
		cache.l0Cache.Del(prefixed)
		_, _, _ = cache.Get(ctx, key)
	}
}

func BenchmarkFallback_L2Hit(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "bench-fallback-l2-*")
	defer os.RemoveAll(tmpDir)

	key := "bench-l2"
	value := []byte(`{"benchmark": "L2 fallback test"}`)

	cache, _ := New(Config{
		L0MaxSize: 64 * 1024 * 1024,
		L1Dir:     tmpDir,
		EnableL1:  false, // Disable L1 to test L2 directly
		EnableL2:  true,
		L2Loader: func(ctx context.Context, k string) ([]byte, error) {
			if k == key {
				return value, nil
			}
			return nil, ErrCacheMiss
		},
	})
	defer cache.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clear L0 to force L2 fallback
		prefixed := cache.prefixKey(key)
		cache.l0Cache.Del(prefixed)
		_, _, _ = cache.Get(ctx, key)
	}
}
