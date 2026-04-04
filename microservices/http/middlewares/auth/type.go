package middlewaresAuth

type AuthConfig struct {
	Strategy    string // "jwt" or "oauth2"
	Secret      string // only used for "jwt"
	JWKSURL     string // only used for "oauth2"
	Algorithm   string // e.g. "HS256" or "RS256"
	RolesPath   string // e.g. "realm_access.roles"
	RolesSource string // "token", "db", or "api"
	RolesAPIURL string // used if RolesSource == "api"
}
