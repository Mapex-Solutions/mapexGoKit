# MinIO Client Wrapper for MapexOS

## Overview

This package provides a MinIO (S3-compatible) client wrapper following MapexOS hexagonal architecture patterns. It abstracts the underlying minio-go library, allowing for easy replacement and consistent interface across services.

## Features

- Simple object CRUD operations
- Streaming support for large objects
- Metadata support
- Automatic key prefixing
- Connection health checks
- Consistent error handling

## Installation

The package is part of MapexOS infrastructure:

```go
import "github.com/Mapex-Solutions/MapexOS/infrastructure/minio"
```

## Usage

### Basic Setup

```go
import minioModel "github.com/Mapex-Solutions/MapexOS/infrastructure/minio"

client, err := minioModel.New(minioModel.Config{
    Endpoint:        "localhost:9000",
    AccessKeyID:     "mapexos_admin",
    SecretAccessKey: "mapexos_admin_secret",
    BucketName:      "mapex-templates",
    UseSSL:          false,
})
if err != nil {
    log.Fatal(err)
}
```

### Upload Object

```go
// Simple upload
err := client.Put(ctx, "templates/script.js", jsBytes, &minioModel.PutOptions{
    ContentType: minioModel.ContentTypeJavaScript,
})

// Upload with metadata
err := client.PutWithMetadata(ctx, "assets/123.json", jsonData,
    minioModel.ContentTypeJSON,
    map[string]string{
        "version": "1.0",
        "author":  "system",
    },
)
```

### Download Object

```go
// Get entire object
result, err := client.Get(ctx, "templates/script.js")
if err != nil {
    if errors.Is(err, minioModel.ErrObjectNotFound) {
        // Handle not found
    }
    return err
}
fmt.Println(result.ContentType)
fmt.Println(len(result.Data))

// Stream large object
stream, err := client.GetStream(ctx, "exports/large-file.csv")
if err != nil {
    return err
}
defer stream.Close()
io.Copy(output, stream)
```

### List Objects

```go
objects, err := client.List(ctx, &minioModel.ListOptions{
    Prefix:    "templates/",
    Recursive: true,
    MaxKeys:   100,
})
for _, obj := range objects {
    fmt.Printf("%s: %d bytes\n", obj.Key, obj.Size)
}
```

### Check Existence

```go
exists, err := client.Exists(ctx, "templates/script.js")
if exists {
    // Object exists
}
```

### Delete Object

```go
err := client.Delete(ctx, "templates/old-script.js")
```

## Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Endpoint` | `string` | Yes | MinIO server address |
| `AccessKeyID` | `string` | Yes | Access key (like AWS) |
| `SecretAccessKey` | `string` | Yes | Secret key (like AWS) |
| `BucketName` | `string` | No | Default bucket name |
| `KeyPrefix` | `string` | No | Prefix for all keys |
| `UseSSL` | `bool` | No | Enable HTTPS |
| `Region` | `string` | No | S3 region (default: us-east-1) |

## Dependency Injection (uber/dig)

```go
func provideMinIO(cfg *config.Config) (*minioModel.MinIOClient, error) {
    return minioModel.New(minioModel.Config{
        Endpoint:        cfg.GetString("minio_endpoint"),
        AccessKeyID:     cfg.GetString("minio_access_key"),
        SecretAccessKey: cfg.GetString("minio_secret_key"),
        BucketName:      cfg.GetString("minio_bucket"),
        UseSSL:          cfg.GetBool("minio_use_ssl"),
    })
}
```

## Buckets in MapexOS

| Bucket | Purpose |
|--------|---------|
| `mapex-templates` | Asset templates (JS source + bytecode) |
| `mapex-exports` | ClickHouse exports, backups |

## Error Handling

```go
err := client.Get(ctx, key)
if errors.Is(err, minioModel.ErrObjectNotFound) {
    // Object doesn't exist
} else if errors.Is(err, minioModel.ErrDownloadFailed) {
    // Network/permission error
}
```

## Integration with TieredCache

MinIO serves as L2 (source of truth) in the TieredCache architecture:

```go
cache, err := tieredcache.New(tieredcache.Config{
    // ... L0/L1 config ...
    EnableL2: true,
    L2Loader: func(ctx context.Context, key string) ([]byte, error) {
        result, err := minioClient.Get(ctx, key)
        if err != nil {
            return nil, err
        }
        return result.Data, nil
    },
})
```
