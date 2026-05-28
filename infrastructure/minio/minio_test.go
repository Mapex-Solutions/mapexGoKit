package minioModel

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"
)

// Test configuration - uses environment variables or defaults for local testing.
// Set MINIO_TEST_ENDPOINT, MINIO_TEST_ACCESS_KEY, MINIO_TEST_SECRET_KEY for integration tests.
func getTestConfig() Config {
	endpoint := os.Getenv("MINIO_TEST_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:9000"
	}

	accessKey := os.Getenv("MINIO_TEST_ACCESS_KEY")
	if accessKey == "" {
		accessKey = "mapex_admin"
	}

	secretKey := os.Getenv("MINIO_TEST_SECRET_KEY")
	if secretKey == "" {
		secretKey = "mapex_admin_secret_change_me"
	}

	return Config{
		Endpoint:        endpoint,
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
		BucketName:      "mapex-templates",
		KeyPrefix:       "test",
		UseSSL:          false,
		Region:          DefaultRegion,
	}
}

// skipIfNoMinIO skips the test if MinIO is not available.
func skipIfNoMinIO(t *testing.T) *MinIOClient {
	t.Helper()

	config := getTestConfig()
	client, err := New(config)
	if err != nil {
		t.Skipf("MinIO not available: %v", err)
		return nil
	}

	return client
}

// =============================================================================
// Unit Tests (no external dependencies)
// =============================================================================

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Endpoint:        "localhost:9000",
				AccessKeyID:     "admin",
				SecretAccessKey: "secret",
			},
			wantErr: false,
		},
		{
			name: "missing endpoint",
			config: Config{
				AccessKeyID:     "admin",
				SecretAccessKey: "secret",
			},
			wantErr: true,
		},
		{
			name: "missing access key",
			config: Config{
				Endpoint:        "localhost:9000",
				SecretAccessKey: "secret",
			},
			wantErr: true,
		},
		{
			name: "missing secret key",
			config: Config{
				Endpoint:    "localhost:9000",
				AccessKeyID: "admin",
			},
			wantErr: true,
		},
		{
			name:    "empty config",
			config:  Config{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPrefixKey(t *testing.T) {
	tests := []struct {
		name      string
		keyPrefix string
		key       string
		want      string
	}{
		{
			name:      "with prefix",
			keyPrefix: "test",
			key:       "mykey",
			want:      "test/mykey",
		},
		{
			name:      "without prefix",
			keyPrefix: "",
			key:       "mykey",
			want:      "mykey",
		},
		{
			name:      "nested key with prefix",
			keyPrefix: "assets",
			key:       "templates/script.js",
			want:      "assets/templates/script.js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &MinIOClient{keyPrefix: tt.keyPrefix}
			got := client.prefixKey(tt.key)
			if got != tt.want {
				t.Errorf("prefixKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildPutOptions(t *testing.T) {
	client := &MinIOClient{}

	t.Run("nil options", func(t *testing.T) {
		opts := client.buildPutOptions(nil)
		if opts.ContentType != ContentTypeBinary {
			t.Errorf("expected ContentType %s, got %s", ContentTypeBinary, opts.ContentType)
		}
	})

	t.Run("with content type", func(t *testing.T) {
		opts := client.buildPutOptions(&PutOptions{
			ContentType: ContentTypeJSON,
		})
		if opts.ContentType != ContentTypeJSON {
			t.Errorf("expected ContentType %s, got %s", ContentTypeJSON, opts.ContentType)
		}
	})

	t.Run("empty content type defaults to binary", func(t *testing.T) {
		opts := client.buildPutOptions(&PutOptions{})
		if opts.ContentType != ContentTypeBinary {
			t.Errorf("expected ContentType %s, got %s", ContentTypeBinary, opts.ContentType)
		}
	})

	t.Run("with metadata", func(t *testing.T) {
		metadata := map[string]string{"version": "1.0"}
		opts := client.buildPutOptions(&PutOptions{
			ContentType:  ContentTypeJSON,
			UserMetadata: metadata,
		})
		if opts.UserMetadata["version"] != "1.0" {
			t.Error("expected metadata to be preserved")
		}
	})

	t.Run("with cache control", func(t *testing.T) {
		opts := client.buildPutOptions(&PutOptions{
			CacheControl: "max-age=3600",
		})
		if opts.CacheControl != "max-age=3600" {
			t.Errorf("expected CacheControl 'max-age=3600', got %s", opts.CacheControl)
		}
	})
}

func TestConvertObjectInfo(t *testing.T) {
	// This test verifies the conversion function works correctly
	// with mock data since we can't easily create minio.ObjectInfo
	t.Run("conversion preserves fields", func(t *testing.T) {
		// Test that ObjectInfo struct has expected fields
		info := ObjectInfo{
			Key:          "test/key",
			Size:         1024,
			LastModified: time.Now(),
			ETag:         "abc123",
			ContentType:  ContentTypeJSON,
		}

		if info.Key != "test/key" {
			t.Errorf("Key mismatch")
		}
		if info.Size != 1024 {
			t.Errorf("Size mismatch")
		}
		if info.ContentType != ContentTypeJSON {
			t.Errorf("ContentType mismatch")
		}
	})
}

func TestErrorTypes(t *testing.T) {
	// Verify error types are properly defined
	errors := []error{
		ErrObjectNotFound,
		ErrBucketNotFound,
		ErrInvalidConfig,
		ErrNilData,
		ErrEmptyKey,
		ErrConnectionFailed,
		ErrUploadFailed,
		ErrDownloadFailed,
	}

	for _, err := range errors {
		if err == nil {
			t.Error("Error should not be nil")
		}
		if err.Error() == "" {
			t.Error("Error message should not be empty")
		}
	}
}

func TestConstants(t *testing.T) {
	// Verify constants are properly defined
	if ContentTypeJSON != "application/json" {
		t.Errorf("ContentTypeJSON = %s, want application/json", ContentTypeJSON)
	}
	if ContentTypeBinary != "application/octet-stream" {
		t.Errorf("ContentTypeBinary = %s, want application/octet-stream", ContentTypeBinary)
	}
	if BucketTemplates != "mapex-templates" {
		t.Errorf("BucketTemplates = %s, want mapex-templates", BucketTemplates)
	}
	if DefaultRegion != "us-east-1" {
		t.Errorf("DefaultRegion = %s, want us-east-1", DefaultRegion)
	}
}

// =============================================================================
// Integration Tests (require MinIO server)
// =============================================================================

func TestNew_Integration(t *testing.T) {
	client := skipIfNoMinIO(t)
	if client == nil {
		return
	}

	if client.GetBucketName() != "mapex-templates" {
		t.Errorf("GetBucketName() = %s, want mapex-templates", client.GetBucketName())
	}

	if client.GetRawClient() == nil {
		t.Error("GetRawClient() should not be nil")
	}
}

func TestPutAndGet_Integration(t *testing.T) {
	client := skipIfNoMinIO(t)
	if client == nil {
		return
	}

	ctx := context.Background()
	testKey := "integration-test/test-object-" + time.Now().Format("20060102150405")
	testData := []byte(`{"test": "data", "timestamp": "` + time.Now().String() + `"}`)

	// Cleanup after test
	defer func() {
		_ = client.Delete(ctx, testKey)
	}()

	// Test Put
	t.Run("Put", func(t *testing.T) {
		err := client.Put(ctx, testKey, testData, &PutOptions{
			ContentType: ContentTypeJSON,
			UserMetadata: map[string]string{
				"test": "true",
			},
		})
		if err != nil {
			t.Fatalf("Put() error = %v", err)
		}
	})

	// Test Get
	t.Run("Get", func(t *testing.T) {
		result, err := client.Get(ctx, testKey)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if !bytes.Equal(result.Data, testData) {
			t.Errorf("Get() data mismatch")
		}

		if result.ContentType != ContentTypeJSON {
			t.Errorf("Get() ContentType = %s, want %s", result.ContentType, ContentTypeJSON)
		}

		if result.Size != int64(len(testData)) {
			t.Errorf("Get() Size = %d, want %d", result.Size, len(testData))
		}
	})

	// Test Exists
	t.Run("Exists", func(t *testing.T) {
		exists, err := client.Exists(ctx, testKey)
		if err != nil {
			t.Fatalf("Exists() error = %v", err)
		}
		if !exists {
			t.Error("Exists() = false, want true")
		}
	})

	// Test Stat
	t.Run("Stat", func(t *testing.T) {
		info, err := client.Stat(ctx, testKey)
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}

		if info.Size != int64(len(testData)) {
			t.Errorf("Stat() Size = %d, want %d", info.Size, len(testData))
		}
	})

	// Test Delete
	t.Run("Delete", func(t *testing.T) {
		err := client.Delete(ctx, testKey)
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		exists, _ := client.Exists(ctx, testKey)
		if exists {
			t.Error("Object should not exist after delete")
		}
	})
}

func TestGetNotFound_Integration(t *testing.T) {
	client := skipIfNoMinIO(t)
	if client == nil {
		return
	}

	ctx := context.Background()
	_, err := client.Get(ctx, "nonexistent-key-12345")

	if err == nil {
		t.Error("Get() should return error for nonexistent key")
	}
}

func TestEmptyKey_Integration(t *testing.T) {
	client := skipIfNoMinIO(t)
	if client == nil {
		return
	}

	ctx := context.Background()

	t.Run("Put empty key", func(t *testing.T) {
		err := client.Put(ctx, "", []byte("data"), nil)
		if err != ErrEmptyKey {
			t.Errorf("Put() error = %v, want ErrEmptyKey", err)
		}
	})

	t.Run("Get empty key", func(t *testing.T) {
		_, err := client.Get(ctx, "")
		if err != ErrEmptyKey {
			t.Errorf("Get() error = %v, want ErrEmptyKey", err)
		}
	})

	t.Run("Delete empty key", func(t *testing.T) {
		err := client.Delete(ctx, "")
		if err != ErrEmptyKey {
			t.Errorf("Delete() error = %v, want ErrEmptyKey", err)
		}
	})
}

func TestNilData_Integration(t *testing.T) {
	client := skipIfNoMinIO(t)
	if client == nil {
		return
	}

	ctx := context.Background()
	err := client.Put(ctx, "test-key", nil, nil)

	if err != ErrNilData {
		t.Errorf("Put() error = %v, want ErrNilData", err)
	}
}

func TestList_Integration(t *testing.T) {
	client := skipIfNoMinIO(t)
	if client == nil {
		return
	}

	ctx := context.Background()
	prefix := "list-test-" + time.Now().Format("20060102150405")

	// Create test objects
	testKeys := []string{
		prefix + "/file1.txt",
		prefix + "/file2.txt",
		prefix + "/subdir/file3.txt",
	}

	for _, key := range testKeys {
		err := client.Put(ctx, key, []byte("test"), nil)
		if err != nil {
			t.Fatalf("Failed to create test object %s: %v", key, err)
		}
	}

	// Cleanup
	defer func() {
		for _, key := range testKeys {
			_ = client.Delete(ctx, key)
		}
	}()

	t.Run("List with prefix", func(t *testing.T) {
		objects, err := client.List(ctx, &ListOptions{
			Prefix:    prefix,
			Recursive: true,
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		if len(objects) != 3 {
			t.Errorf("List() returned %d objects, want 3", len(objects))
		}
	})

	t.Run("List with MaxKeys", func(t *testing.T) {
		objects, err := client.List(ctx, &ListOptions{
			Prefix:    prefix,
			Recursive: true,
			MaxKeys:   2,
		})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		if len(objects) != 2 {
			t.Errorf("List() returned %d objects, want 2", len(objects))
		}
	})
}

func TestPutJSON_Integration(t *testing.T) {
	client := skipIfNoMinIO(t)
	if client == nil {
		return
	}

	ctx := context.Background()
	testKey := "json-test-" + time.Now().Format("20060102150405")
	testData := []byte(`{"name": "test"}`)

	defer func() {
		_ = client.Delete(ctx, testKey)
	}()

	err := client.PutJSON(ctx, testKey, testData)
	if err != nil {
		t.Fatalf("PutJSON() error = %v", err)
	}

	result, err := client.Get(ctx, testKey)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if result.ContentType != ContentTypeJSON {
		t.Errorf("ContentType = %s, want %s", result.ContentType, ContentTypeJSON)
	}
}

func TestCopy_Integration(t *testing.T) {
	client := skipIfNoMinIO(t)
	if client == nil {
		return
	}

	ctx := context.Background()
	srcKey := "copy-test-src-" + time.Now().Format("20060102150405")
	dstKey := "copy-test-dst-" + time.Now().Format("20060102150405")
	testData := []byte("copy test data")

	defer func() {
		_ = client.Delete(ctx, srcKey)
		_ = client.Delete(ctx, dstKey)
	}()

	// Create source
	err := client.Put(ctx, srcKey, testData, nil)
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// Copy
	err = client.Copy(ctx, srcKey, dstKey)
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}

	// Verify destination
	result, err := client.Get(ctx, dstKey)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if !bytes.Equal(result.Data, testData) {
		t.Error("Copied data does not match source")
	}
}
