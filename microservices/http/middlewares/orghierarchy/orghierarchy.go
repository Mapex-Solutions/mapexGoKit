package middlewaresOrgHierarchy

import (
	"fmt"

	"github.com/gofiber/fiber/v2"

	ctx "github.com/Mapex-Solutions/myAIOffice/contracts/common/context"
	orgDtos "github.com/Mapex-Solutions/myAIOffice/contracts/services/mapexIam/organizations"
	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
	requestValidation "github.com/Mapex-Solutions/mapexGoKit/microservices/http/requestValidation"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/http/response"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/http/status"
)

// ValidChildTypes maps each organization type to the valid child type it can contain.
// This enforces the strict hierarchy: vendor → customer → site → building → floor → zone.
// Zone is a leaf node and is NOT present as a key (cannot have children).
var ValidChildTypes = map[string]string{
	"vendor":   "customer",
	"customer": "site",
	"site":     "building",
	"building": "floor",
	"floor":    "zone",
}

// ValidateOrgHierarchy is a Fiber middleware that validates organization creation
// requests against the user's coverage and the hierarchy rules.
//
// This middleware MUST be placed AFTER:
//   - ValidationMiddleware (to have bodyDTO in Locals)
//   - InjectRequestContext (to have requestContext in Locals)
//
// Validations performed:
//  1. If parentOrgId is provided, checks user has access to it (via coverage)
//  2. Validates that parent is not a "zone" (leaf node, no children)
//  3. Validates that the requested type matches the expected child type for the parent
//  4. If no parentOrgId, only ROOT orgs (vendor) can be created
//
// Middleware chain example:
//
//	group.Post("/",
//	    permissionMw.RequirePermission(perms.OrganizationCreate),
//	    validation.ValidationMiddleware(createDto),
//	    coverageMw.InjectRequestContext(),
//	    orgHierarchyMw.ValidateOrgHierarchy(),
//	    handlers.CreateOrganization(service),
//	)
//
// Responses:
//   - 400 Bad Request: Invalid hierarchy (wrong child type, zone is leaf, vendor requires no parent)
//   - 403 Forbidden: User has no access to parent organization
func ValidateOrgHierarchy() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Extract body DTO (already parsed by ValidationMiddleware)
		dto, err := requestValidation.GetDTO[*orgDtos.OrganizationCreate](c, "bodyDTO")
		if err != nil {
			logger.Error(err, "[MIDDLEWARE:OrgHierarchy] Failed to extract body DTO")
			return response.BadRequest(c, []string{"Invalid request body"})
		}

		// 2. If no parentOrgId, only vendor type is allowed (root organization)
		if dto.ParentOrgID == nil {
			if dto.Type != "vendor" {
				logger.Info(fmt.Sprintf("[MIDDLEWARE:OrgHierarchy] Non-vendor type '%s' requires parentOrgId", dto.Type))
				return response.BadRequest(c, []string{
					fmt.Sprintf("Organization type '%s' requires a parent organization. Only 'vendor' can be created without a parent.", dto.Type),
				})
			}
			// Vendor creation without parent - allowed, continue
			return c.Next()
		}

		// 3. Extract RequestContext (injected by coverage middleware)
		reqCtx, ok := c.Locals("requestContext").(*ctx.RequestContext)
		if !ok || reqCtx == nil {
			logger.Error(nil, "[MIDDLEWARE:OrgHierarchy] RequestContext not found in Locals")
			return response.Custom(c, status.INTERNAL_SERVER_ERROR,
				[]string{"Request context not available"})
		}

		// 4. Find parent organization in user's coverage
		var parentOrg *ctx.CoverageOrg
		for i, org := range reqCtx.CoverageOrgs {
			if org.ID == *dto.ParentOrgID {
				parentOrg = &reqCtx.CoverageOrgs[i]
				break
			}
		}

		if parentOrg == nil {
			logger.Info(fmt.Sprintf("[MIDDLEWARE:OrgHierarchy] User %s has no access to parent org %s",
				reqCtx.UserId, *dto.ParentOrgID))
			return response.Custom(c, status.FORBIDDEN,
				[]string{"No access to parent organization"})
		}

		// 5. Validate parent is not a leaf (zone cannot have children)
		if parentOrg.Type == "zone" {
			logger.Info(fmt.Sprintf("[MIDDLEWARE:OrgHierarchy] Cannot create children under zone org %s",
				parentOrg.ID))
			return response.BadRequest(c, []string{
				"Cannot create child organizations under a zone. Zone is the lowest level in the hierarchy.",
			})
		}

		// 6. Validate the requested type matches the expected child type
		expectedType, exists := ValidChildTypes[parentOrg.Type]
		if !exists {
			logger.Error(nil, fmt.Sprintf("[MIDDLEWARE:OrgHierarchy] Unknown parent type '%s' for org %s",
				parentOrg.Type, parentOrg.ID))
			return response.BadRequest(c, []string{
				fmt.Sprintf("Unknown parent organization type '%s'", parentOrg.Type),
			})
		}

		if dto.Type != expectedType {
			logger.Info(fmt.Sprintf("[MIDDLEWARE:OrgHierarchy] Invalid child type: parent=%s (type=%s), expected child=%s, got=%s",
				parentOrg.ID, parentOrg.Type, expectedType, dto.Type))
			return response.BadRequest(c, []string{
				fmt.Sprintf("Invalid organization type '%s' for parent type '%s'. Expected '%s'.",
					dto.Type, parentOrg.Type, expectedType),
			})
		}

		logger.Info(fmt.Sprintf("[MIDDLEWARE:OrgHierarchy] Hierarchy validated: parent=%s (type=%s) → child type=%s",
			parentOrg.ID, parentOrg.Type, dto.Type))

		return c.Next()
	}
}
