package redisLockModel

import redsync "github.com/go-redsync/redsync/v4"

// LockManager handles distributed locking with Redis.
type LockManager struct {
	rs *redsync.Redsync
}
