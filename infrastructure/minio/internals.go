package minioModel

import (
	"github.com/minio/minio-go/v7"
)

// prefixKey adds the configured prefix to an object key.
func (m *MinIOClient) prefixKey(key string) string {
	if m.keyPrefix == "" {
		return key
	}
	// If prefix already ends with /, don't add another one
	if m.keyPrefix[len(m.keyPrefix)-1] == '/' {
		return m.keyPrefix + key
	}
	return m.keyPrefix + "/" + key
}

// buildPutOptions converts PutOptions to minio.PutObjectOptions.
func (m *MinIOClient) buildPutOptions(opts *PutOptions) minio.PutObjectOptions {
	if opts == nil {
		return minio.PutObjectOptions{
			ContentType: ContentTypeBinary,
		}
	}

	putOpts := minio.PutObjectOptions{
		ContentType:  opts.ContentType,
		UserMetadata: opts.UserMetadata,
		CacheControl: opts.CacheControl,
	}

	if opts.ContentType == "" {
		putOpts.ContentType = ContentTypeBinary
	}

	if opts.Expires != nil {
		putOpts.Expires = *opts.Expires
	}

	return putOpts
}

// convertObjectInfo converts minio.ObjectInfo to our ObjectInfo type.
func convertObjectInfo(info minio.ObjectInfo) ObjectInfo {
	return ObjectInfo{
		Key:          info.Key,
		Size:         info.Size,
		LastModified: info.LastModified,
		ETag:         info.ETag,
		ContentType:  info.ContentType,
	}
}

// isNotFoundError checks if the error indicates the object was not found.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errResp, ok := err.(minio.ErrorResponse)
	if !ok {
		return false
	}
	return errResp.Code == "NoSuchKey" || errResp.Code == "NoSuchBucket"
}
