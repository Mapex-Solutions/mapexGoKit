package minioModel

import (
	"io"
	"time"

	"github.com/minio/minio-go/v7"
)

// MinIOClient wraps the minio-go client providing a simplified interface
// for MapexOS services. Follows the hexagonal architecture pattern.
type MinIOClient struct {
	client     *minio.Client
	bucketName string
	keyPrefix  string
}

// Config holds the MinIO connection configuration.
// Used for dependency injection via uber/dig.
type Config struct {
	// Endpoint is the MinIO server address (e.g., "localhost:9000")
	Endpoint string

	// AccessKeyID is the MinIO access key (like AWS Access Key)
	AccessKeyID string

	// SecretAccessKey is the MinIO secret key (like AWS Secret Key)
	SecretAccessKey string

	// BucketName is the default bucket for this client instance
	BucketName string

	// KeyPrefix is prepended to all object keys (optional)
	KeyPrefix string

	// UseSSL enables HTTPS connections
	UseSSL bool

	// Region is the S3 region (optional, defaults to "us-east-1")
	Region string
}

// PutOptions contains options for uploading objects.
type PutOptions struct {
	// ContentType is the MIME type of the object (e.g., "application/json")
	ContentType string

	// UserMetadata are custom key-value pairs stored with the object
	UserMetadata map[string]string

	// CacheControl sets the Cache-Control header for the object
	CacheControl string

	// Expires sets the expiration time for the object
	Expires *time.Time
}

// GetResult contains the result of a Get operation.
type GetResult struct {
	// Data is the object content as bytes
	Data []byte

	// ContentType is the MIME type of the object
	ContentType string

	// Size is the size of the object in bytes
	Size int64

	// LastModified is the last modification time
	LastModified time.Time

	// ETag is the entity tag (hash) of the object
	ETag string

	// UserMetadata are custom key-value pairs stored with the object
	UserMetadata map[string]string
}

// ObjectInfo contains metadata about an object.
type ObjectInfo struct {
	// Key is the object key (path)
	Key string

	// Size is the size in bytes
	Size int64

	// LastModified is the last modification time
	LastModified time.Time

	// ETag is the entity tag (hash)
	ETag string

	// ContentType is the MIME type
	ContentType string
}

// ListOptions contains options for listing objects.
type ListOptions struct {
	// Prefix filters objects by key prefix
	Prefix string

	// Recursive lists objects recursively (default: false)
	Recursive bool

	// MaxKeys limits the number of objects returned (0 = no limit)
	MaxKeys int
}

// StreamReader wraps io.ReadCloser for streaming large objects.
type StreamReader struct {
	io.ReadCloser
	Info ObjectInfo
}
