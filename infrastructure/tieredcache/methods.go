package tieredcache

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	common "github.com/Mapex-Solutions/mapexGoKit/infrastructure/common/ports"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// Compile-time interface compliance verification.
var (
	_ common.LocalCache  = (*TieredCacheClient)(nil)
	_ common.TieredCache = (*TieredCacheClient)(nil)
)

// Get retrieves a value from cache following the tier hierarchy: L0 → L1 → L2.
//
// Returns:
//   - data: the cached bytes (nil if not found)
//   - tier: which tier served the data (0=L0, 1=L1, 2=L2, -1=miss)
//   - error: nil on success, ErrCacheMiss if not found in any tier
//
// Critical behavior:
//   - Checks L0 (RAM) first for lowest latency (if enabled)
//   - Falls back to L1 (Disk) if enabled and L0 misses
//   - Falls back to L2 (Remote) via loader if L0/L1 miss
//   - Promotes data to higher tiers on L1/L2 hit
func (c *TieredCacheClient) Get(ctx context.Context, key string) ([]byte, int, error) {
	if key == "" {
		return nil, int(TierMiss), ErrEmptyKey
	}

	prefixed := c.prefixKey(key)

	// L0 (RAM) lookup - only if enabled
	if c.isL0Enabled() {
		if data, found := c.GetFromL0(prefixed); found {
			c.stats.L0Hits.Add(1)
			return data, int(TierL0), nil
		}
		c.stats.L0Misses.Add(1)
	}

	// L1 (Disk) lookup
	if c.isL1Enabled() {
		data, err := c.GetFromL1(ctx, prefixed)
		if err == nil {
			c.stats.L1Hits.Add(1)
			// Promote to L0 if enabled
			c.setL0(prefixed, data, c.config.L0DefaultTTL)
			return data, int(TierL1), nil
		}
		c.stats.L1Misses.Add(1)
	}

	// L2 (Remote) lookup via loader
	if c.l2Loader != nil {
		data, err := c.l2Loader(ctx, key) // Use original key for L2
		if err == nil && data != nil {
			c.stats.L2Hits.Add(1)
			// Promote to L0 (if enabled) and L1 (if enabled)
			c.setL0(prefixed, data, c.config.L0DefaultTTL)
			if c.isL1Enabled() {
				_ = c.writeL1(prefixed, data, c.config.L1DefaultTTL)
			}
			return data, int(TierL2), nil
		}
		c.stats.L2Misses.Add(1)
	}

	// Fallback HTTP call when L2 misses
	// This calls the source service to fetch from MongoDB and repopulate L2
	if c.fallbackClient != nil {
		data, err := c.fetchFromFallback(ctx, key)
		if err == nil && data != nil {
			c.stats.FallbackHits.Add(1)
			// L2 was repopulated by the internal endpoint
			// Populate L0/L1 locally
			c.setL0(prefixed, data, c.config.L0DefaultTTL)
			if c.isL1Enabled() {
				_ = c.writeL1(prefixed, data, c.config.L1DefaultTTL)
			}
			return data, int(TierFallback), nil
		}
		c.stats.FallbackMisses.Add(1)
	}

	return nil, int(TierMiss), ErrCacheMiss
}

// Set stores a value in L0 (RAM) and/or L1 (Disk) based on configuration.
//
// Does NOT write to L2 - that's the source of truth managed separately.
// Use this for caching data that was loaded from L2.
//
// Critical behavior:
//   - Writes to L0 (RAM) if enabled
//   - Writes to L1 (Disk) if enabled
//   - When ttl=0, uses the default TTLs from bootstrap config (L0DefaultTTL / L1DefaultTTL)
//   - When ttl>0, applies the same TTL to both L0 and L1
func (c *TieredCacheClient) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if key == "" {
		return ErrEmptyKey
	}
	if value == nil {
		return ErrNilData
	}

	prefixed := c.prefixKey(key)

	// Resolve TTLs — when ttl=0, use bootstrap defaults per tier
	l0TTL := ttl
	l1TTL := ttl
	if ttl == 0 {
		l0TTL = c.config.L0DefaultTTL
		l1TTL = c.config.L1DefaultTTL
	}

	// Set in L0 (RAM) if enabled
	c.setL0(prefixed, value, l0TTL)

	// Set in L1 (Disk) if enabled
	if c.isL1Enabled() {
		if err := c.writeL1(prefixed, value, l1TTL); err != nil {
			// Log but don't fail - L0 may still be cached
			// logger.Warn(fmt.Sprintf("L1 write failed for key %s: %v", key, err))
		}
	}

	return nil
}

// Delete removes a value from all cache tiers (L0, L1).
//
// Does NOT delete from L2 - use the MinIO client directly for that.
func (c *TieredCacheClient) Delete(ctx context.Context, key string) error {
	if key == "" {
		return ErrEmptyKey
	}

	prefixed := c.prefixKey(key)

	// Delete from L0 if enabled
	if c.isL0Enabled() {
		c.l0Cache.Del(prefixed)
	}

	// Delete from L1 if enabled
	if c.isL1Enabled() {
		c.deleteL1Files(prefixed)
	}

	return nil
}

// Invalidate removes a value from local cache only (L0 + L1).
// Alias for Delete - used for semantic clarity in invalidation scenarios.
func (c *TieredCacheClient) Invalidate(ctx context.Context, key string) error {
	return c.Delete(ctx, key)
}

// Stats returns current cache statistics.
func (c *TieredCacheClient) Stats() common.LocalCacheStats {
	return common.LocalCacheStats{
		L0Hits:         c.stats.L0Hits.Load(),
		L0Misses:       c.stats.L0Misses.Load(),
		L0Size:         c.stats.L0Size.Load(),
		L1Hits:         c.stats.L1Hits.Load(),
		L1Misses:       c.stats.L1Misses.Load(),
		L1Size:         c.stats.L1Size.Load(),
		L2Hits:         c.stats.L2Hits.Load(),
		L2Misses:       c.stats.L2Misses.Load(),
		FallbackHits:   c.stats.FallbackHits.Load(),
		FallbackMisses: c.stats.FallbackMisses.Load(),
		L1LazyExpired:  c.stats.L1LazyExpired.Load(),
	}
}

// fetchFromFallback calls the HTTP fallback endpoint to fetch data from source.
//
// The key format is expected to be "{orgId}/{resourceId}". This function extracts
// only the resourceId (last segment) to build the endpoint URL.
// The internal endpoint fetches from MongoDB and repopulates L2 (MinIO) before returning.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - key: The cache key (e.g., "orgId123/assetUUID456")
//
// Returns:
//   - []byte: The fetched data as JSON bytes
//   - error: If the request fails or resource not found
func (c *TieredCacheClient) fetchFromFallback(ctx context.Context, key string) ([]byte, error) {
	// Extract only the resourceId from key format "{orgId}/{resourceId}"
	// The fallback endpoint expects only the resourceId, not the full cache key
	resourceId := key
	if idx := strings.LastIndex(key, "/"); idx != -1 {
		resourceId = key[idx+1:]
	}
	endpoint := c.fallbackEndpoint + "/" + resourceId

	var response map[string]interface{}
	if err := c.fallbackClient.Get(ctx, endpoint, &response); err != nil {
		logger.Debug("[INFRA:CACHE] Fallback miss for key: " + key)
		return nil, err
	}

	// Extract data from response wrapper if present
	// API returns: { "data": { ... actual payload ... } }
	data, ok := response["data"]
	if !ok {
		// Response is the payload itself
		data = response
	}

	// Marshal back to JSON bytes for caching
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	logger.Debug("[INFRA:CACHE] Fallback hit for key: " + key)
	return jsonData, nil
}

// GetFromL0 retrieves directly from RAM cache.
// Returns the data and whether it was found.
// Returns nil, false if L0 is not enabled.
func (c *TieredCacheClient) GetFromL0(key string) ([]byte, bool) {
	if !c.isL0Enabled() {
		return nil, false
	}
	val, found := c.l0Cache.Get(key)
	if !found {
		return nil, false
	}
	data, ok := val.([]byte)
	if !ok {
		return nil, false
	}
	return data, true
}

// GetFromL1 retrieves directly from disk cache.
func (c *TieredCacheClient) GetFromL1(ctx context.Context, key string) ([]byte, error) {
	if !c.isL1Enabled() {
		return nil, ErrCacheMiss
	}
	return c.readL1(key)
}

// Warmup preloads keys into L0/L1 from L2.
// Useful for pre-populating cache on service startup.
func (c *TieredCacheClient) Warmup(ctx context.Context, keys []string) error {
	if c.l2Loader == nil {
		return ErrNoLoader
	}

	for _, key := range keys {
		// Try to load from L2 and populate L0/L1
		_, _, _ = c.Get(ctx, key)
	}

	return nil
}

// setL0 stores a value in L0 (RAM) cache with TTL.
// Does nothing if L0 is not enabled.
func (c *TieredCacheClient) setL0(key string, value []byte, ttl time.Duration) {
	if !c.isL0Enabled() {
		return
	}
	cost := int64(len(value))
	c.l0Cache.SetWithTTL(key, value, cost, ttl)
	c.l0Cache.Wait() // Ensure the value is stored
}

// GetOrLoad retrieves from cache or loads from L2 if not cached.
// This is a convenience method that combines Get + automatic L2 loading.
func (c *TieredCacheClient) GetOrLoad(ctx context.Context, key string, loader common.LocalCacheLoader) ([]byte, error) {
	// Try cache first
	data, tier, err := c.Get(ctx, key)
	if err == nil {
		return data, nil
	}

	// Cache miss - use provided loader or default
	l := loader
	if l == nil {
		l = c.l2Loader
	}
	if l == nil {
		return nil, ErrNoLoader
	}

	// Load from L2
	data, err = l(ctx, key)
	if err != nil {
		return nil, err
	}

	// Cache the loaded data (TTL=0 → uses bootstrap defaults)
	if data != nil {
		_ = c.Set(ctx, key, data, 0)
	}

	// Update stats based on result
	if tier == int(TierMiss) && data != nil {
		c.stats.L2Hits.Add(1)
	}

	return data, nil
}
