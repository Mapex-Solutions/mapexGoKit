package configuration

import "sync"

// resetConfigSingleton resets the config singleton for test isolation.
// This is only available in test files (_test.go).
func resetConfigSingleton() {
	once = sync.Once{}
	instance = nil
}
