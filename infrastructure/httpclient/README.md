# httpclient — Generic HTTP client for internal services

Lightweight `net/http` wrapper used to call internal Mapex services. Provides automatic JSON marshal/unmarshal, an `X-API-Key` auth shortcut, persistent custom headers, and an escape hatch (`Raw` / `RawWithHeaders`) for callers that need full control.

## Surface

### Config

```go
type Config struct {
    BaseURL string        // e.g. "http://localhost:5003"
    APIKey  string        // sent as X-API-Key (omitted when empty)
    Timeout time.Duration // default: 10s
}
```

### Constructor

```go
func New(config Config) *HTTPClient
```

Always succeeds — no validation. `Timeout=0` falls back to 10 seconds.

### Methods on `*HTTPClient`

| Method | Behaviour |
|---|---|
| `SetHeader(key, value)` | Registers a header attached to every subsequent request. Empty value **removes** the key. Caller headers override the standard ones (incl. `Content-Type`). |
| `Get(ctx, endpoint, result)` | GET; auto unmarshal into `result` if non-nil. |
| `Post(ctx, endpoint, body, result)` | POST; auto JSON-marshal `body` (nil = empty body), auto unmarshal into `result`. |
| `Put(ctx, endpoint, body, result)` | Same shape as `Post`. |
| `Delete(ctx, endpoint, result)` | DELETE; no body. |
| `Raw(ctx, method, endpoint, body) (*http.Response, error)` | Returns the raw response. **Caller MUST close `resp.Body`.** Does **not** check 2xx and does **not** decode. |
| `RawWithHeaders(ctx, method, endpoint, body, headers)` | Same as `Raw` but merges a one-shot `headers` map on top of `SetHeader`. The override is request-scoped — does not mutate the client. |

### Status code handling

- `Get/Post/Put/Delete` treat `200–299` as success. `300` and above return:
  ```
  request failed with status %d: %s
  ```
- `Raw` / `RawWithHeaders` surface every status via `resp.StatusCode` and never wrap it as a Go error.

### Headers applied to every request

```
Content-Type: application/json
X-API-Key:   <Config.APIKey>     (omitted when APIKey == "")
<every entry registered via SetHeader>     (caller wins; can override Content-Type)
```

### Errors

Plain `fmt.Errorf` wrapping at each failure point — no sentinel errors. Documented messages:

- `failed to marshal request body`
- `failed to create request`
- `failed to execute request`
- `failed to read response body`
- `request failed with status %d: %s`
- `failed to unmarshal response`

## Usage

### Typed helper

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

### Persistent identity headers

```go
client.SetHeader("Authorization", "Bearer "+token)
client.SetHeader("X-Org-Context", orgID)
// every subsequent call sends both
client.SetHeader("Authorization", "") // removes
```

### Per-call header (one-shot)

```go
resp, err := client.RawWithHeaders(ctx, "POST", "/auth/refresh", nil, map[string]string{
    "X-Refresh-Token": refreshToken,
})
if err != nil { return err }
defer resp.Body.Close()
// caller decides what to do with status / body
```

### Asserting on status (saga journeys)

```go
resp, err := client.Raw(ctx, "POST", "/api/assets", payload)
if err != nil { return err }
defer resp.Body.Close()
if resp.StatusCode != http.StatusCreated {
    return fmt.Errorf("expected 201, got %d", resp.StatusCode)
}
```

## Tested behaviours (`client_test.go`)

- Default `Timeout` = 10s when `Config.Timeout == 0`; custom timeout preserved.
- `BaseURL` and `APIKey` round-tripped through `New`.
- `Get`/`Post`/`Put`/`Delete` happy paths with JSON encode + decode.
- `nil` result skips unmarshal; `nil` body sends empty request body.
- 4xx and 5xx return errors for typed helpers; `Raw` does not.
- Status boundaries: `299` is success, `300` is error.
- Context cancellation propagates (50 ms ctx vs 2 s server returns error).
- Channel passed as body returns marshal error.
- Invalid JSON in 2xx response returns unmarshal error.
- `X-API-Key` set only when `Config.APIKey != ""`.
- `SetHeader` empty-value removes; can override `Content-Type` to `text/plain`.
- `Raw` preserves status, headers and body bytes verbatim.
- `RawWithHeaders` is request-scoped (subsequent `Raw` does not see the per-call header).
