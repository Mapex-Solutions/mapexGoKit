# minio — Cliente MinIO/S3 do Mapex (`minioModel`)

Wrapper sobre [`minio/minio-go/v7`](https://github.com/minio/minio-go) provendo uma superfície simplificada e prefixada para object storage. Usado como camada L2 (fonte da verdade) em `infrastructure/tieredcache` e como blob store genérico entre serviços.

> Nome do pacote: `minioModel` (diretório: `minio/`).

## Superfície

### Configuração

```go
type Config struct {
    Endpoint        string  // ex: "localhost:9000" — obrigatório
    AccessKeyID     string  // obrigatório
    SecretAccessKey string  // obrigatório
    BucketName      string  // opcional; bucket padrão para todas as operações
    KeyPrefix       string  // opcional; prefixado em toda key de objeto
    UseSSL          bool    // false → http, true → https
    Region          string  // opcional; padrão "us-east-1"
}
```

Campos obrigatórios validados em construção: `Endpoint`, `AccessKeyID`, `SecretAccessKey`. `BucketName` **não** é obrigatório, mas todos os métodos abaixo usam `m.bucketName` — se deixar vazio, será necessário acessar o bucket via `GetRawClient()`.

### Construtor

```go
func New(cfg Config) (*MinIOClient, error)
```

Sequência:

1. Valida campos obrigatórios → falhas embrulhadas como `ErrInvalidConfig`.
2. Cria o cliente subjacente do `minio-go` → falhas embrulhadas como `ErrConnectionFailed`.
3. Health check via `ListBuckets` com timeout de **5 s** → falhas embrulhadas como `ErrConnectionFailed`.
4. Se `BucketName != ""`, chama `BucketExists`. **Não cria o bucket** — apenas emite `[INFRA:MINIO] Bucket %s does not exist` (Warn) e segue.
5. Loga `[INFRA:MINIO] Initialized (bucket: %s)`.

### Constantes (`constants.go`)

| Constante | Valor |
|---|---|
| `ContentTypeJSON` | `application/json` |
| `ContentTypeBinary` | `application/octet-stream` |
| `ContentTypeText` | `text/plain` |
| `ContentTypeJavaScript` | `application/javascript` |
| `ContentTypeMessagePack` | `application/msgpack` |
| `DefaultRegion` | `us-east-1` |
| `DefaultMaxRetries` | `3` (declarado, **não consumido pelo wrapper hoje**) |
| `BucketTemplates` | `mapex-templates` |
| `BucketExports` | `mapex-exports` |

### Erros (`errors.go`)

| Sentinel | Disparado quando |
|---|---|
| `ErrObjectNotFound` | S3 retorna `NoSuchKey` / `NoSuchBucket` (mapeado por `isNotFoundError`). |
| `ErrBucketNotFound` | Reservado; não é levantado pelo código do wrapper hoje. |
| `ErrInvalidConfig` | Falha de validação em `New`. Embrulhado via `%w: %v`. |
| `ErrNilData` | `Put(...)` chamado com `data == nil`. |
| `ErrEmptyKey` | Qualquer operação chamada com `key == ""`. |
| `ErrConnectionFailed` | Falha de construção ou health-check (`ListBuckets`). |
| `ErrUploadFailed` | Falha em `Put` / `PutStream`. Embrulhado via `%w: %v`. |
| `ErrDownloadFailed` | Falha em `Get` / `GetStream`. Embrulhado via `%w: %v`. |

Use `errors.Is(err, ErrObjectNotFound)` etc. para discriminar.

## Métodos (`*MinIOClient`)

### Diagnóstico

| Método | Notas |
|---|---|
| `GetRawClient() *minio.Client` | Escape hatch — ignora prefixing e o bucket do wrapper. |
| `GetBucketName() string` | Bucket padrão configurado. |
| `Ping(ctx) error` | Chama `ListBuckets` no cliente configurado. |

### I/O de objetos

Toda operação read/write usa `m.bucketName` e `m.prefixKey(key)`.

| Método | Comportamento |
|---|---|
| `Put(ctx, key, data []byte, *PutOptions) error` | Upload em memória. `data == nil` → `ErrNilData`; `key == ""` → `ErrEmptyKey`. |
| `PutStream(ctx, key, io.Reader, size int64, *PutOptions) error` | Upload por streaming. `size = -1` faz MinIO buferizar o body inteiro (menos eficiente). |
| `Get(ctx, key) (*GetResult, error)` | Lê o objeto inteiro em memória e retorna metadados. `ErrObjectNotFound` em miss. |
| `GetStream(ctx, key) (*StreamReader, error)` | Download por streaming. **O caller deve fechar o `StreamReader` retornado.** |
| `Delete(ctx, key) error` | `RemoveObject`. Não erra se a chave não existe (semântica S3). |
| `Exists(ctx, key) (bool, error)` | Baseado em `StatObject`; retorna `(false, nil)` em `NoSuchKey`/`NoSuchBucket`. |
| `Stat(ctx, key) (*ObjectInfo, error)` | Apenas metadados. `ErrObjectNotFound` em miss. |
| `List(ctx, *ListOptions) ([]ObjectInfo, error)` | Itera `ListObjects`; respeita `Prefix` (também recebe key-prefix), `Recursive`, `MaxKeys`. |
| `Copy(ctx, srcKey, dstKey) error` | Cópia server-side dentro de `m.bucketName`. Ambas as chaves são prefixadas. |

### Helpers de conveniência

| Método | Efeito |
|---|---|
| `PutJSON(ctx, key, data)` | `Put` com `ContentType=ContentTypeJSON`. |
| `PutWithMetadata(ctx, key, data, contentType, metadata)` | `Put` com content type e `UserMetadata` informados pelo caller. |

### `PutOptions`

```go
type PutOptions struct {
    ContentType  string             // vazio → ContentTypeBinary
    UserMetadata map[string]string  // encaminhado para user metadata do S3
    CacheControl string             // header Cache-Control
    Expires      *time.Time         // header Expires
}
```

`buildPutOptions(nil)` retorna `{ ContentType: ContentTypeBinary }`. `ContentType` vazio também é coercido para `ContentTypeBinary`.

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
    Prefix    string  // também passa por prefixKey antes do ListObjects subjacente
    Recursive bool
    MaxKeys   int     // 0 = sem limite
}
```

### `StreamReader`

```go
type StreamReader struct {
    io.ReadCloser
    Info ObjectInfo
}
```

## Prefixação de chaves

`prefixKey` é a única regra aplicada a toda operação:

```go
""              → key
"prefix"        → "prefix/" + key
"prefix/"       → "prefix" + key   // já tem trailing slash, evita barra dupla
```

Casos testados (`TestPrefixKey`): `"test"+"mykey" → "test/mykey"`, `""+"mykey" → "mykey"`, `"assets"+"templates/script.js" → "assets/templates/script.js"`.

## Uso

### Upload + download de JSON

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

### Streaming de objeto grande

```go
sr, err := client.GetStream(ctx, "exports/big-report.csv")
if err != nil { return err }
defer sr.Close()
io.Copy(w, sr)
```

### Listagem com limite

```go
objs, err := client.List(ctx, &minioModel.ListOptions{
    Prefix:    "audit/2026-05-09",
    Recursive: true,
    MaxKeys:   100,
})
```

## Testes

- Testes unitários (`TestValidateConfig`, `TestPrefixKey`, `TestBuildPutOptions`, `TestConvertObjectInfo`, `TestErrorTypes`, `TestConstants`) rodam sem rede.
- Testes de integração (`*_Integration`) são pulados quando `New(Config)` falha (MinIO indisponível). Padrões: `localhost:9000`, `mapexos_admin` / `mapexos_admin_secret_change_me`, bucket `mapex-templates`, prefixo `test`. Sobrescreva via `MINIO_TEST_ENDPOINT`, `MINIO_TEST_ACCESS_KEY`, `MINIO_TEST_SECRET_KEY`.
- Cobertura de integração: round-trip `Put`/`Get`/`Exists`/`Stat`/`Delete`; `Get` em chave inexistente; guards de chave vazia; guard de data nula; `List` com prefix e `MaxKeys`; round-trip de content-type via `PutJSON`; round-trip de `Copy`.
