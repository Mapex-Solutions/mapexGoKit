package middlewaresCoverage

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	ctx "github.com/Mapex-Solutions/mapexGoKit/microservices/common/context"
	common "github.com/Mapex-Solutions/mapexGoKit/infrastructure/common/ports"
	middlewaresAuth "github.com/Mapex-Solutions/mapexGoKit/microservices/http/middlewares/auth"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/http/response"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/http/status"
	"github.com/Mapex-Solutions/mapexGoKit/utils/orgfilter"
	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// Global variable to hold the shared cache instance
// This is initialized once during app startup via InitCoverageMiddleware
var globalSharedCache common.SharedCache

// RootPermissions defines permissions that grant unrestricted global access
// ONLY these permissions allow users to query without X-Org-Context header
// All other users (including admin_vendor.*, admin_customer.*) MUST specify org context
var RootPermissions = []string{
	"mapex.*", // System-wide ROOT permission - ONLY true admin
}

// InitCoverageMiddleware inicializa o middleware global com o SharedCache.
//
// Esta função DEVE ser chamada uma vez durante o startup da aplicação,
// após o SharedCache estar disponível no container DIG.
//
// Parameters:
//   - sharedCache: Redis cache compartilhado que contém as coverages dos usuários
//
// Example usage in main.go:
//
//	import coverageMw "github.com/Mapex-Solutions/mapexGoKit/microservices/http/middlewares/coverage"
//
//	c.Invoke(func(sharedCache common.SharedCache) {
//	    coverageMw.InitCoverageMiddleware(sharedCache)
//	    logger.Info("✅ Coverage middleware initialized")
//	})
func InitCoverageMiddleware(sharedCache common.SharedCache) {
	globalSharedCache = sharedCache
}

// InjectRequestContext is a Fiber middleware that creates and injects RequestContext
// into c.Locals("requestContext") for use by service layers.
//
// This middleware is the foundation for the standardized list endpoint pattern,
// providing context-aware organization filtering with hierarchical support.
//
// Flow:
//  1. Extract userId from JWT (via AuthMiddleware)
//  2. Fetch user coverage from cache (includes PathKey for each org)
//  3. Process X-Org-Context header (optional)
//  4. Validate org context against scoped orgs
//  5. Extract org context data (with PathKey) from coverage
//  6. Build and inject RequestContext into c.Locals()
//
// Headers processed:
//   - X-Org-Context: Optional organization ID to filter results
//
// Context injected into c.Locals():
//   - "requestContext": *context.RequestContext - Contains all context data
//
// RequestContext structure:
//   - ScopedOrgIds: []string - All accessible org IDs (from coverage cache)
//   - CoverageOrgs: []CoverageOrg - Detailed org data with PathKey
//   - OrgContext: *string - Org ID from X-Org-Context header (nil if not provided)
//   - OrgContextData: *CoverageOrg - Detailed data for org context (nil if not provided)
//   - UserId: string - Authenticated user ID (from JWT token)
//
// Usage in services with orgfilter:
//
//	reqContext := c.Locals("requestContext").(*context.RequestContext)
//	orgFilter, err := orgfilter.BuildOrgFilter(orgfilter.BuildFilterParams{
//	    ReqContext: reqContext,
//	    Query:      query,
//	})
//
// Middleware chain example:
//
//	group.Get("/",
//	    authMw.AuthMiddleware(),
//	    permissionMw.RequirePermission(perms.AssetList),
//	    coverageMw.InjectRequestContext(),
//	    validation.ValidationMiddleware(queryDto),
//	    handlers.ListAssets(service),
//	)
//
// Responses:
//   - 401 Unauthorized: userId missing in token
//   - 403 Forbidden: Org context not in user's scoped orgs
//   - 500 Internal Server Error: Failed to fetch coverage
func InjectRequestContext() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Extract userId from JWT token (authentication)
		userId, ok := middlewaresAuth.GetUserIdFromToken(c)
		if !ok {
			return response.Custom(c, status.UNAUTHORIZED, []string{"missing or invalid userId in token"})
		}

		// 2. Fetch coverage from cache (authorization - lazy build if miss)
		userAccess, err := getCoverageWithLazyBuild(c.UserContext(), userId)
		if err != nil {
			logger.Error(err, fmt.Sprintf("[MIDDLEWARE:Coverage] Failed to get coverage for userId=%s", userId))
			return response.Custom(c, status.INTERNAL_SERVER_ERROR,
				[]string{"Failed to fetch user access information"})
		}

		// 3. Process X-Org-Context header
		orgContextHeader := c.Get("X-Org-Context")
		var orgContext *string
		if orgContextHeader != "" {
			orgContext = &orgContextHeader
		}

		// 4. Validate that X-Org-Context is provided (unless user is ROOT)
		// Only mapex.* users can query without org context
		// All other users (admin_vendor.*, admin_customer.*, etc.) MUST specify org
		if orgContext == nil {
			isRootUser := hasRootPermissions(c.UserContext(), userId)
			if !isRootUser {
				logger.Info(fmt.Sprintf("[MIDDLEWARE:Coverage] Non-ROOT user without org context: userId=%s", userId))
				return response.Custom(c, status.FORBIDDEN,
					[]string{"X-Org-Context header required. Only ROOT users (mapex.*) can query without organization context."})
			}
			logger.Info(fmt.Sprintf("[MIDDLEWARE:Coverage] ROOT user querying without org context: userId=%s", userId))
		}

		// 5. Validate org context against scoped orgs (if provided)
		if orgContext != nil {
			if !orgfilter.ValidateOrgContext(*orgContext, userAccess.AccessibleOrgIds) {
				logger.Info(fmt.Sprintf("[MIDDLEWARE:Coverage] Invalid org context: userId=%s orgContext=%s not in scoped orgs",
					userId, *orgContext))
				return response.Custom(c, status.FORBIDDEN,
					[]string{"Access denied: organization not in your accessible scope"})
			}
		}

		// 5. Convert coverage organizations to CoverageOrg format
		coverageOrgs := make([]ctx.CoverageOrg, 0, len(userAccess.Organizations))
		for _, org := range userAccess.Organizations {
			coverageOrgs = append(coverageOrgs, ctx.CoverageOrg{
				ID:      org.ID,
				Name:    org.Name,
				Type:    org.Type,
				PathKey: org.PathKey,
			})
		}

		// 6. Extract org context data (if org context provided)
		var orgContextData *ctx.CoverageOrg
		if orgContext != nil {
			orgContextData = orgfilter.FindOrgInCoverage(*orgContext, coverageOrgs)
			if orgContextData == nil {
				logger.Warn(fmt.Sprintf("[MIDDLEWARE:Coverage] Org context data not found in coverage: userId=%s orgContext=%s",
					userId, *orgContext))
				// This shouldn't happen if ValidateOrgContext passed, but handle gracefully
				return response.Custom(c, status.FORBIDDEN,
					[]string{"Organization context data not found"})
			}
		}

		// 7. Determine org filtering scope
		// ROOT users (mapex.*) without org context get unrestricted access (empty filter)
		// All other users use their coverage-based scoped orgs
		scopedOrgIds := userAccess.AccessibleOrgIds
		if orgContext == nil {
			// At this point, we know user is ROOT (validated above)
			logger.Info(fmt.Sprintf("[MIDDLEWARE:Coverage] ROOT user (mapex.*) querying globally: userId=%s", userId))
			scopedOrgIds = []string{} // Empty array = no org filtering in BuildOrgFilter
		}

		// 8. Build RequestContext
		reqContext := &ctx.RequestContext{
			ScopedOrgIds:   scopedOrgIds,
			CoverageOrgs:   coverageOrgs,
			OrgContext:     orgContext,
			OrgContextData: orgContextData,
			UserId:         userId,
		}

		// 9. Inject into Fiber context
		c.Locals("requestContext", reqContext)

		logger.Info(fmt.Sprintf("[MIDDLEWARE:Coverage] RequestContext injected: userId=%s scopedOrgs=%d hasOrgContext=%t",
			userId, len(reqContext.ScopedOrgIds), orgContext != nil))

		return c.Next()
	}
}

// hasRootPermissions checks if a user has ROOT permissions (mapex.*) by checking their
// global auth cache (orgId=""). ROOT users have unrestricted access to all organizations.
//
// This function implements retry logic to handle race conditions during cache build.
// When the version key exists but the versioned data doesn't, it retries for up to 2 seconds.
//
// Race Condition Scenario:
//   1. Cache build updates version key to N
//   2. hasRootPermissions() reads version N
//   3. Cache build hasn't saved data to vN yet (window of ~50-200ms)
//   4. Retry allows time for build to complete
//
// Parameters:
//   - ctx: Context for cache operations
//   - userId: User ID to check
//
// Returns:
//   - bool: true if user has ROOT permissions, false otherwise
func hasRootPermissions(ctx context.Context, userId string) bool {
	if globalSharedCache == nil {
		logger.Warn("[MIDDLEWARE:Coverage] SharedCache not initialized, cannot check ROOT permissions")
		return false
	}

	// Check auth cache for global scope (orgId="global" normalized by auth cache repository)
	// The auth cache repository normalizes empty orgId to "global" for readability
	cacheKey := fmt.Sprintf("auth:org:global:user:%s:ver", userId)

	var versionStr string
	if err := globalSharedCache.Get(ctx, cacheKey, &versionStr); err != nil {
		// No global auth cache found - user doesn't have ROOT permissions
		logger.Info(fmt.Sprintf("[MIDDLEWARE:Coverage] No global auth cache for userId=%s (key=%s): %v", userId, cacheKey, err))
		return false
	}

	logger.Info(fmt.Sprintf("[MIDDLEWARE:Coverage] Found global auth version for userId=%s: version=%s", userId, versionStr))

	// Parse version
	version := 0
	if versionStr != "" {
		fmt.Sscanf(versionStr, "%d", &version)
	}

	if version == 0 {
		logger.Info(fmt.Sprintf("[MIDDLEWARE:Coverage] Invalid version for userId=%s", userId))
		return false
	}

	// Get actual permissions from versioned cache with retry for race condition
	permissionsKey := fmt.Sprintf("auth:org:global:user:%s:v%d", userId, version)
	var permissions []string

	// Retry configuration for handling race condition during cache build
	maxRetries := 10
	retryDelay := 200 * time.Millisecond // Wait 200ms between retries (total max: 2s)

	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := globalSharedCache.Get(ctx, permissionsKey, &permissions); err != nil {
			if attempt < maxRetries-1 {
				// Retry: version key updated but data not saved yet (race condition)
				logger.Info(fmt.Sprintf("[MIDDLEWARE:Coverage] Versioned cache not ready (attempt %d/%d) for userId=%s (key=%s) - retrying in %v",
					attempt+1, maxRetries, userId, permissionsKey, retryDelay))
				time.Sleep(retryDelay)
				continue
			}
			// Final attempt failed
			logger.Info(fmt.Sprintf("[MIDDLEWARE:Coverage] Failed to get permissions after %d attempts for userId=%s (key=%s): %v",
				maxRetries, userId, permissionsKey, err))
			return false
		}

		// Success - permissions retrieved
		if attempt > 0 {
			logger.Info(fmt.Sprintf("[MIDDLEWARE:Coverage] Permissions retrieved on attempt %d for userId=%s", attempt+1, userId))
		}
		break
	}

	logger.Info(fmt.Sprintf("[MIDDLEWARE:Coverage] Checking permissions for userId=%s: %v", userId, permissions))

	// Check if user has any ROOT permission
	for _, userPerm := range permissions {
		for _, rootPerm := range RootPermissions {
			if userPerm == rootPerm {
				logger.Info(fmt.Sprintf("[MIDDLEWARE:Coverage] Found ROOT permission %s for userId=%s", rootPerm, userId))
				return true
			}
		}
	}

	logger.Info(fmt.Sprintf("[MIDDLEWARE:Coverage] No ROOT permissions found for userId=%s", userId))
	return false
}
