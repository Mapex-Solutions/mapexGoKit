package redisModel

import "errors"

var (
	ErrKeyNotFound    = errors.New("key not found in cache")
	ErrNilDestination = errors.New("redis: destination is nil")
)
