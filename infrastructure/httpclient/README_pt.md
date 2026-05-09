# httpclient — Cliente HTTP genérico para serviços internos

Wrapper leve sobre `net/http` usado para chamar serviços Mapex internos. Faz marshal/unmarshal JSON automático, atalho de auth via `X-API-Key`, headers customizados persistentes e uma escape hatch (`Raw` / `RawWithHeaders`) para callers que precisam de controle total.

## Superfície

### Config

```go
type Config struct {
    BaseURL string        // ex: "http://localhost:5003"
    APIKey  string        // enviado como X-API-Key (omitido quando vazio)
    Timeout time.Duration // padrão: 10s
}
```

### Construtor

```go
func New(config Config) *HTTPClient
```

Sempre tem sucesso — sem validação. `Timeout=0` cai no fallback de 10 segundos.

### Métodos em `*HTTPClient`

| Método | Comportamento |
|---|---|
| `SetHeader(key, value)` | Registra um header anexado a toda requisição subsequente. Valor vazio **remove** a chave. Headers do caller sobrepõem os padrões (inclusive `Content-Type`). |
| `Get(ctx, endpoint, result)` | GET; faz unmarshal automático em `result` se não-nil. |
| `Post(ctx, endpoint, body, result)` | POST; auto JSON-marshal de `body` (nil = body vazio), auto unmarshal em `result`. |
| `Put(ctx, endpoint, body, result)` | Mesma assinatura de `Post`. |
| `Delete(ctx, endpoint, result)` | DELETE; sem body. |
| `Raw(ctx, method, endpoint, body) (*http.Response, error)` | Retorna a resposta crua. **O caller DEVE fechar `resp.Body`.** **Não** verifica 2xx e **não** decodifica. |
| `RawWithHeaders(ctx, method, endpoint, body, headers)` | Igual a `Raw` mas mescla um map `headers` por chamada sobre os de `SetHeader`. O override é escopo de requisição — não muta o cliente. |

### Tratamento de status code

- `Get/Post/Put/Delete` tratam `200–299` como sucesso. `300` ou superior retorna:
  ```
  request failed with status %d: %s
  ```
- `Raw` / `RawWithHeaders` expõem qualquer status via `resp.StatusCode` e nunca empacotam como erro Go.

### Headers aplicados a toda requisição

```
Content-Type: application/json
X-API-Key:   <Config.APIKey>     (omitido quando APIKey == "")
<cada entrada registrada via SetHeader>     (caller vence; pode sobrescrever Content-Type)
```

### Erros

`fmt.Errorf` simples em cada ponto de falha — sem erros sentinel. Mensagens documentadas:

- `failed to marshal request body`
- `failed to create request`
- `failed to execute request`
- `failed to read response body`
- `request failed with status %d: %s`
- `failed to unmarshal response`

## Uso

### Helper tipado

```go
client := httpclient.New(httpclient.Config{
    BaseURL: "http://routegroups-svc:5003",
    APIKey:  os.Getenv("INTERNAL_API_KEY"),
    Timeout: 5 * time.Second,
})

var groups []RouteGroupResponse
if err := client.Get(ctx, "/api/internal/v1/routegroups?ids=id1,id2", &groups); err != nil {
    return err
}
```

### Headers persistentes de identidade

```go
client.SetHeader("Authorization", "Bearer "+token)
client.SetHeader("X-Org-Context", orgID)
// toda chamada seguinte envia ambos
client.SetHeader("Authorization", "") // remove
```

### Header por chamada (one-shot)

```go
resp, err := client.RawWithHeaders(ctx, "POST", "/auth/refresh", nil, map[string]string{
    "X-Refresh-Token": refreshToken,
})
if err != nil { return err }
defer resp.Body.Close()
// caller decide o que fazer com status / body
```

### Asserção sobre status (saga journeys)

```go
resp, err := client.Raw(ctx, "POST", "/api/assets", payload)
if err != nil { return err }
defer resp.Body.Close()
if resp.StatusCode != http.StatusCreated {
    return fmt.Errorf("expected 201, got %d", resp.StatusCode)
}
```

## Comportamentos testados (`client_test.go`)

- `Timeout` padrão = 10s quando `Config.Timeout == 0`; timeout customizado é preservado.
- `BaseURL` e `APIKey` retornam corretamente após `New`.
- Caminhos felizes de `Get`/`Post`/`Put`/`Delete` com encode + decode JSON.
- `nil` em `result` pula unmarshal; `nil` em `body` envia body vazio.
- 4xx e 5xx retornam erros para os helpers tipados; `Raw` não.
- Limites de status: `299` é sucesso, `300` é erro.
- Cancelamento de contexto se propaga (ctx de 50 ms vs servidor de 2 s retorna erro).
- Channel passado como body retorna erro de marshal.
- JSON inválido em resposta 2xx retorna erro de unmarshal.
- `X-API-Key` é setado apenas quando `Config.APIKey != ""`.
- `SetHeader` com valor vazio remove; pode sobrescrever `Content-Type` para `text/plain`.
- `Raw` preserva status, headers e bytes do body verbatim.
- `RawWithHeaders` tem escopo de requisição (`Raw` subsequente não vê o header por-chamada).
