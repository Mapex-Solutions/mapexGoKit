package redisLockModel

import (
	"context"
	"fmt"
	"time"

	redsync "github.com/go-redsync/redsync/v4"
)

// SetLock attempts to acquire a distributed lock using context.
func (lm *LockManager) SetLock(ctx context.Context, key string, ttl time.Duration) (*redsync.Mutex, error) {
	if ttl < MinTTL {
		return nil, ErrTTLTooShort
	}

	mutex := lm.rs.NewMutex(key,
		redsync.WithExpiry(ttl),
		redsync.WithTries(DefaultTries),
		redsync.WithRetryDelay(DefaultRetryDelay),
	)

	if err := mutex.LockContext(ctx); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLockAcquire, err)
	}

	return mutex, nil
}

// SetUnlock releases the given distributed lock using context.
func (lm *LockManager) SetUnlock(ctx context.Context, mutex *redsync.Mutex) error {
	ok, err := mutex.UnlockContext(ctx)
	if !ok || err != nil {
		return fmt.Errorf("%w: %v", ErrLockRelease, err)
	}
	return nil
}

// SetWithLock runs a critical section with automatic locking using context.
func (lm *LockManager) SetWithLock(ctx context.Context, key string, ttl time.Duration, fn func() error) error {
	mutex, err := lm.SetLock(ctx, key, ttl)
	if err != nil {
		return err
	}
	defer lm.SetUnlock(ctx, mutex)

	return fn()
}
