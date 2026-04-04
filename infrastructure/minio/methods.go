package minioModel

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
)

// Put uploads an object to MinIO.
//
// It accepts a key (path), data as bytes, and optional PutOptions.
// The key is automatically prefixed using the MinIOClient's keyPrefix.
//
// Critical behavior:
//   - Uses the default bucket configured in the client
//   - Automatically sets content type if not provided
//   - Stores user metadata if provided in options
func (m *MinIOClient) Put(ctx context.Context, key string, data []byte, opts *PutOptions) error {
	if key == "" {
		return ErrEmptyKey
	}
	if data == nil {
		return ErrNilData
	}

	prefixed := m.prefixKey(key)
	reader := bytes.NewReader(data)
	putOpts := m.buildPutOptions(opts)

	_, err := m.client.PutObject(ctx, m.bucketName, prefixed, reader, int64(len(data)), putOpts)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUploadFailed, err)
	}

	return nil
}

// PutStream uploads an object from a reader (for large objects).
//
// Use this for streaming large files without loading them entirely into memory.
// The size must be known in advance. Use -1 for unknown size (less efficient).
//
// Critical behavior:
//   - For unknown size (-1), MinIO will buffer the entire content
//   - Prefer providing the exact size for optimal memory usage
func (m *MinIOClient) PutStream(ctx context.Context, key string, reader io.Reader, size int64, opts *PutOptions) error {
	if key == "" {
		return ErrEmptyKey
	}

	prefixed := m.prefixKey(key)
	putOpts := m.buildPutOptions(opts)

	_, err := m.client.PutObject(ctx, m.bucketName, prefixed, reader, size, putOpts)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUploadFailed, err)
	}

	return nil
}

// Get retrieves an object from MinIO and returns its content as bytes.
//
// The key is automatically prefixed using the MinIOClient's keyPrefix.
// Returns ErrObjectNotFound if the object does not exist.
//
// Critical behavior:
//   - Loads the entire object into memory
//   - For large objects, consider using GetStream instead
func (m *MinIOClient) Get(ctx context.Context, key string) (*GetResult, error) {
	if key == "" {
		return nil, ErrEmptyKey
	}

	prefixed := m.prefixKey(key)

	obj, err := m.client.GetObject(ctx, m.bucketName, prefixed, minio.GetObjectOptions{})
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("%w: %v", ErrDownloadFailed, err)
	}
	defer obj.Close()

	// Get object info
	info, err := obj.Stat()
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("%w: %v", ErrDownloadFailed, err)
	}

	// Read all data
	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDownloadFailed, err)
	}

	return &GetResult{
		Data:         data,
		ContentType:  info.ContentType,
		Size:         info.Size,
		LastModified: info.LastModified,
		ETag:         info.ETag,
		UserMetadata: info.UserMetadata,
	}, nil
}

// GetStream retrieves an object as a stream reader (for large objects).
//
// The caller is responsible for closing the returned StreamReader.
// Use this for large files to avoid loading them entirely into memory.
//
// Critical behavior:
//   - Returns a StreamReader that must be closed by the caller
//   - Does not buffer the entire object in memory
func (m *MinIOClient) GetStream(ctx context.Context, key string) (*StreamReader, error) {
	if key == "" {
		return nil, ErrEmptyKey
	}

	prefixed := m.prefixKey(key)

	obj, err := m.client.GetObject(ctx, m.bucketName, prefixed, minio.GetObjectOptions{})
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("%w: %v", ErrDownloadFailed, err)
	}

	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		if isNotFoundError(err) {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("%w: %v", ErrDownloadFailed, err)
	}

	return &StreamReader{
		ReadCloser: obj,
		Info:       convertObjectInfo(info),
	}, nil
}

// Delete removes an object from MinIO.
//
// The key is automatically prefixed using the MinIOClient's keyPrefix.
// Does not return an error if the object does not exist.
func (m *MinIOClient) Delete(ctx context.Context, key string) error {
	if key == "" {
		return ErrEmptyKey
	}

	prefixed := m.prefixKey(key)

	err := m.client.RemoveObject(ctx, m.bucketName, prefixed, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// Exists checks if an object exists in MinIO.
//
// Returns true if the object exists, false otherwise.
// Does not download the object content.
func (m *MinIOClient) Exists(ctx context.Context, key string) (bool, error) {
	if key == "" {
		return false, ErrEmptyKey
	}

	prefixed := m.prefixKey(key)

	_, err := m.client.StatObject(ctx, m.bucketName, prefixed, minio.StatObjectOptions{})
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// Stat returns metadata about an object without downloading it.
//
// Returns ErrObjectNotFound if the object does not exist.
func (m *MinIOClient) Stat(ctx context.Context, key string) (*ObjectInfo, error) {
	if key == "" {
		return nil, ErrEmptyKey
	}

	prefixed := m.prefixKey(key)

	info, err := m.client.StatObject(ctx, m.bucketName, prefixed, minio.StatObjectOptions{})
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrObjectNotFound
		}
		return nil, err
	}

	objInfo := convertObjectInfo(info)
	return &objInfo, nil
}

// List returns a list of objects matching the given options.
//
// Use ListOptions to filter by prefix and control recursion.
// Returns a slice of ObjectInfo containing metadata for each object.
func (m *MinIOClient) List(ctx context.Context, opts *ListOptions) ([]ObjectInfo, error) {
	listOpts := minio.ListObjectsOptions{}

	if opts != nil {
		listOpts.Prefix = m.prefixKey(opts.Prefix)
		listOpts.Recursive = opts.Recursive
	}

	var objects []ObjectInfo
	for obj := range m.client.ListObjects(ctx, m.bucketName, listOpts) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		objects = append(objects, convertObjectInfo(obj))

		if opts != nil && opts.MaxKeys > 0 && len(objects) >= opts.MaxKeys {
			break
		}
	}

	return objects, nil
}

// Copy copies an object to a new key within the same bucket.
//
// Both source and destination keys are automatically prefixed.
func (m *MinIOClient) Copy(ctx context.Context, srcKey, dstKey string) error {
	if srcKey == "" || dstKey == "" {
		return ErrEmptyKey
	}

	srcPrefixed := m.prefixKey(srcKey)
	dstPrefixed := m.prefixKey(dstKey)

	src := minio.CopySrcOptions{
		Bucket: m.bucketName,
		Object: srcPrefixed,
	}

	dst := minio.CopyDestOptions{
		Bucket: m.bucketName,
		Object: dstPrefixed,
	}

	_, err := m.client.CopyObject(ctx, dst, src)
	if err != nil {
		return fmt.Errorf("failed to copy object: %w", err)
	}

	return nil
}

// PutJSON is a convenience method for uploading JSON data.
//
// Automatically sets the content type to application/json.
func (m *MinIOClient) PutJSON(ctx context.Context, key string, data []byte) error {
	return m.Put(ctx, key, data, &PutOptions{
		ContentType: ContentTypeJSON,
	})
}

// PutWithMetadata is a convenience method for uploading data with metadata.
//
// Useful for storing additional information with the object.
func (m *MinIOClient) PutWithMetadata(ctx context.Context, key string, data []byte, contentType string, metadata map[string]string) error {
	return m.Put(ctx, key, data, &PutOptions{
		ContentType:  contentType,
		UserMetadata: metadata,
	})
}
