// Package configuration provides helpers to read strongly-typed settings
// from a process-wide configuration singleton. Call InitConfig (not shown
// here) before using any getters; otherwise, these functions will panic.
package configuration

import (
	"fmt"
	"strings"
	"time"

	clickhouseModel "github.com/Mapex-Solutions/mapexGoKit/infrastructure/clickhouse"
	mongoManager "github.com/Mapex-Solutions/mapexGoKit/infrastructure/mongodb/manager"
	natsModel "github.com/Mapex-Solutions/mapexGoKit/infrastructure/nats"
	redisModel "github.com/Mapex-Solutions/mapexGoKit/infrastructure/redis"

	middlewaresAuth "github.com/Mapex-Solutions/mapexGoKit/microservices/http/middlewares/auth"
	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// GetConfigValue returns the raw value stored for the given key, or nil if the
// key does not exist.
//
// It panics if the configuration singleton has not been initialized yet
// (i.e., InitConfig was not called).
func GetConfigValue(key string) interface{} {
	if instance == nil {
		panic("Config not initialized. Call InitConfig first.")
	}
	if val, exists := instance.config[key]; exists {
		return val
	}
	return nil
}

// GetStringValue returns the string value stored for the given key.
//
// It returns an error if the key does not exist or if the stored value cannot
// be asserted to string. It panics if the configuration singleton is not
// initialized.
func GetStringValue(key string) (string, error) {
	if instance == nil {
		panic("Config not initialized. Call InitConfig first.")
	}

	val, exists := instance.config[key]
	if !exists {
		return "", fmt.Errorf("key %s not found", key)
	}
	strVal, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("error converting %s to string", key)
	}
	return strVal, nil
}

// GetIntValue returns the int value stored for the given key.
//
// It returns an error if the key does not exist or if the stored value cannot
// be asserted to int. It panics if the configuration singleton is not
// initialized.
func GetIntValue(key string) (int, error) {
	if instance == nil {
		panic("Config not initialized. Call InitConfig first.")
	}

	val, exists := instance.config[key]
	if !exists {
		return 0, fmt.Errorf("key %s not found", key)
	}
	intVal, ok := val.(int)
	if !ok {
		return 0, fmt.Errorf("error converting %s to int", key)
	}
	return intVal, nil
}

// GetBoolValue returns the bool value stored for the given key.
//
// It returns an error if the key does not exist or if the stored value cannot
// be asserted to bool. It panics if the configuration singleton is not
// initialized.
func GetBoolValue(key string) (bool, error) {
	if instance == nil {
		panic("Config not initialized. Call InitConfig first.")
	}

	val, exists := instance.config[key]
	if !exists {
		return false, fmt.Errorf("key %s not found", key)
	}
	boolVal, ok := val.(bool)
	if !ok {
		return false, fmt.Errorf("error converting %s to bool", key)
	}
	return boolVal, nil
}

// GetMongoConfig builds and returns a mongoManager.Config using the current
// configuration keys:
//
//   - "mongo_uri" (string): connection URI
//   - "mongo_database" (string): default database name
//
// It enables the connection monitor with a 10-second interval by default.
// Missing keys will cause GetStringValue to return empty strings, which may
// lead to downstream connection errors. This function panics if the config
// singleton is not initialized.
func GetMongoConfig() mongoManager.Config {
	mongoUri, _ := GetStringValue("mongo_uri")
	mongoDB, _ := GetStringValue("mongo_database")
	go_env, _ := GetStringValue("go_env")

	mongoConfig := mongoManager.Config{
		URI:             mongoUri,
		Database:        go_env + "-" + mongoDB,
		EnableMonitor:   true,
		MonitorInterval: 10,
	}

	return mongoConfig
}

// GetAuthConfig returns a middlewaresAuth.AuthConfig assembled from the
// following keys:
//
//   - "auth_strategy"    (string): "jwt" or "oauth2"
//   - "auth_secret"      (string): required for JWT HS256
//   - "auth_jwks_url"    (string): required for JWT RS256 and OAuth2
//   - "auth_algorithm"   (string): e.g., "HS256" or "RS256"
//   - "auth_roles_source"(string): "claims" or "api"
//   - "auth_roles_path"  (string): JSON path in claims (when source is "claims")
//   - "auth_roles_api_url"(string): roles endpoint (when source is "api")
//
// It logs validation errors (using logger.Error) for inconsistent combinations
// (e.g., HS256 without secret, RS256/OAuth2 without JWKS URL) but does not
// return an error. Callers should ensure required keys are present for the
// chosen strategy. Panics if the config singleton is not initialized.
func GetAuthConfig() middlewaresAuth.AuthConfig {
	strategy, _ := GetStringValue("auth_strategy")
	secret, _ := GetStringValue("auth_secret")
	jwksUrl, _ := GetStringValue("auth_jwks_url")
	algorithm, _ := GetStringValue("auth_algorithm")
	rolesSource, _ := GetStringValue("auth_roles_source")
	rolesPath, _ := GetStringValue("auth_roles_path")
	rolesApiURL, _ := GetStringValue("auth_roles_api_url")

	switch strategy {
	case "jwt":
		if algorithm == "HS256" && secret == "" {
			logger.Error(nil, "AuthConfig validation error: JWT strategy with HS256 requires 'auth_secret'")
		}
		if algorithm == "RS256" && jwksUrl == "" {
			logger.Error(nil, "AuthConfig validation error: JWT strategy with RS256 requires 'auth_jwks_url'")
		}
	case "oauth2":
		if jwksUrl == "" {
			logger.Error(nil, "AuthConfig validation error: OAuth2 strategy requires 'auth_jwks_url'")
		}
	default:
		logger.Error(nil, "AuthConfig validation error: unknown auth strategy")
	}

	if rolesSource == "api" && rolesApiURL == "" {
		logger.Error(nil, "AuthConfig validation error: 'auth_roles_api_url' is required when roles source is 'api'")
	}

	return middlewaresAuth.AuthConfig{
		Strategy:    strategy,
		Secret:      secret,
		JWKSURL:     jwksUrl,
		Algorithm:   algorithm,
		RolesSource: rolesSource,
		RolesPath:   rolesPath,
		RolesAPIURL: rolesApiURL,
	}
}

// getRedisConfigBase is a helper function that builds a redisModel.Config
// with the provided db and keyPrefix. This avoids code duplication between
// GetRedisConfig and GetSharedRedisConfig.
func getRedisConfigBase(db int, keyPrefix string) redisModel.Config {
	host, _ := GetStringValue("redis_host")
	port, _ := GetIntValue("redis_port")
	username, _ := GetStringValue("redis_username")
	password, _ := GetStringValue("redis_password")

	return redisModel.Config{
		Host:      host,
		Port:      port,
		Username:  username,
		Password:  password,
		DB:        db,
		KeyPrefix: keyPrefix,
	}
}

// GetRedisConfig builds and returns a redisModel.Config for the service's
// private Redis database using the following keys:
//
//   - "service_name" (string): used to compose a namespaced KeyPrefix
//   - "go_env"       (string): environment name (e.g., "dev", "prod")
//   - "redis_host"   (string)
//   - "redis_port"   (int)
//   - "redis_username"(string)
//   - "redis_password"(string)
//   - "redis_db"     (int)
//
// The KeyPrefix is computed as "<go_env>:<service_name>" to avoid key
// collisions across environments/services. Missing or malformed keys result
// in zero-values for the corresponding fields. Panics if the config singleton
// is not initialized.
func GetRedisConfig() redisModel.Config {
	serviceName, _ := GetStringValue("service_name")
	goEnv, _ := GetStringValue("go_env")
	db, _ := GetIntValue("redis_db")

	keyPrefix := goEnv + ":" + serviceName
	return getRedisConfigBase(db, keyPrefix)
}

// GetSharedRedisConfig builds and returns a redisModel.Config for the shared
// Redis database (used for cross-service data like authorization cache).
//
// Uses the following keys:
//   - "go_env"       (string): environment name (e.g., "dev", "prod")
//   - "redis_host"   (string)
//   - "redis_port"   (int)
//   - "redis_username"(string)
//   - "redis_password"(string)
//   - "redis_shared_db" (int): dedicated DB for shared data (default: 5)
//
// The KeyPrefix is computed as "<go_env>:shared" to indicate this cache
// is shared across all services. Missing or malformed keys result in
// zero-values for the corresponding fields. Panics if the config singleton
// is not initialized.
func GetSharedRedisConfig() redisModel.Config {
	goEnv, _ := GetStringValue("go_env")
	db, _ := GetIntValue("redis_shared_db")

	keyPrefix := goEnv + ":shared"
	return getRedisConfigBase(db, keyPrefix)
}

// GetNatsConfig assembles and returns a natsModel.Config using the following keys:
//
//   - "nats_host"       (string): NATS server host and port (e.g., "nats://localhost:4222")
//   - "nats_username"   (string): optional username for authentication
//   - "nats_password"   (string): optional password for authentication
//   - "nats_client_name"(string): client name for NATS server
//
// The function retrieves the values for these keys using the GetStringValue function.
// If any of the required keys are missing, the function will panic.
//
// The function constructs a natsModel.Options object with the retrieved values and default settings.
// If the "nats_username" and "nats_password" keys are not empty, the corresponding fields in the Options object are set.
//
// Finally, the function returns a natsModel.Config object containing the constructed Options object.
func GetNatsConfig() natsModel.Config {
	natsURL, _ := GetStringValue("nats_url")
	natsUsername, _ := GetStringValue("nats_username")
	natsPassword, _ := GetStringValue("nats_password")
	natsClient_name, _ := GetStringValue("nats_client_name")

	options := natsModel.Options{
		Url:          natsURL,
		Name:         natsClient_name,
		MaxReconnect: -1,
		Timeout:      5 * time.Second,
	}

	if natsUsername != "" {
		options.User = natsUsername
	}

	if natsPassword != "" {
		options.Password = natsPassword
	}

	return natsModel.Config{
		Options: options,
	}
}

// GetNatsCoreConfig assembles and returns a natsModel.Config for the NATS Core connection.
// This connection is used for JetStream streams and domain events communication.
//
// Uses the following configuration keys:
//   - "nats_core_url"         (string): NATS Core server URL (e.g., "nats://localhost:4222")
//   - "nats_core_username"    (string): optional username for authentication
//   - "nats_core_password"    (string): optional password for authentication
//   - "nats_core_client_name" (string): client name for NATS server identification
//
// The Core connection typically connects to the main NATS cluster where JetStream
// streams are configured for domain events and microservice communication.
func GetNatsCoreConfig() natsModel.Config {
	natsURL, _ := GetStringValue("nats_core_url")
	natsUsername, _ := GetStringValue("nats_core_username")
	natsPassword, _ := GetStringValue("nats_core_password")
	natsClientName, _ := GetStringValue("nats_core_client_name")

	options := natsModel.Options{
		Url:          natsURL,
		Name:         natsClientName,
		MaxReconnect: -1,
		Timeout:      5 * time.Second,
	}

	if natsUsername != "" {
		options.User = natsUsername
	}

	if natsPassword != "" {
		options.Password = natsPassword
	}

	return natsModel.Config{
		Options: options,
	}
}

// GetNatsLeafConfig assembles and returns a natsModel.Config for the NATS Leaf connection.
// This connection is used for Auth Callout when MQTT devices authenticate.
//
// Uses the following configuration keys:
//   - "nats_leaf_url"         (string): NATS Leaf server URL (e.g., "nats://localhost:4223")
//   - "nats_leaf_username"    (string): username for authentication (usually "auth_service")
//   - "nats_leaf_password"    (string): password for authentication
//   - "nats_leaf_client_name" (string): client name for NATS server identification
//
// The Leaf connection typically connects to a NATS Leaf node that handles MQTT
// client connections. The auth_service user must have permission to subscribe
// to $SYS.REQ.USER.AUTH for Auth Callout to work.
func GetNatsLeafConfig() natsModel.Config {
	natsURL, _ := GetStringValue("nats_leaf_url")
	natsUsername, _ := GetStringValue("nats_leaf_username")
	natsPassword, _ := GetStringValue("nats_leaf_password")
	natsClientName, _ := GetStringValue("nats_leaf_client_name")

	options := natsModel.Options{
		Url:          natsURL,
		Name:         natsClientName,
		MaxReconnect: -1,
		Timeout:      5 * time.Second,
	}

	if natsUsername != "" {
		options.User = natsUsername
	}

	if natsPassword != "" {
		options.Password = natsPassword
	}

	return natsModel.Config{
		Options: options,
	}
}

// GetMyApiKey retrieves the API key from the configuration.
//
// It attempts to fetch the value associated with the "my_api_key" key
// from the configuration. If the key is not set or the value is empty,
// it returns an empty string. The middleware layer is responsible for
// rejecting all requests when no API key is configured.
//
// Returns:
//
//	A string representing the API key, or empty string if not configured.
func GetMyApiKey() string {
	apiKey, _ := GetStringValue("my_api_key")

	if apiKey == "" {
		logger.Warn("[APP:Config] MY_API_KEY is not configured. All API key auth requests will be rejected.")
	}

	return apiKey
}

// GetClickHouseConfig builds and returns a clickhouseModel.Config using the
// following configuration keys:
//
//   - "clickhouse_host" (string): ClickHouse server host
//   - "clickhouse_port" (int): ClickHouse server port (default: 9000 for native protocol)
//   - "clickhouse_database" (string): default database name
//   - "clickhouse_username" (string): optional username for authentication
//   - "clickhouse_password" (string): optional password for authentication
//
// Missing or malformed keys result in zero-values for the corresponding fields.
// This function panics if the config singleton is not initialized.
func GetClickHouseConfig() clickhouseModel.Config {
	host, _ := GetStringValue("clickhouse_host")
	port, _ := GetIntValue("clickhouse_port")
	database, _ := GetStringValue("clickhouse_database")
	username, _ := GetStringValue("clickhouse_username")
	password, _ := GetStringValue("clickhouse_password")

	return clickhouseModel.Config{
		Host:     host,
		Port:     port,
		Database: database,
		Username: username,
		Password: password,
	}
}

// GetEnv returns the runtime environment prefix used to namespace JetStream
// streams, subjects, and consumer durables. It reads the "go_env" config key
// (registered in every service via {Key: "go_env", Env: "GO_ENV", Default: "dev"}).
// If the key is missing, the value is the empty string, or the config
// singleton has not been initialized yet (which happens when stream/subject
// constants are computed at package init in tests that do not call
// InitConfig), GetEnv returns "dev".
//
// Example: GetEnv() returns "dev" by default, "prod" when GO_ENV=prod.
func GetEnv() string {
	if instance == nil {
		return "dev"
	}
	v, err := GetStringValue("go_env")
	if err != nil || v == "" {
		return "dev"
	}
	return v
}

// StreamName builds a canonical JetStream stream name following the pattern
// ${ENV}-MAPEXOS-{SERVICE}-{CONTEXT}. Env, service, and context are uppercased
// independently of the input casing. When context is empty, the trailing dash
// is omitted (returns ${ENV}-MAPEXOS-{SERVICE}).
//
// Example: StreamName("ASSETS", "HEARTBEAT") returns "DEV-MAPEXOS-ASSETS-HEARTBEAT"
// when GO_ENV=dev, "PROD-MAPEXOS-ASSETS-HEARTBEAT" when GO_ENV=prod.
func StreamName(service, context string) string {
	env := strings.ToUpper(GetEnv())
	svc := strings.ToUpper(service)
	if context == "" {
		return env + "-MAPEXOS-" + svc
	}
	return env + "-MAPEXOS-" + svc + "-" + strings.ToUpper(context)
}

// Subject builds a canonical NATS subject following the pattern
// ${env}.mapexos.{service}.{action}. Env, service, and action are lowercased
// independently of the input casing.
//
// Example: Subject("events", "save") returns "dev.mapexos.events.save"
// when GO_ENV=dev, "prod.mapexos.events.save" when GO_ENV=prod.
func Subject(service, action string) string {
	env := strings.ToLower(GetEnv())
	return env + ".mapexos." + strings.ToLower(service) + "." + strings.ToLower(action)
}

// Durable builds a canonical JetStream consumer durable name following the
// pattern ${env}-{service}-{context}-consumer. Env, service, and context are
// lowercased independently of the input casing.
//
// Example: Durable("events", "save") returns "dev-events-save-consumer"
// when GO_ENV=dev, "prod-events-save-consumer" when GO_ENV=prod.
func Durable(service, context string) string {
	env := strings.ToLower(GetEnv())
	return env + "-" + strings.ToLower(service) + "-" + strings.ToLower(context) + "-consumer"
}
