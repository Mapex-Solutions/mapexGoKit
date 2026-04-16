package middlewaresPermission

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
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

// BuildAuthorizationRequest is the request payload for building authorization cache
type BuildAuthorizationRequest struct {
	UserID string `json:"userId"`
	OrgID  string `json:"orgId"`
}

// BuildAuthorizationResponse is the response from building authorization cache
type BuildAuthorizationResponse struct {
	Permissions []string `json:"permissions"`
	Version     int      `json:"version"`
}

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
			Timeout: 10 * time.Second,
		},
	}
}

// buildAuthorizationCacheViaAPI calls the internal API to build authorization cache
func (c *CacheBuildClient) buildAuthorizationCacheViaAPI(ctx context.Context, userId, orgId string) (*BuildAuthorizationResponse, error) {
	url := fmt.Sprintf("%s/internal/auth/build-authorization", c.baseURL)

	reqBody := BuildAuthorizationRequest{
		UserID: userId,
		OrgID:  orgId,
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
		Data BuildAuthorizationResponse `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result.Data, nil
}

// getUserPermissionsWithVersioning retrieves user permissions using the versioning strategy.
//
// Algorithm:
//  1. Get version pointer: auth:org:{orgId}:user:{userId}:ver
//  2. If version exists, get cached permissions: auth:org:{orgId}:user:{userId}:v{version}
//  3. If cache miss, call internal API to build cache
//  4. Return permissions
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - userId: User ID
//   - orgId: Organization ID
//
// Returns:
//   - []string: List of permissions
//   - error: Error if failed to retrieve or build
func getUserPermissionsWithVersioning(ctx context.Context, userId, orgId string) ([]string, error) {
	// Normalize orgId: empty string → "global" to match auth cache repository
	normalizedOrgId := orgId
	if orgId == "" {
		normalizedOrgId = "global"
	}

	// 1. Get version pointer
	verKey := fmt.Sprintf("auth:org:%s:user:%s:ver", normalizedOrgId, userId)

	var versionStr string
	err := globalSharedCache.Get(ctx, verKey, &versionStr)

	// If version not found, cache doesn't exist - build it
	if err != nil && errors.Is(err, redis.Nil) {
		logger.Info(fmt.Sprintf("[MIDDLEWARE:Permission] Cache miss for user=%s org=%s - building via internal API", userId, orgId))

		if globalCacheBuildClient == nil {
			return nil, errors.New("cache build client not initialized")
		}

		// Call internal API to build cache
		result, err := globalCacheBuildClient.buildAuthorizationCacheViaAPI(ctx, userId, orgId)
		if err != nil {
			logger.Error(err, fmt.Sprintf("[MIDDLEWARE:Permission] Failed to build cache via API for user=%s org=%s", userId, orgId))
			return nil, fmt.Errorf("failed to build cache: %w", err)
		}

		logger.Info(fmt.Sprintf("[MIDDLEWARE:Permission] Built cache for user=%s org=%s version=%d (%d permissions)",
			userId, orgId, result.Version, len(result.Permissions)))

		return result.Permissions, nil
	}

	// If real error getting version, return it
	if err != nil {
		return nil, fmt.Errorf("failed to get version: %w", err)
	}

	// 2. Parse version
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return nil, fmt.Errorf("invalid version format: %w", err)
	}

	// 3. Get cached permissions using version with retry for race condition
	cacheKey := fmt.Sprintf("auth:org:%s:user:%s:v%d", normalizedOrgId, userId, version)

	var permissions []string

	// Retry configuration for handling race condition during cache build
	// When version key exists but versioned data doesn't, retry for up to 2 seconds
	maxRetries := 10
	retryDelay := 200 * time.Millisecond // Wait 200ms between retries (total max: 2s)

	for attempt := 0; attempt < maxRetries; attempt++ {
		err = globalSharedCache.Get(ctx, cacheKey, &permissions)

		if err == nil {
			// Success - permissions retrieved
			if attempt > 0 {
				logger.Info(fmt.Sprintf("[MIDDLEWARE:Permission] Permissions retrieved on attempt %d for user=%s org=%s", attempt+1, userId, orgId))
			}
			return permissions, nil
		}

		// If not a cache miss, return the error
		if !errors.Is(err, redis.Nil) {
			return nil, fmt.Errorf("failed to get permissions: %w", err)
		}

		// Cache miss - retry or rebuild
		if attempt < maxRetries-1 {
			// Retry: version key updated but data not saved yet (race condition)
			logger.Info(fmt.Sprintf("[MIDDLEWARE:Permission] Versioned cache not ready (attempt %d/%d) for user=%s org=%s (key=%s) - retrying in %v",
				attempt+1, maxRetries, userId, orgId, cacheKey, retryDelay))
			time.Sleep(retryDelay)
			continue
		}
	}

	// If versioned cache not found after all retries, rebuild
	logger.Info(fmt.Sprintf("[MIDDLEWARE:Permission] Versioned cache miss after %d retries for user=%s org=%s version=%d - rebuilding", maxRetries, userId, orgId, version))

	if globalCacheBuildClient == nil {
		return nil, errors.New("cache build client not initialized")
	}

	result, err := globalCacheBuildClient.buildAuthorizationCacheViaAPI(ctx, userId, orgId)
	if err != nil {
		return nil, fmt.Errorf("failed to rebuild cache: %w", err)
	}

	logger.Info(fmt.Sprintf("[MIDDLEWARE:Permission] Rebuilt cache for user=%s org=%s version=%d (%d permissions)",
		userId, orgId, result.Version, len(result.Permissions)))

	return result.Permissions, nil
}
