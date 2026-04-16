package middlewaresPermission

import (
	"context"
	"fmt"
	"strings"

	common "github.com/Mapex-Solutions/mapexGoKit/infrastructure/common/ports"
	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// globalSharedCache - instância global do Redis cache compartilhado,
// inicializada uma vez durante o startup da aplicação.
var globalSharedCache common.SharedCache

// getUserPermissions retrieves user permissions using versioning strategy and builds cache on-demand.
//
// Cache Strategy (Versioning):
//   - Version Pointer: auth:org:{orgId}:user:{userId}:ver → integer (1-100)
//   - Versioned Cache: auth:org:{orgId}:user:{userId}:v{version} → permissions array (TTL: 30 days)
//
// Algorithm:
//  1. Try to get version pointer
//  2. If exists, get versioned permissions
//  3. If cache miss, call internal API to build cache on-demand
//  4. Return permissions or empty list
//
// Returns:
//   - []string: List of permissions (empty if build fails or user has no permissions)
//   - error: Error only for critical failures (connection issues, etc)
func getUserPermissions(ctx context.Context, userId, orgId string) ([]string, error) {
	// Use versioning strategy with on-demand building
	permissions, err := getUserPermissionsWithVersioning(ctx, userId, orgId)

	// If build failed, log error but return empty permissions (fail-open for availability)
	if err != nil {
		logger.Error(err, fmt.Sprintf("[PERMISSION_MW] Failed to get/build permissions for user=%s org=%s", userId, orgId))
		return []string{}, nil
	}

	// If no permissions, return empty list
	if permissions == nil {
		return []string{}, nil
	}

	return permissions, nil
}

// hasAnyPermission verifica se o usuário tem QUALQUER UMA das permissões requeridas.
//
// Parameters:
//   - userPerms: Lista de permissões que o usuário possui
//   - requiredPerms: Lista de permissões requeridas (usuário precisa de pelo menos uma)
//
// Returns:
//   - bool: true se usuário tem pelo menos uma das permissões requeridas
func hasAnyPermission(userPerms []string, requiredPerms []string) bool {
	for _, userPerm := range userPerms {
		for _, required := range requiredPerms {
			if matchesPermission(userPerm, required) {
				return true
			}
		}
	}
	return false
}

// matchesPermission verifica se uma permissão do usuário atende à permissão requerida.
//
// Regras de matching (em ordem de precedência):
//  1. "mapex.*" → ROOT permission (matches TUDO)
//  2. "admin_vendor.*" → Vendor admin permission (matches TUDO no escopo do vendor)
//  3. "admin_customer.*" → Customer admin permission (matches TUDO no escopo do customer)
//  4. "admin.*" → Admin permission (matches TUDO)
//  5. Exact match → "user.read" matches "user.read"
//  6. Wildcard match → "user.*" matches "user.read", "user.create", etc.
//
// Parameters:
//   - userPerm: Permissão que o usuário possui
//   - requiredPerm: Permissão requerida pela rota
//
// Returns:
//   - bool: true se a permissão do usuário atende ao requerido
func matchesPermission(userPerm, requiredPerm string) bool {
	// Root permission (mapex.*) - acessa TUDO
	if userPerm == RootPermission {
		return true
	}

	// Vendor admin permission (admin_vendor.*) - acessa tudo no escopo do vendor
	if userPerm == AdminVendorPermission {
		return true
	}

	// Customer admin permission (admin_customer.*) - acessa tudo no escopo do customer
	if userPerm == AdminCustomerPermission {
		return true
	}

	// Admin permission (admin.*) - acessa TUDO
	if userPerm == AdminPermission {
		return true
	}

	// Exact match
	if userPerm == requiredPerm {
		return true
	}

	// Wildcard match (e.g., "user.*" matches "user.read", "user.create")
	if strings.HasSuffix(userPerm, ".*") {
		resource := strings.TrimSuffix(userPerm, ".*")
		if strings.HasPrefix(requiredPerm, resource+".") {
			return true
		}
	}

	return false
}
