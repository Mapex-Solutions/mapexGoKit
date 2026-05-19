package configuration

import (
	"log"
	"os"
	"strings"
	"sync"
)

const (
	ansiReset   = "\033[0m"
	ansiBoldRed = "\033[1;31m"
)

var (
	once     sync.Once
	instance *ConfigModule
)

// logFatalf and logWarnf are package-level indirections over the stdlib
// log functions so tests can substitute non-exiting stubs to assert the
// production-default guard behavior.
var (
	logFatalf = log.Fatalf
	logWarnf  = log.Printf
)

// paintRed wraps text in ANSI bold-red so the prefix tag stands out in
// terminal output. Honors the NO_COLOR convention (https://no-color.org)
// so log capture in CI or piped output stays clean.
func paintRed(s string) string {
	if os.Getenv("NO_COLOR") != "" {
		return s
	}
	return ansiBoldRed + s + ansiReset
}

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

		goEnv, _ := config["go_env"].(string)
		if violations := FindSensitiveDefaultsInUse(definitions, config); len(violations) > 0 {
			envs := make([]string, 0, len(violations))
			for _, v := range violations {
				envs = append(envs, v.Env)
			}
			joined := strings.Join(envs, ", ")

			if IsDevEnv(goEnv) {
				logWarnf(
					"%s GO_ENV=%s — %d sensitive env var(s) using DEV defaults: %s. Safe for local development. NEVER deploy with these values.",
					paintRed("[SECURITY WARNING]"), goEnv, len(violations), joined,
				)
			} else {
				logFatalf(
					"%s refusing to start in GO_ENV=%s — sensitive env vars using DEV defaults: %s. Set them to production values before deploying.",
					paintRed("[SECURITY]"), goEnv, joined,
				)
				return
			}
		}

		instance = &ConfigModule{config: config}
	})

	return instance
}
