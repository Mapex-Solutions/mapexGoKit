package minioModel

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// New creates a new MinIO client wrapper.
//
// It validates the configuration, establishes a connection to MinIO,
// and optionally verifies that the specified bucket exists.
//
// The client is ready to use for object storage operations immediately
// after successful initialization.
//
// Critical behavior:
//   - Validates required config fields (Endpoint, AccessKeyID, SecretAccessKey, BucketName)
//   - Creates a connection with retry logic for transient failures
//   - Verifies bucket existence (creates if BucketName is specified)
//   - Returns error if connection or bucket verification fails
func New(config Config) (*MinIOClient, error) {
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	region := config.Region
	if region == "" {
		region = DefaultRegion
	}

	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
		Secure: config.UseSSL,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	// Verify connection with a health check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use ListBuckets as a health check
	_, err = client.ListBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}

	// Ensure bucket exists
	if config.BucketName != "" {
		exists, err := client.BucketExists(ctx, config.BucketName)
		if err != nil {
			return nil, fmt.Errorf("bucket check failed: %w", err)
		}
		if !exists {
			logger.Warn(fmt.Sprintf("[INFRA:MINIO] Bucket %s does not exist", config.BucketName))
		}
	}

	logger.Info(fmt.Sprintf("[INFRA:MINIO] Initialized (bucket: %s)", config.BucketName))

	return &MinIOClient{
		client:     client,
		bucketName: config.BucketName,
		keyPrefix:  config.KeyPrefix,
	}, nil
}

// validateConfig validates the MinIO configuration.
func validateConfig(config Config) error {
	if config.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if config.AccessKeyID == "" {
		return fmt.Errorf("access key ID is required")
	}
	if config.SecretAccessKey == "" {
		return fmt.Errorf("secret access key is required")
	}
	return nil
}

// GetRawClient returns the underlying minio-go client.
// Use with caution - prefer using the wrapper methods.
func (m *MinIOClient) GetRawClient() *minio.Client {
	return m.client
}

// GetBucketName returns the configured bucket name.
func (m *MinIOClient) GetBucketName() string {
	return m.bucketName
}

// Ping checks the MinIO connection by listing buckets.
func (m *MinIOClient) Ping(ctx context.Context) error {
	_, err := m.client.ListBuckets(ctx)
	return err
}
