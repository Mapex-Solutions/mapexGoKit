package middlewaresOrgHierarchy

import (
	"fmt"

	"github.com/gofiber/fiber/v2"

	ctx "github.com/Mapex-Solutions/mapexGoKit/microservices/common/context"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/http/response"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/http/status"
	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
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

// DTOExtractor extracts the organization-creation DTO from the Fiber context
// and adapts it to the OrganizationCreateContract interface. Consumer projects
// provide this callback so the middleware stays DTO-agnostic.
//
// Typical implementation in a consumer project:
//
//	func(c *fiber.Ctx) (OrganizationCreateContract, error) {
//	    return requestValidation.GetDTO[*orgDtos.OrganizationCreate](c, "bodyDTO")
//	}
type DTOExtractor func(c *fiber.Ctx) (OrganizationCreateContract, error)

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
// The middleware accepts a DTOExtractor so it stays decoupled from any specific
// consumer project's DTO type. Consumer projects provide an extractor that pulls
// their concrete DTO out of c.Locals and returns it as OrganizationCreateContract.
//
// Middleware chain example:
//
//	group.Post("/",
//	    permissionMw.RequirePermission(perms.OrganizationCreate),
//	    validation.ValidationMiddleware(createDto),
//	    coverageMw.InjectRequestContext(),
//	    orgHierarchyMw.ValidateOrgHierarchy(func(c *fiber.Ctx) (orgHierarchyMw.OrganizationCreateContract, error) {
//	        return requestValidation.GetDTO[*orgDtos.OrganizationCreate](c, "bodyDTO")
//	    }),
//	    handlers.CreateOrganization(service),
//	)
//
// Responses:
//   - 400 Bad Request: Invalid hierarchy (wrong child type, zone is leaf, vendor requires no parent)
//   - 403 Forbidden: User has no access to parent organization
func ValidateOrgHierarchy(extractor DTOExtractor) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. Extract body DTO via the consumer-provided extractor
		dto, err := extractor(c)
		if err != nil {
			logger.Error(err, "[MIDDLEWARE:OrgHierarchy] Failed to extract body DTO")
			return response.BadRequest(c, []string{"Invalid request body"})
		}

		// 2. If no parentOrgId, only vendor type is allowed (root organization)
		if dto.GetParentOrgID() == nil {
			if dto.GetType() != "vendor" {
				logger.Info(fmt.Sprintf("[MIDDLEWARE:OrgHierarchy] Non-vendor type '%s' requires parentOrgId", dto.GetType()))
				return response.BadRequest(c, []string{
					fmt.Sprintf("Organization type '%s' requires a parent organization. Only 'vendor' can be created without a parent.", dto.GetType()),
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
		parentOrgID := dto.GetParentOrgID()
		var parentOrg *ctx.CoverageOrg
		for i, org := range reqCtx.CoverageOrgs {
			if org.ID == *parentOrgID {
				parentOrg = &reqCtx.CoverageOrgs[i]
				break
			}
		}

		if parentOrg == nil {
			logger.Info(fmt.Sprintf("[MIDDLEWARE:OrgHierarchy] User %s has no access to parent org %s",
				reqCtx.UserId, *parentOrgID))
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

		if dto.GetType() != expectedType {
			logger.Info(fmt.Sprintf("[MIDDLEWARE:OrgHierarchy] Invalid child type: parent=%s (type=%s), expected child=%s, got=%s",
				parentOrg.ID, parentOrg.Type, expectedType, dto.GetType()))
			return response.BadRequest(c, []string{
				fmt.Sprintf("Invalid organization type '%s' for parent type '%s'. Expected '%s'.",
					dto.GetType(), parentOrg.Type, expectedType),
			})
		}

		logger.Info(fmt.Sprintf("[MIDDLEWARE:OrgHierarchy] Hierarchy validated: parent=%s (type=%s) → child type=%s",
			parentOrg.ID, parentOrg.Type, dto.GetType()))

		return c.Next()
	}
}
