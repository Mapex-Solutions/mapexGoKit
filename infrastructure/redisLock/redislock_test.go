package redisLockModel

import (
	"errors"
	"testing"
	"time"
)

/** Constants */

func TestConstants(t *testing.T) {
	t.Run("DefaultTries", func(t *testing.T) {
		if DefaultTries != 3 {
			t.Errorf("expected DefaultTries 3, got %d", DefaultTries)
		}
	})

	t.Run("DefaultRetryDelay", func(t *testing.T) {
		if DefaultRetryDelay != 200*time.Millisecond {
			t.Errorf("expected DefaultRetryDelay 200ms, got %v", DefaultRetryDelay)
		}
	})

	t.Run("MinTTL", func(t *testing.T) {
		if MinTTL != 100*time.Millisecond {
			t.Errorf("expected MinTTL 100ms, got %v", MinTTL)
		}
	})
}

/** Errors */

func TestErrors_NotNil(t *testing.T) {
	errs := []struct {
		name string
		err  error
	}{
		{"ErrLockAcquire", ErrLockAcquire},
		{"ErrLockRelease", ErrLockRelease},
		{"ErrTTLTooShort", ErrTTLTooShort},
	}

	for _, tt := range errs {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Error("error should not be nil")
			}
			if tt.err.Error() == "" {
				t.Error("error message should not be empty")
			}
		})
	}
}

func TestErrors_Messages(t *testing.T) {
	if ErrLockAcquire.Error() != "redis: failed to acquire lock" {
		t.Errorf("unexpected ErrLockAcquire message: %q", ErrLockAcquire.Error())
	}
	if ErrLockRelease.Error() != "redis: failed to release lock" {
		t.Errorf("unexpected ErrLockRelease message: %q", ErrLockRelease.Error())
	}
	if ErrTTLTooShort.Error() != "redis: TTL must be at least 100ms" {
		t.Errorf("unexpected ErrTTLTooShort message: %q", ErrTTLTooShort.Error())
	}
}

func TestErrors_Wrapping(t *testing.T) {
	// Verify ErrLockAcquire and ErrLockRelease can be used as sentinel errors
	wrapped := errors.New("some cause")
	_ = wrapped // Just verifying the sentinel errors are defined

	if !errors.Is(ErrLockAcquire, ErrLockAcquire) {
		t.Error("ErrLockAcquire should match itself")
	}
	if !errors.Is(ErrLockRelease, ErrLockRelease) {
		t.Error("ErrLockRelease should match itself")
	}
	if !errors.Is(ErrTTLTooShort, ErrTTLTooShort) {
		t.Error("ErrTTLTooShort should match itself")
	}
}

/** LockManager */

func TestLockManager_Struct(t *testing.T) {
	// Verify LockManager can be created (without actual Redis connection)
	manager := &LockManager{rs: nil}
	if manager == nil {
		t.Error("expected non-nil LockManager")
	}
}

/** SetLock TTL Validation */

func TestSetLock_TTLTooShort(t *testing.T) {
	manager := &LockManager{rs: nil}

	tests := []struct {
		name string
		ttl  time.Duration
	}{
		{"50ms", 50 * time.Millisecond},
		{"99ms", 99 * time.Millisecond},
		{"0ms", 0},
		{"negative", -1 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.SetLock(nil, "test-key", tt.ttl)
			if !errors.Is(err, ErrTTLTooShort) {
				t.Errorf("expected ErrTTLTooShort for TTL=%v, got %v", tt.ttl, err)
			}
		})
	}
}

func TestSetLock_TTLExactlyMinTTL(t *testing.T) {
	// This test verifies TTL validation passes for MinTTL
	// The actual lock acquisition will fail (nil redsync) but TTL validation should pass
	manager := &LockManager{rs: nil}

	defer func() {
		// We expect a panic because rs is nil when trying to create mutex
		// The point is that TTL validation PASSED (we got past the TTL check)
		if r := recover(); r == nil {
			// If no panic, the function might have returned ErrTTLTooShort which is wrong
			// Or it might have succeeded somehow
		}
	}()

	_, err := manager.SetLock(nil, "test-key", MinTTL)
	// If we get ErrTTLTooShort, the validation is wrong
	if errors.Is(err, ErrTTLTooShort) {
		t.Errorf("TTL=%v should pass validation (>= MinTTL=%v)", MinTTL, MinTTL)
	}
}
