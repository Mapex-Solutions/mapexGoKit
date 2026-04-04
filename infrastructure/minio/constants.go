package minioModel

// Default content types for common file formats.
const (
	ContentTypeJSON        = "application/json"
	ContentTypeBinary      = "application/octet-stream"
	ContentTypeText        = "text/plain"
	ContentTypeJavaScript  = "application/javascript"
	ContentTypeMessagePack = "application/msgpack"
)

// Default configuration values.
const (
	DefaultRegion     = "us-east-1"
	DefaultMaxRetries = 3
)

// Bucket prefixes for MapexOS.
const (
	BucketTemplates = "mapex-templates"
	BucketExports   = "mapex-exports"
)
