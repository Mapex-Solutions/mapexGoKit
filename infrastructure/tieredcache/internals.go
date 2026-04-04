package tieredcache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// prefixKey adds the configured prefix to a cache key.
func (c *TieredCacheClient) prefixKey(key string) string {
	if c.keyPrefix == "" {
		return key
	}
	return c.keyPrefix + ":" + key
}

// hashKey creates a filesystem-safe hash of the cache key.
// Used for L1 disk cache filenames.
func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:16]) // Use first 16 bytes (32 hex chars)
}

// l1FilePath returns the file path for a cached item in L1.
// Uses 2-level subdirectory structure for better distribution with 200M+ assets.
// Example: hash "a1b2c3d4..." → /cache/a1/b2/a1b2c3d4.cache
// This creates 65,536 directories (256 × 256), keeping ~3K files per dir.
func (c *TieredCacheClient) l1FilePath(key string) string {
	hash := hashKey(key)
	// 2-level subdirectory: first 2 chars + next 2 chars
	// e.g., "a1b2c3d4..." → "a1/b2/a1b2c3d4.cache"
	level1 := hash[:2]
	level2 := hash[2:4]
	return filepath.Join(c.l1Dir, level1, level2, hash+L1FileExtension)
}

// l1MetaPath returns the metadata file path for a cached item in L1.
func (c *TieredCacheClient) l1MetaPath(key string) string {
	hash := hashKey(key)
	level1 := hash[:2]
	level2 := hash[2:4]
	return filepath.Join(c.l1Dir, level1, level2, hash+L1MetaExtension)
}

// writeL1 writes data to L1 disk cache.
func (c *TieredCacheClient) writeL1(key string, data []byte, ttl time.Duration) error {
	filePath := c.l1FilePath(key)
	metaPath := c.l1MetaPath(key)

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write data file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return err
	}

	// Write metadata
	entry := CacheEntry{
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now(),
		Size:      int64(len(data)),
	}
	metaData, err := json.Marshal(entry)
	if err != nil {
		os.Remove(filePath)
		return err
	}
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		os.Remove(filePath)
		return err
	}

	// Update L1 size stats
	c.stats.L1Size.Add(int64(len(data)))

	return nil
}

// readL1 reads data from L1 disk cache.
func (c *TieredCacheClient) readL1(key string) ([]byte, error) {
	filePath := c.l1FilePath(key)
	metaPath := c.l1MetaPath(key)

	// Check metadata for expiration
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrCacheMiss
		}
		return nil, err
	}

	var entry CacheEntry
	if err := json.Unmarshal(metaData, &entry); err != nil {
		// Corrupted metadata, delete files
		c.deleteL1Files(key)
		return nil, ErrCacheMiss
	}

	// Check expiration (lazy cleanup)
	if time.Now().After(entry.ExpiresAt) {
		c.deleteL1Files(key)
		c.stats.L1LazyExpired.Add(1)
		return nil, ErrEntryExpired
	}

	// Read data file
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Metadata exists but data doesn't - corrupted state
			c.deleteL1Files(key)
			return nil, ErrCacheMiss
		}
		return nil, err
	}

	return data, nil
}

// deleteL1Files removes both data and metadata files from L1.
func (c *TieredCacheClient) deleteL1Files(key string) {
	filePath := c.l1FilePath(key)
	metaPath := c.l1MetaPath(key)

	// Get file size before deletion for stats
	if info, err := os.Stat(filePath); err == nil {
		c.stats.L1Size.Add(-info.Size())
	}

	os.Remove(filePath)
	os.Remove(metaPath)
}

// isL0Enabled returns true if L0 RAM cache is enabled.
func (c *TieredCacheClient) isL0Enabled() bool {
	return c.config.EnableL0 && c.l0Cache != nil
}

// isL1Enabled returns true if L1 disk cache is enabled.
func (c *TieredCacheClient) isL1Enabled() bool {
	return c.config.EnableL1 && c.l1Dir != ""
}

// Note: Background cleanup removed in favor of external script (cache-cleanup.sh)
// The script uses Linux atime (relatime) for LRU-based cleanup, which is more
// efficient and doesn't require TTL management in Go.
//
// Cleanup strategy:
//   1. Lazy: expired files are removed on read (see readL1)
//   2. External: cron script removes files not accessed in 72h
