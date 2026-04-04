package redisLockModel

import (
	redsync "github.com/go-redsync/redsync/v4"
	goredis "github.com/go-redsync/redsync/v4/redis/goredis/v9"
	goredislib "github.com/redis/go-redis/v9"
)

// New creates a new LockManager using the provided go-redis client.
func New(client *goredislib.Client) *LockManager {
	pool := goredis.NewPool(client)
	return &LockManager{
		rs: redsync.New(pool),
	}
}
