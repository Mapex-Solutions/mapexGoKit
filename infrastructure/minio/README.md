# minio — Mapex MinIO/S3 client (`minioModel`)

Wrapper around [`minio/minio-go/v7`](https://github.com/minio/minio-go) providing a simplified, key-prefixed object-storage surface. Used as the L2 source-of-truth tier in `infrastructure/tieredcache` and as a generic blob store across services.

> Package name: `minioModel` (directory: `minio/`).

## Surface

### Configuration

```go
type Config struct {
    Endpoint        string  // e.g. "localhost:9000" — required
    AccessKeyID     string  // required
    SecretAccessKey string  // required
    BucketName      string  // optional; default bucket for all operations
    KeyPrefix       string  // optional; prepended to every object key
    UseSSL          bool    // false → http, true → https
    Region          string  // optional; default "us-east-1"
}
```

Required fields validated at construction time: `Endpoint`, `AccessKeyID`, `SecretAccessKey`. `BucketName` is **not** required but every read/write method below uses `m.bucketName` — if you leave it empty, you must access the bucket via `GetRawClient()`.

### Constructor

```go
func New(cfg Config) (*MinIOClient, error)
```

Sequence:

1. Validates required fields → wraps failures as `ErrInvalidConfig`.
2. Creates the underlying `minio-go` client → wraps failures as `ErrConnectionFailed`.
3. Health check via `ListBuckets` with a **5 s** timeout → wraps failures as `ErrConnectionFailed`.
4. If `BucketName != ""`, calls `BucketExists`. **Does not create the bucket** — only emits `[INFRA:MINIO] Bucket %s does not exist` (Warn) and continues.
5. Logs `[INFRA:MINIO] Initialized (bucket: %s)`.

### Constants (`constants.go`)

| Constant | Value |
|---|---|
| `ContentTypeJSON` | `application/json` |
| `ContentTypeBinary` | `application/octet-stream` |
| `ContentTypeText` | `text/plain` |
| `ContentTypeJavaScript` | `application/javascript` |
| `ContentTypeMessagePack` | `application/msgpack` |
| `DefaultRegion` | `us-east-1` |
| `DefaultMaxRetries` | `3` (declared, **not currently consumed by the wrapper**) |
| `BucketTemplates` | `mapex-templates` |
| `BucketExports` | `mapex-exports` |

### Errors (`errors.go`)

| Sentinel | Triggered when |
|---|---|
| `ErrObjectNotFound` | S3 returns `NoSuchKey` / `NoSuchBucket` (mapped by `isNotFoundError`). |
| `ErrBucketNotFound` | Reserved; not currently raised by wrapper code. |
| `ErrInvalidConfig` | `New` validation failure. Wrapped via `%w: %v`. |
| `ErrNilData` | `Put(...)` called with `data == nil`. |
| `ErrEmptyKey` | Any operation called with `key == ""`. |
| `ErrConnectionFailed` | Construction or health-check (`ListBuckets`) failure. |
| `ErrUploadFailed` | `Put` / `PutStream` underlying failure. Wrapped via `%w: %v`. |
| `ErrDownloadFailed` | `Get` / `GetStream` underlying failure. Wrapped via `%w: %v`. |

Use `errors.Is(err, ErrObjectNotFound)` etc. to discriminate.

## Methods (`*MinIOClient`)

### Diagnostics

| Method | Notes |
|---|---|
| `GetRawClient() *minio.Client` | Escape hatch — bypasses prefixing and the wrapper bucket. |
| `GetBucketName() string` | The configured default bucket. |
| `Ping(ctx) error` | Calls `ListBuckets` against the configured client. |

### Object I/O

All write/read operations use `m.bucketName` and `m.prefixKey(key)`.

| Method | Behaviour |
|---|---|
| `Put(ctx, key, data []byte, *PutOptions) error` | In-memory upload. `data == nil` → `ErrNilData`; `key == ""` → `ErrEmptyKey`. |
| `PutStream(ctx, key, io.Reader, size int64, *PutOptions) error` | Streaming upload. `size = -1` lets MinIO buffer the full body (less efficient). |
| `Get(ctx, key) (*GetResult, error)` | Reads the entire object into memory and returns metadata. `ErrObjectNotFound` on miss. |
| `GetStream(ctx, key) (*StreamReader, error)` | Streaming download. **Caller must close the returned `StreamReader`.** |
| `Delete(ctx, key) error` | `RemoveObject`. Does not error if the key does not exist (S3 semantics). |
| `Exists(ctx, key) (bool, error)` | `StatObject`-based; returns `(false, nil)` on `NoSuchKey`/`NoSuchBucket`. |
| `Stat(ctx, key) (*ObjectInfo, error)` | Metadata only. `ErrObjectNotFound` on miss. |
| `List(ctx, *ListOptions) ([]ObjectInfo, error)` | Iterates `ListObjects`; honours `Prefix` (also key-prefixed), `Recursive`, `MaxKeys`. |
| `Copy(ctx, srcKey, dstKey) error` | Server-side copy within `m.bucketName`. Both keys are prefixed. |

### Convenience helpers

| Method | Effect |
|---|---|
| `PutJSON(ctx, key, data)` | `Put` with `ContentType=ContentTypeJSON`. |
| `PutWithMetadata(ctx, key, data, contentType, metadata)` | `Put` with caller-supplied content type and `UserMetadata`. |

### `PutOptions`

```go
type PutOptions struct {
    ContentType  string             // empty → ContentTypeBinary
    UserMetadata map[string]string  // forwarded to S3 user metadata
    CacheControl string             // sets Cache-Control header
    Expires      *time.Time         // sets Expires header
}
```

`buildPutOptions(nil)` returns `{ ContentType: ContentTypeBinary }`. An explicit empty `ContentType` is also coerced to `ContentTypeBinary`.

### `GetResult` / `ObjectInfo`

```go
type GetResult struct {
    Data         []byte
    ContentType  string
    Size         int64
    LastModified time.Time
    ETag         string
    UserMetadata map[string]string
}

type ObjectInfo struct {
    Key          string
    Size         int64
    LastModified time.Time
    ETag         string
    ContentType  string
}
```

### `ListOptions`

```go
type ListOptions struct {
    Prefix    string  // also passed through prefixKey before the underlying ListObjects call
    Recursive bool
    MaxKeys   int     // 0 = no limit
}
```

### `StreamReader`

```go
type StreamReader struct {
    io.ReadCloser
    Info ObjectInfo
}
```

## Key prefixing

`prefixKey` is the single rule applied to every operation:

```go
""              → key
"prefix"        → "prefix/" + key
"prefix/"       → "prefix" + key   // already trailing-slash, no double slash
```

Tested cases (`TestPrefixKey`): `"test"+"mykey" → "test/mykey"`, `""+"mykey" → "mykey"`, `"assets"+"templates/script.js" → "assets/templates/script.js"`.

## Usage

### Upload + download JSON

```go
client, err := minioModel.New(minioModel.Config{
    Endpoint:        "localhost:9000",
    AccessKeyID:     "admin",
    SecretAccessKey: secret,
    BucketName:      minioModel.BucketTemplates,
    KeyPrefix:       "tenant-a",
    UseSSL:          false,
})
if err != nil { return err }

if err := client.PutJSON(ctx, "templates/v1/asset-123.json", payload); err != nil {
    return err
}

res, err := client.Get(ctx, "templates/v1/asset-123.json")
if errors.Is(err, minioModel.ErrObjectNotFound) {
    return loadFromOrigin()
}
if err != nil { return err }
_ = res.Data // bytes
```

### Streaming a large object

```go
sr, err := client.GetStream(ctx, "exports/big-report.csv")
if err != nil { return err }
defer sr.Close()
io.Copy(w, sr)
```

### Listing with a cap

```go
objs, err := client.List(ctx, &minioModel.ListOptions{
    Prefix:    "audit/2026-05-09",
    Recursive: true,
    MaxKeys:   100,
})
```

## Tests

- Unit tests (`TestValidateConfig`, `TestPrefixKey`, `TestBuildPutOptions`, `TestConvertObjectInfo`, `TestErrorTypes`, `TestConstants`) run with no network.
- Integration tests (`*_Integration`) skip when `New(Config)` fails (no MinIO available). Defaults: `localhost:9000`, `mapexos_admin` / `mapexos_admin_secret_change_me`, bucket `mapex-templates`, prefix `test`. Override via `MINIO_TEST_ENDPOINT`, `MINIO_TEST_ACCESS_KEY`, `MINIO_TEST_SECRET_KEY`.
- Integration coverage: `Put`/`Get`/`Exists`/`Stat`/`Delete` round-trip; non-existent `Get`; empty-key guards; nil-data guard; `List` with prefix and `MaxKeys`; `PutJSON` content-type round-trip; `Copy` round-trip.
