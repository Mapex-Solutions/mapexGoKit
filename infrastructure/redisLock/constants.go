package redisLockModel

import "time"

const (
	DefaultTries      = 3
	DefaultRetryDelay = 200 * time.Millisecond
	MinTTL            = 100 * time.Millisecond
)
