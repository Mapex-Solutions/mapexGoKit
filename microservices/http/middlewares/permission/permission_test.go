package middlewaresPermission

import (
	"testing"
)

/** matchesPermission */

func TestMatchesPermission_RootPermission(t *testing.T) {
	if !matchesPermission("mapex.*", "user.read") {
		t.Error("expected mapex.* to match any permission")
	}
}

func TestMatchesPermission_AdminVendorPermission(t *testing.T) {
	if !matchesPermission("admin_vendor.*", "asset.create") {
		t.Error("expected admin_vendor.* to match any permission")
	}
}

func TestMatchesPermission_AdminCustomerPermission(t *testing.T) {
	if !matchesPermission("admin_customer.*", "group.delete") {
		t.Error("expected admin_customer.* to match any permission")
	}
}

func TestMatchesPermission_AdminPermission(t *testing.T) {
	if !matchesPermission("admin.*", "user.update") {
		t.Error("expected admin.* to match any permission")
	}
}

func TestMatchesPermission_ExactMatch(t *testing.T) {
	if !matchesPermission("user.read", "user.read") {
		t.Error("expected exact match to return true")
	}
}

func TestMatchesPermission_ExactMismatch(t *testing.T) {
	if matchesPermission("user.read", "user.write") {
		t.Error("expected exact mismatch to return false")
	}
}

func TestMatchesPermission_WildcardMatch(t *testing.T) {
	if !matchesPermission("user.*", "user.read") {
		t.Error("expected user.* to match user.read")
	}
}

func TestMatchesPermission_WildcardMatchCreate(t *testing.T) {
	if !matchesPermission("user.*", "user.create") {
		t.Error("expected user.* to match user.create")
	}
}

func TestMatchesPermission_WildcardWrongResource(t *testing.T) {
	if matchesPermission("user.*", "asset.read") {
		t.Error("expected user.* NOT to match asset.read")
	}
}

func TestMatchesPermission_EmptyUserPerm(t *testing.T) {
	if matchesPermission("", "user.read") {
		t.Error("expected empty userPerm to return false")
	}
}

func TestMatchesPermission_EmptyRequired(t *testing.T) {
	if matchesPermission("user.read", "") {
		t.Error("expected empty requiredPerm to return false")
	}
}

func TestMatchesPermission_BothEmpty(t *testing.T) {
	// Empty string exact match
	if !matchesPermission("", "") {
		t.Error("expected empty strings to exact match")
	}
}

func TestMatchesPermission_WildcardDoesNotMatchPartial(t *testing.T) {
	// "user.*" should NOT match "username.read" (different prefix)
	if matchesPermission("user.*", "username.read") {
		t.Error("expected user.* NOT to match username.read")
	}
}

func TestMatchesPermission_NestedWildcard(t *testing.T) {
	if !matchesPermission("asset.*", "asset.template.read") {
		t.Error("expected asset.* to match asset.template.read")
	}
}

/** hasAnyPermission */

func TestHasAnyPermission_Match(t *testing.T) {
	userPerms := []string{"user.read", "user.write"}
	required := []string{"user.read"}
	if !hasAnyPermission(userPerms, required) {
		t.Error("expected hasAnyPermission to return true when user has required permission")
	}
}

func TestHasAnyPermission_NoMatch(t *testing.T) {
	userPerms := []string{"user.read", "user.write"}
	required := []string{"asset.create"}
	if hasAnyPermission(userPerms, required) {
		t.Error("expected hasAnyPermission to return false when user lacks required permission")
	}
}

func TestHasAnyPermission_EmptyUserPerms(t *testing.T) {
	if hasAnyPermission([]string{}, []string{"user.read"}) {
		t.Error("expected false for empty user permissions")
	}
}

func TestHasAnyPermission_EmptyRequired(t *testing.T) {
	if hasAnyPermission([]string{"user.read"}, []string{}) {
		t.Error("expected false for empty required permissions")
	}
}

func TestHasAnyPermission_WildcardMatch(t *testing.T) {
	userPerms := []string{"user.*"}
	required := []string{"user.read", "asset.create"}
	if !hasAnyPermission(userPerms, required) {
		t.Error("expected wildcard user.* to satisfy user.read")
	}
}

func TestHasAnyPermission_RootMatchesAnything(t *testing.T) {
	userPerms := []string{"mapex.*"}
	required := []string{"asset.create", "user.delete"}
	if !hasAnyPermission(userPerms, required) {
		t.Error("expected mapex.* to match any required permission")
	}
}

func TestHasAnyPermission_MultipleRequired_OneMatches(t *testing.T) {
	userPerms := []string{"group.read", "user.read"}
	required := []string{"asset.create", "user.read"}
	if !hasAnyPermission(userPerms, required) {
		t.Error("expected true when at least one required permission matches")
	}
}

func TestHasAnyPermission_AdminVendor(t *testing.T) {
	userPerms := []string{"admin_vendor.*"}
	required := []string{"user.read"}
	if !hasAnyPermission(userPerms, required) {
		t.Error("expected admin_vendor.* to match any permission")
	}
}

func TestHasAnyPermission_AdminCustomer(t *testing.T) {
	userPerms := []string{"admin_customer.*"}
	required := []string{"asset.delete"}
	if !hasAnyPermission(userPerms, required) {
		t.Error("expected admin_customer.* to match any permission")
	}
}

func TestHasAnyPermission_Admin(t *testing.T) {
	userPerms := []string{"admin.*"}
	required := []string{"group.update"}
	if !hasAnyPermission(userPerms, required) {
		t.Error("expected admin.* to match any permission")
	}
}
