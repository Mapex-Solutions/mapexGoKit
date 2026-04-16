package middlewaresPermission

import (
	"fmt"

	"github.com/gofiber/fiber/v2"

	common "github.com/Mapex-Solutions/mapexGoKit/infrastructure/common/ports"
	middlewaresAuth "github.com/Mapex-Solutions/mapexGoKit/microservices/http/middlewares/auth"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/http/response"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/http/status"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// InitPermissionMiddleware inicializa o middleware global com o SharedCache e cache build client.
//
// Esta função DEVE ser chamada uma vez durante o startup da aplicação,
// após o SharedCache estar disponível no container DIG.
//
// Parameters:
//   - sharedCache: Redis cache compartilhado (DB 5) que contém as permissões dos usuários
//   - baseURL: Base URL for internal API calls (e.g., "http://localhost:5000")
//   - apiKey: API key for internal authentication
//
// Example usage in main.go:
//
//	import permissionMw "github.com/Mapex-Solutions/mapexGoKit/microservices/http/middlewares/permission"
//
//	c.Invoke(func(sharedCache common.SharedCache) {
//	    baseURL := "http://localhost:5000"
//	    apiKey, _ := config.GetStringValue("internal_api_key")
//	    permissionMw.InitPermissionMiddleware(sharedCache, baseURL, apiKey)
//	    logger.Info("✅ Permission middleware initialized")
//	})
func InitPermissionMiddleware(sharedCache common.SharedCache, baseURL, apiKey string) {
	globalSharedCache = sharedCache
	InitCacheBuildClient(baseURL, apiKey)
}

// RequirePermission retorna um Fiber middleware que verifica se o usuário autenticado
// tem a permissão especificada.
//
// Este é um wrapper de conveniência para RequirePermissions com um único argumento.
//
// Permission checking logic:
//  1. mapex.* → ROOT permission (acessa tudo sem org context)
//  2. admin.* → Admin permission (acessa tudo)
//  3. Specific permission → Exact match ou wildcard match
//
// Usage example:
//
//	import (
//	    permissionMw "github.com/Mapex-Solutions/mapexGoKit/microservices/http/middlewares/permission"
//	    perms "github.com/Mapex-Solutions/myAIOffice/permissions/mapexos"
//	)
//
//	group.Get("/",
//	    permissionMw.RequirePermission(perms.OrganizationList),
//	    validation.ValidationMiddleware(queryDto),
//	    handlers.GetOrganizations(service),
//	)
func RequirePermission(permission string) fiber.Handler {
	return RequirePermissions(permission)
}

// RequirePermissions retorna um Fiber middleware que verifica se o usuário autenticado
// tem QUALQUER UMA das permissões especificadas.
//
// Verificações automáticas (em ordem de precedência):
//  1. mapex.* → Permissão ROOT do MapexOS (acessa tudo sem X-Organization-Id)
//  2. admin.* → Permissão de admin (acessa tudo dentro da organização)
//  3. Permissões específicas passadas como parâmetro
//  4. Wildcard permissions (e.g., user.* matches user.read)
//
// Organization Context Requirements:
//   - Usuários com ROOT permission (mapex.*): Podem acessar SEM X-Organization-Id header
//   - Todos os outros usuários: DEVEM fornecer X-Organization-Id header (retorna 403 se ausente)
//
// Requisitos:
//   - Token JWT válido (AuthMiddleware deve vir ANTES deste middleware)
//   - Header X-Organization-Id (exceto para usuários com mapex.*)
//
// Respostas:
//   - 401 Unauthorized: userId ausente ou inválido no token
//   - 403 Forbidden: Contexto organizacional ausente OU usuário sem permissão
//   - 500 Internal Server Error: Falha ao verificar permissões no cache
//
// Usage example (múltiplas permissões):
//
//	group.Put("/:id",
//	    permissionMw.RequirePermissions("user.update", "user.manage"),
//	    validation.ValidationMiddleware(updateDto),
//	    handlers.UpdateUser(service),
//	)
func RequirePermissions(requiredPermissions ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract userId from JWT token (always required)
		userId, ok := middlewaresAuth.GetUserIdFromToken(c)
		if !ok {
			return response.Custom(c, status.UNAUTHORIZED, []string{"missing or invalid userId in token"})
		}

		// Try to extract organizationId from X-Org-Context header
		organizationId := c.Get("X-Org-Context")

		hasOrgId := organizationId != ""

		// If no org context provided, check if user has admin-level permissions
		if !hasOrgId {
			// Check if user has admin-level permissions (mapex.*, admin_vendor.*, admin_customer.*)
			permissions, err := getUserPermissions(c.UserContext(), userId, "")
			if err != nil {
				return response.Custom(
					c,
					status.INTERNAL_SERVER_ERROR,
					[]string{
						"failed to check admin permissions",
						err.Error(),
					},
				)
			}

			hasAdminAccess := hasAnyPermission(permissions, []string{RootPermission, AdminVendorPermission, AdminCustomerPermission})
			if !hasAdminAccess {
				return response.Custom(
					c,
					status.FORBIDDEN,
					[]string{"Organization context required (X-Org-Context header missing)"},
				)
			}

			// User has admin-level permission, allow access without org context
			organizationId = ""
		}

		// Get user permissions from cache
		permissions, err := getUserPermissions(c.UserContext(), userId, organizationId)
		if err != nil {
			return response.Custom(
				c,
				status.INTERNAL_SERVER_ERROR,
				[]string{
					"failed to check permissions",
					err.Error(),
				},
			)
		}

		// Check if user has any of the required permissions
		if !hasAnyPermission(permissions, requiredPermissions) {

			logger.Error(nil, fmt.Sprintf("User %s lacks required permissions: %v (has: %v)", userId, requiredPermissions, permissions))

			return response.Custom(
				c,
				status.FORBIDDEN,
				[]string{"insufficient permissions"},
			)
		}

		// User has permission, continue to handler
		return c.Next()
	}
}
