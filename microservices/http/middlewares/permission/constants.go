package middlewaresPermission

// Permission levels - ordem de precedência
const (
	// RootPermission - Permissão ROOT do MapexOS (acessa tudo sem contexto organizacional)
	RootPermission = "mapex.*"

	// AdminVendorPermission - Permissão de admin vendor (acessa tudo abaixo do vendor dele)
	AdminVendorPermission = "admin_vendor.*"

	// AdminCustomerPermission - Permissão de admin customer (acessa tudo abaixo do customer dele)
	AdminCustomerPermission = "admin_customer.*"

	// AdminPermission - Permissão de administrador (acessa tudo dentro da organização)
	AdminPermission = "admin.*"
)

// Cache key format
const (
	// CacheKeyFormat - Formato da chave no Redis para permissões do usuário
	// Formato: "auth:org:{orgId}:user:{userId}"
	CacheKeyFormat = "auth:org:%s:user:%s"
)
