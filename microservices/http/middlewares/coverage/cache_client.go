package middlewaresCoverage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// CacheBuildClient handles communication with internal cache building API
type CacheBuildClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// BuildCoverageRequest is the request payload for building coverage cache
type BuildCoverageRequest struct {
	UserID string `json:"userId"`
}

// BuildCoverageResponse is the response from building coverage cache
// Uses UserAccess type which is already defined in types.go
type BuildCoverageResponse = UserAccess

// globalCacheBuildClient - singleton instance initialized once
var globalCacheBuildClient *CacheBuildClient

// InitCacheBuildClient initializes the global cache build client.
// This should be called once during application startup.
//
// Parameters:
//   - baseURL: Base URL of the internal API (e.g., "http://localhost:5000")
//   - apiKey: API key for internal authentication
func InitCacheBuildClient(baseURL, apiKey string) {
	globalCacheBuildClient = &CacheBuildClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // Longer timeout for coverage build (hierarchy expansion)
		},
	}
	logger.Info(fmt.Sprintf("[MIDDLEWARE:Coverage] Cache build client initialized with baseURL=%s", baseURL))
}

// buildCoverageCacheViaAPI calls the internal API to build coverage cache
func (c *CacheBuildClient) buildCoverageCacheViaAPI(ctx context.Context, userId string) (*UserAccess, error) {
	url := fmt.Sprintf("%s/internal/auth/build-coverage", c.baseURL)

	reqBody := BuildCoverageRequest{
		UserID: userId,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	logger.Info(fmt.Sprintf("[MIDDLEWARE:Coverage] Calling internal API to build coverage for userId=%s", userId))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call internal API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("internal API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data struct {
			Organizations *UserAccess `json:"organizations"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Data.Organizations == nil {
		return nil, errors.New("invalid response: organizations is nil")
	}

	return result.Data.Organizations, nil
}

// getCoverageWithLazyBuild retrieves coverage from cache, building it on-demand if not found.
//
// Algorithm:
//  1. Try to get coverage from cache
//  2. If cache miss, call internal API to build cache
//  3. Return coverage
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - userId: User ID
//
// Returns:
//   - *UserAccess: User's coverage (accessible organizations)
//   - error: Error if failed to retrieve or build
func getCoverageWithLazyBuild(ctx context.Context, userId string) (*UserAccess, error) {
	cacheKey := fmt.Sprintf("coverage:user:%s", userId)

	// 1. Try cache first
	var userAccess UserAccess
	err := globalSharedCache.Get(ctx, cacheKey, &userAccess)

	// Cache hit - return immediately
	if err == nil {
		logger.Info(fmt.Sprintf("[MIDDLEWARE:Coverage] Cache hit for userId=%s (%d accessible orgs)", userId, len(userAccess.AccessibleOrgIds)))
		return &userAccess, nil
	}

	// If not a cache miss error, return the error
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("failed to get coverage from cache: %w", err)
	}

	// 2. Cache miss - build on-demand via internal API
	logger.Info(fmt.Sprintf("[MIDDLEWARE:Coverage] Cache miss for userId=%s - building via internal API", userId))

	if globalCacheBuildClient == nil {
		return nil, errors.New("cache build client not initialized")
	}

	// Call internal API to build cache
	result, err := globalCacheBuildClient.buildCoverageCacheViaAPI(ctx, userId)
	if err != nil {
		logger.Error(err, fmt.Sprintf("[MIDDLEWARE:Coverage] Failed to build cache via API for userId=%s", userId))
		return nil, fmt.Errorf("failed to build coverage cache: %w", err)
	}

	logger.Info(fmt.Sprintf("[MIDDLEWARE:Coverage] Built coverage cache for userId=%s (%d accessible orgs)",
		userId, len(result.AccessibleOrgIds)))

	return result, nil
}
