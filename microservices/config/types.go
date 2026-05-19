package configuration

// ConfigDefinition declares a single configuration key, its env-var name,
// runtime type, and dev-friendly Default.
//
// Sensitive marks keys that carry credentials, secrets, or any value that
// must never be the hardcoded Default in a non-dev environment. When
// Sensitive is true and the resolved value still equals Default while
// GO_ENV != "dev", InitConfig refuses to start the process. See
// ValidateProductionDefaults.
type ConfigDefinition struct {
	Key       string
	Env       string
	Type      string
	Default   interface{}
	Sensitive bool
}

type ConfigModule struct {
	config map[string]interface{}
}
