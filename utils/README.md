# utils — Mapex shared utilities

Twenty small, self-contained packages used across `infrastructure/` and `microservices/`. Each one has a single, well-bounded purpose — no package here knows about HTTP, NATS, MongoDB, etc. unless the package itself is *about* that concern (e.g. `natsjwt`, `orgfilter`).

## Subpackages

| Package | What it is |
|---|---|
| [`bcrypt/password`](./bcrypt) | Two-line bcrypt hash + verify helpers (default cost). |
| [`deepcopy`](./deepcopy) | JSON-roundtrip deep copy for `map[string]any` (and map-of-maps). |
| [`envelope`](./envelope) | AES-256-GCM envelope encryption (Master Key + per-record DEK). |
| [`flatten`](./flatten) | Nested map ↔ flat-key translation + dotted-path lookup with array fan-out. |
| [`jsrunner`](./jsrunner) | Sandboxed ES5 transform runner over `goja` with timeout. |
| [`jwt`](./jwt) | HS256 access + refresh token helpers (`golang-jwt/jwt/v5`). |
| [`mapper`](./mapper) | Generic DTO ↔ Entity copy with ObjectId conversions and Created/Updated taps. |
| [`natsjwt`](./natsjwt) | Issue and parse NATS user JWTs (Ed25519/nkey, bearer mode). |
| [`orgfilter`](./orgfilter) | Multi-tenant org filters (Mongo + ClickHouse), pathKey scoping, template-ancestor visibility. |
| [`pathkey`](./pathkey) | Hierarchical pathKey utilities — next-sibling, ancestors, descendant checks. |
| [`random`](./random) | `crypto/rand`-backed run IDs and session IDs. |
| [`serialize`](./serialize) | Thin JSON marshal/unmarshal wrappers for swap-out. |
| [`slice`](./slice) | Generic slice utilities (currently `Reverse`). |
| [`templatereplace`](./templatereplace) | `{{path.to.value}}` interpolation in JSON-like trees. |
| [`time`](./time) | RFC3339-with-milliseconds helpers + `NullTime`. |
| [`typeconv`](./typeconv) | Permissive `interface{}` → typed conversions with `Tryxxx` variants. |
| [`validations`](./validations) | Singleton `go-playground/validator` with Mapex extras (`mongoid`, `uuid`) and humanised messages. |
| [`zerovalue`](./zerovalue) | Zero-value generators by string-keyed type plus generic `Ptr[T]`. |

## How packages relate

Most packages stand alone. The cross-package edges that matter:

- `mapper` depends on `serialize` (JSON round-trips) and `time` (`NullTime`).
- `orgfilter` depends on `pathkey` (range queries) and on `microservices/common/context` + `infrastructure/mongodb/model` (request scope and ObjectID parsing).
- `validations/customvalidation` is the home for new field-level rules; `validations.New()` registers them on first use.
- `serialize` is the single JSON entry point — change `serialize.Marshal/Unmarshal` to swap encoders globally.

## Conventions

- **Singletons** are `sync.Once`-guarded (`validations.New`, `microservices/logger.InitLogger`, etc.) — first call wins, subsequent calls return the existing instance.
- **No HTTP errors here.** Functions return `error` or sentinel `errors.New(...)` — consumers map them to status codes.
- **Errors that mean nothing happened** (e.g. `pathkey.IsDescendant` returning `false`) are returned as `bool`, not `error`.
- **Deep copies are JSON-shaped.** `deepcopy` and `mapper.*Map*` round-trip through JSON — non-JSON values (channels, functions, NaN/Inf, time precision) lose information.

For details on every subpackage — types, defaults, edge cases, tested behaviours — see the README in each directory.
