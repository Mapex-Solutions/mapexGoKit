package configuration

import (
	"log"
	"sync"
)

var (
	once     sync.Once
	instance *ConfigModule
)

func InitConfig(definitions []ConfigDefinition) *ConfigModule {
	once.Do(func() {
		config := make(map[string]interface{})

		for _, def := range definitions {
			var value interface{}

			switch def.Type {
			case "string":
				value = getEnvString(def.Env, def.Default.(string))
			case "int":
				value = getEnvInt(def.Env, def.Default.(int))
			case "bool":
				value = getEnvBool(def.Env, def.Default.(bool))
			case "array":
				value = getEnvArray(def.Env, def.Default.([]string))
			case "json":
				value = getEnvJSON(def.Env, def.Default.(map[string]interface{}))
			default:
				log.Printf("Type not supported: %s to the key %s", def.Type, def.Key)
				continue
			}

			config[def.Key] = value
		}

		instance = &ConfigModule{config: config}
	})

	return instance
}
