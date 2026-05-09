# jwt — Helpers de access e refresh token HS256

Wrapper enxuto sobre [`golang-jwt/jwt/v5`](https://github.com/golang-jwt/jwt) para o padrão access/refresh token usado pelos serviços Mapex. Único algoritmo de assinatura (HS256), secret string compartilhado, sem suporte a JWKS — para OAuth2/JWKS veja `microservices/http/auth.ParseJWTTokenWithJWKS`.

> Nome do pacote: `jwt` (diretório: `jwt/`).

## Superfície

```go
type CustomClaims struct {
    ID        string `json:"sessionId"`
    UserID    string `json:"userId"`
    Email     string `json:"email"`
    FisrtName string `json:"firtName"`   // ⚠ typo no nome do field "Fisrt"; tag JSON "firtName"
    LastName  string `json:"lastName"`
    jwt.RegisteredClaims
}

func SignJWT(secret string, userID, sessionId, email string, ttl time.Duration) (string, error)
func ParseJWT(secret string, tokenStr string) (*CustomClaims, error)

func SignRefreshToken(secret string, userID string, sessionId string, ttl time.Duration) (string, error)
func ParseRefreshToken(secret string, tokenStr string) (*jwt.RegisteredClaims, error)
```

### `SignJWT` — access token

Assinado com HS256. Embute `CustomClaims`:

| Field | Fonte |
|---|---|
| `ID` (claim `sessionId`) | arg `sessionId` |
| `UserID` | arg `userID` |
| `Email` | arg `email` |
| `RegisteredClaims.ExpiresAt` | `time.Now().Add(ttl)` |
| `RegisteredClaims.IssuedAt` | `time.Now()` |

Nota: o construtor **não** popula `FisrtName` / `LastName` — esses fields existem na struct mas são reservados para callers que constroem claims na mão. Eles fazem round-trip via `ParseJWT`, então um token criado em outro lugar pode carregá-los.

### `ParseJWT`

Faz parse com `ParseWithClaims(&CustomClaims{}, …)` usando o secret informado. Retorna `errors.New("invalid or expired token")` em qualquer falha de verificação (**não** embrulha o erro subjacente do jwt). Retorna `errors.New("invalid claims type")` se os claims parseados não são `*CustomClaims`.

### `SignRefreshToken` — refresh token

Assinado com HS256, **sem claims customizados** — apenas `RegisteredClaims`:

| Field | Fonte |
|---|---|
| `Subject` | arg `userID` |
| `ID` | arg `sessionId` |
| `ExpiresAt` | `time.Now().Add(ttl)` |
| `IssuedAt` | `time.Now()` |

### `ParseRefreshToken`

Mesmo formato de `ParseJWT` mas com `*jwt.RegisteredClaims`.

## Uso

```go
secret := os.Getenv("JWT_SECRET")
sid := uuid.NewString()

access,  _ := jwt.SignJWT(secret, userID, sid, email, 15*time.Minute)
refresh, _ := jwt.SignRefreshToken(secret, userID, sid, 30*24*time.Hour)

claims, err := jwt.ParseJWT(secret, access)
if err != nil { return err } // "invalid or expired token"
_ = claims.UserID
```

## Notas

- Ambos helpers de parse colapsam qualquer erro em `"invalid or expired token"` / `"invalid or expired refresh token"`. Se precisa distinguir "assinatura errada" de "expirado", faça o parse com `golang-jwt` diretamente.
- Typo no nome do field: o field é `FisrtName` (com a grafia errada) e o tag JSON é `firtName`. Renomear muda o formato wire — preserve o typo quando interoperar com tokens existentes.
- A struct embeda `jwt.RegisteredClaims`, então `ParseJWT` honra validação de `exp`/`iat`/`nbf`/etc. como a biblioteca subjacente define.
