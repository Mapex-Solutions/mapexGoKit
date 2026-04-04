package module

// ModuleConfig defines the configuration for a module initialization.
// This structure is used across all services (mapexos, router, http_gateway, assets, etc.)
// to standardize module initialization in 3 phases:
//  1. InitRepositories - Registers all repositories in DIG container
//  2. InitServices - Registers all services in DIG container
//  3. InitInterfaces - Registers HTTP routes and consumers
//
// All init functions are optional (can be nil).
// The Lazy flag is reserved for future implementation of lazy loading.
type ModuleConfig struct {
	// Name is the module identifier
	Name string

	// Lazy indicates if the module should be loaded lazily (not implemented yet)
	Lazy bool

	// InitRepositories registers repositories in the DIG container
	// Optional - set to nil if module has no repositories
	InitRepositories func()

	// InitServices registers services in the DIG container
	// Optional - set to nil if module has no services
	InitServices func()

	// InitInterfaces registers HTTP routes and message consumers
	// Optional - set to nil if module has no interfaces
	InitInterfaces func()
}
