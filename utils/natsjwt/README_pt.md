# natsjwt — Emite e faz parse de NATS user JWTs (Ed25519/nkey)

Emite e faz parse de **NATS user JWTs** — as credenciais Ed25519/nkey-assinadas que leaf nodes do NATS validam localmente durante auth descentralizada. Empacota [`nats-io/jwt/v2`](https://github.com/nats-io/jwt) e [`nats-io/nkeys`](https://github.com/nats-io/nkeys) com uma API plana que espelha o formato de `utils/zerovalue` e `utils/random`.

> Nome do pacote: `natsjwt` (diretório: `natsjwt/`).

Fluxo típico:

- **Emitir** durante provisionamento de device (com o seed de assinatura da plataforma).
- **Parsear** em middleware HTTP que verifica um bearer token apresentado pelo device.

## Superfície

```go
type UserClaims struct {
    Name        string             // claim "name" do NATS — Mapex coloca o asset uuid aqui
    Issuer      string             // public key que assinou o JWT (read-only no parse)
    BearerToken bool               // true → leaf aceita JWT sem nonce challenge (caminho IoT)
    Tags        map[string]string  // map plano de strings (orgId/assetUUID/templateId, …)
    Pub         PermissionList     // subjects em que o device pode publicar
    Sub         PermissionList     // subjects em que o device pode subscrever
}

type PermissionList struct {
    Allow []string
    Deny  []string
}

func IssueUserJWT(signingKeySeed string, userPubKey string, claims UserClaims, ttl time.Duration) (jwt string, jti string, expiresAt time.Time, err error)
func ParseUserJWT(jwtToken string)                                                        (UserClaims, string, time.Time, error)
```

### `IssueUserJWT`

| Argumento | Significado |
|---|---|
| `signingKeySeed` | Seed do NKey de account (ou signing key delegada listada no JWT do account). Vazio → `"natsjwt: signing key seed is empty"`. |
| `userPubKey` | Metade pública de um keypair user gerado pelo caller. O device que tem o seed correspondente conecta como esse user. Vazio → `"natsjwt: user public key is empty"`. |
| `claims` | Fields documentados acima. |
| `ttl` | Somado a `time.Now()` para calcular o expiry. O expiry é **truncado a precisão de segundos** para que o `expiresAt` retornado bata com o que `ParseUserJWT` lerá (o "exp" do NATS é Unix-seconds). |

Retorna:

- `jwt` — string do JWT assinado.
- `jti` — JWT ID atribuído por `nats-io/jwt` (um **hash derivado do conteúdo, NÃO um UUID aleatório** — a biblioteca calcula o JTI a partir do hash do claim para tamper-evidence). Persista quando precisar revogar antes do expiry natural.
- `expiresAt` — UTC, truncado a segundos.

Tags de `claims.Tags` são emitidos como strings `"key:value"` via `uc.Tags.Add(...)`. Chaves e valores de tag são normalizados a lowercase pela biblioteca NATS; codifique case sensitivity no valor por conta própria (base64, separador, etc.) se precisar.

### Modo `BearerToken`

Quando `claims.BearerToken == true`, o leaf aceita o JWT sem desafiar o device a assinar um nonce de conexão com o seed do user nkey. Esse é o **caminho IoT** — devices conectam com `Username = devID` + `Password = JWT` e não precisam guardar o seed do user nkey. Use para JWTs de device em protocolo MQTT.

### `ParseUserJWT`

Decodifica um JWT e retorna o subset de claims. **NÃO verifica a assinatura contra um issuer** — isso é função do leaf no CONNECT MQTT e do middleware HTTP para refresh. Callers que precisam de verificação devem somar isso por cima desta leitura.

Erros:

- `"natsjwt: jwt token is empty"` em input vazio.
- `"natsjwt: decode user jwt: <err>"` em JWTs malformados.

Retorna `(claims, jti, expiresAt, nil)` em sucesso. Tags são divididos no primeiro `:` — valores que contêm dois-pontos preservam o sufixo (não há escape adicional).

## Uso

```go
// Gera keypair de user (lado do caller)
ukp, _ := nkeys.CreateUser()
userSeed, _ := ukp.Seed()
userPub, _  := ukp.PublicKey()

token, jti, exp, err := natsjwt.IssueUserJWT(
    accountSigningSeed,
    userPub,
    natsjwt.UserClaims{
        Name:        assetUUID,
        BearerToken: true, // device MQTT
        Tags:        map[string]string{"orgid": orgID, "asset": assetUUID},
        Pub:         natsjwt.PermissionList{Allow: []string{"telemetry." + assetUUID + ".>"}},
        Sub:         natsjwt.PermissionList{Allow: []string{"cmd." + assetUUID + ".>"}},
    },
    24*time.Hour,
)

// Persista (token, jti, exp) — o device recebe `token`, a plataforma armazena
// `jti` para revogação e `exp` para limpeza.

// Depois, no middleware:
claims, parsedJTI, parsedExp, err := natsjwt.ParseUserJWT(token)
```

## Notas

- A biblioteca sobrescreve o claim `ID` (JTI) com hash do conteúdo durante o `Encode`. A função decodifica o JWT recém-assinado para ler o JTI ao invés de prevê-lo.
- O parse de `Tags` usa apenas o primeiro `:` — chaves que contêm `:` não são suportadas.
- Semântica allow-vs-deny de Pub/Sub segue regras NATS: `Allow` vazio com `Deny` vazio significa "sem permissão" (broker rejeita tudo); `Allow` não-vazio com `Deny` vazio significa "exatamente esses subjects são permitidos".
