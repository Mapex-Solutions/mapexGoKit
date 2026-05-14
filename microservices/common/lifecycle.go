package common

import (
	"fmt"

	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// Mountable is an optional lifecycle interface for services.
// Services that implement OnMount will have it called after construction,
// when all dependencies are wired and the service is ready to operate.
// Use for: bootstrap schedules, seed data, publish initial messages.
type Mountable interface {
	OnMount()
}

// RunLifecycleHooks checks if a service implements lifecycle interfaces and calls them.
// Safe to call with any service — skips silently if the service has no lifecycle hooks.
//
// Usage in module.go:
//
//	c.Invoke(func(svc ports.MyServicePort) {
//	    common.RunLifecycleHooks(svc, "MyModule")
//	})
func RunLifecycleHooks(service interface{}, moduleName string) {
	if m, ok := service.(Mountable); ok {
		logger.Info(fmt.Sprintf("[MODULE:%s] Running OnMount lifecycle hook", moduleName))
		m.OnMount()
	}
}
