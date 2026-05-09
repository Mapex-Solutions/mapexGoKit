# random — Run IDs and session IDs from `crypto/rand`

Two cryptographically-secure helpers used across Mapex services for ephemeral identifiers.

> Package name: `random` (directory: `random/`).

## Surface

```go
func NewRunID() string
func GenerateSessionID(length int) (string, error)
```

### `NewRunID`

Returns an identifier safe to embed into payloads whose entries are later cleaned up by prefix scan. Format:

```
YYYYMMDDhhmmss-XXXXXXXX
```

- The first segment is the **UTC** timestamp at the moment `NewRunID` was called (compact, no separators).
- The second segment is **4 random bytes** (`crypto/rand`) hex-encoded as 8 chars.

The timestamp guarantees lexicographic order across runs; the random suffix guarantees uniqueness when several runs start in the same second.

```go
id := random.NewRunID() // "20260509T143012-9a7b3c4d" — order-friendly + unique
```

### `GenerateSessionID`

Returns a hex-encoded random string of `2*length` characters (each byte → 2 hex chars). Uses `crypto/rand`, so failure to read entropy is propagated.

| `length` | Output length | Example |
|---:|---:|---|
| `4` | `8` chars | `9a7b3c4d` |
| `16` | `32` chars | `4f2a9b3e8d7c6b1a…` |

```go
sid, err := random.GenerateSessionID(4)
if err != nil { return err }
```

## Notes

- `NewRunID` ignores the error from `rand.Read` — entropy failure produces an all-zero suffix in practice. The intended use (run IDs) is non-security-critical, so this trade-off is acceptable.
- `GenerateSessionID` propagates the error, so callers can fall back or fail loudly.
