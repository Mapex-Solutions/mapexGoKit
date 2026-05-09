# jwt — HS256 access and refresh token helpers

Tiny wrapper around [`golang-jwt/jwt/v5`](https://github.com/golang-jwt/jwt) for the access/refresh-token pattern Mapex services use. Single signing algorithm (HS256), shared secret string, no JWKS support — for OAuth2/JWKS see `microservices/http/auth.ParseJWTTokenWithJWKS`.

> Package name: `jwt` (directory: `jwt/`).

## Surface

```go
type CustomClaims struct {
    ID        string `json:"sessionId"`
    UserID    string `json:"userId"`
    Email     string `json:"email"`
    FisrtName string `json:"firtName"`   // ⚠ field name typo "Fisrt"; JSON tag "firtName"
    LastName  string `json:"lastName"`
    jwt.RegisteredClaims
}

func SignJWT(secret string, userID, sessionId, email string, ttl time.Duration) (string, error)
func ParseJWT(secret string, tokenStr string) (*CustomClaims, error)

func SignRefreshToken(secret string, userID string, sessionId string, ttl time.Duration) (string, error)
func ParseRefreshToken(secret string, tokenStr string) (*jwt.RegisteredClaims, error)
```

### `SignJWT` — access token

Signed with HS256. Embeds `CustomClaims`:

| Field | Source |
|---|---|
| `ID` (claim `sessionId`) | `sessionId` arg |
| `UserID` | `userID` arg |
| `Email` | `email` arg |
| `RegisteredClaims.ExpiresAt` | `time.Now().Add(ttl)` |
| `RegisteredClaims.IssuedAt` | `time.Now()` |

Note: the constructor does **not** populate `FisrtName` / `LastName` — those fields exist on the struct but are reserved for callers that build claims by hand. They will round-trip through `ParseJWT`, so a token created elsewhere can carry them.

### `ParseJWT`

Parses with `ParseWithClaims(&CustomClaims{}, …)` using the supplied secret. Returns `errors.New("invalid or expired token")` on any verification failure (does **not** wrap the underlying jwt error). Returns `errors.New("invalid claims type")` if the parsed claims are not `*CustomClaims`.

### `SignRefreshToken` — refresh token

Signed with HS256, **no custom claims** — only `RegisteredClaims`:

| Field | Source |
|---|---|
| `Subject` | `userID` arg |
| `ID` | `sessionId` arg |
| `ExpiresAt` | `time.Now().Add(ttl)` |
| `IssuedAt` | `time.Now()` |

### `ParseRefreshToken`

Same shape as `ParseJWT` but with `*jwt.RegisteredClaims`.

## Usage

```go
secret := os.Getenv("JWT_SECRET")
sid := uuid.NewString()

access,  _ := jwt.SignJWT(secret, userID, sid, email, 15*time.Minute)
refresh, _ := jwt.SignRefreshToken(secret, userID, sid, 30*24*time.Hour)

claims, err := jwt.ParseJWT(secret, access)
if err != nil { return err } // "invalid or expired token"
_ = claims.UserID
```

## Notes

- Both parse helpers collapse every error into `"invalid or expired token"` / `"invalid or expired refresh token"`. If you need to distinguish "wrong signature" from "expired", parse with `golang-jwt` directly.
- Field name typo: the struct field is `FisrtName` (note the misspelling) and the JSON tag is `firtName`. Renaming would change wire format — keep the typo when interoperating with existing tokens.
- The struct embeds `jwt.RegisteredClaims`, so `ParseJWT` honours `exp`/`iat`/`nbf`/etc. validation as the underlying library defines.
