package configuration

type ConfigDefinition struct {
	Key     string
	Env     string
	Type    string
	Default interface{}
}

type ConfigModule struct {
	config map[string]interface{}
}
