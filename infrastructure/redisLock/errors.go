package redisLockModel

import "errors"

var (
	ErrLockAcquire  = errors.New("redis: failed to acquire lock")
	ErrLockRelease  = errors.New("redis: failed to release lock")
	ErrTTLTooShort  = errors.New("redis: TTL must be at least 100ms")
)
