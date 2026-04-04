package minioModel

import "errors"

var (
	// ErrObjectNotFound is returned when the requested object does not exist.
	ErrObjectNotFound = errors.New("object not found")

	// ErrBucketNotFound is returned when the bucket does not exist.
	ErrBucketNotFound = errors.New("bucket not found")

	// ErrInvalidConfig is returned when the configuration is invalid.
	ErrInvalidConfig = errors.New("invalid minio configuration")

	// ErrNilData is returned when attempting to upload nil data.
	ErrNilData = errors.New("data cannot be nil")

	// ErrEmptyKey is returned when the object key is empty.
	ErrEmptyKey = errors.New("object key cannot be empty")

	// ErrConnectionFailed is returned when MinIO connection fails.
	ErrConnectionFailed = errors.New("failed to connect to MinIO")

	// ErrUploadFailed is returned when object upload fails.
	ErrUploadFailed = errors.New("failed to upload object")

	// ErrDownloadFailed is returned when object download fails.
	ErrDownloadFailed = errors.New("failed to download object")
)
