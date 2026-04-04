package configuration

import (
	"os"
	"testing"

	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

func TestMain(m *testing.M) {
	logger.InitLogger(logger.LoggerOptions{
		ServiceName: "test",
		Environment: "test",
		Level:       logger.ErrorLevel,
	})
	os.Exit(m.Run())
}

/** getEnvString */

func TestGetEnvString_Set(t *testing.T) {
	t.Setenv("TEST_STRING", "hello")
	result := getEnvString("TEST_STRING", "default")
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestGetEnvString_Unset(t *testing.T) {
	os.Unsetenv("TEST_STRING_UNSET")
	result := getEnvString("TEST_STRING_UNSET", "default")
	if result != "default" {
		t.Errorf("expected 'default', got %q", result)
	}
}

func TestGetEnvString_Empty(t *testing.T) {
	t.Setenv("TEST_STRING_EMPTY", "")
	result := getEnvString("TEST_STRING_EMPTY", "fallback")
	if result != "fallback" {
		t.Errorf("expected 'fallback' for empty env, got %q", result)
	}
}

/** getEnvBool */

func TestGetEnvBool_True(t *testing.T) {
	t.Setenv("TEST_BOOL", "true")
	result := getEnvBool("TEST_BOOL", false)
	if !result {
		t.Error("expected true")
	}
}

func TestGetEnvBool_False(t *testing.T) {
	t.Setenv("TEST_BOOL", "false")
	result := getEnvBool("TEST_BOOL", true)
	if result {
		t.Error("expected false")
	}
}

func TestGetEnvBool_Invalid(t *testing.T) {
	t.Setenv("TEST_BOOL", "notbool")
	result := getEnvBool("TEST_BOOL", true)
	if !result {
		t.Error("expected default (true) for invalid bool")
	}
}

func TestGetEnvBool_Unset(t *testing.T) {
	os.Unsetenv("TEST_BOOL_UNSET")
	result := getEnvBool("TEST_BOOL_UNSET", true)
	if !result {
		t.Error("expected default (true) for unset env")
	}
}

/** getEnvInt */

func TestGetEnvInt_Valid(t *testing.T) {
	t.Setenv("TEST_INT", "42")
	result := getEnvInt("TEST_INT", 0)
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
}

func TestGetEnvInt_Invalid(t *testing.T) {
	t.Setenv("TEST_INT", "abc")
	result := getEnvInt("TEST_INT", 10)
	if result != 10 {
		t.Errorf("expected default 10 for invalid int, got %d", result)
	}
}

func TestGetEnvInt_Unset(t *testing.T) {
	os.Unsetenv("TEST_INT_UNSET")
	result := getEnvInt("TEST_INT_UNSET", 99)
	if result != 99 {
		t.Errorf("expected default 99, got %d", result)
	}
}

func TestGetEnvInt_Negative(t *testing.T) {
	t.Setenv("TEST_INT", "-5")
	result := getEnvInt("TEST_INT", 0)
	if result != -5 {
		t.Errorf("expected -5, got %d", result)
	}
}

/** getEnvArray */

func TestGetEnvArray_CSV(t *testing.T) {
	t.Setenv("TEST_ARRAY", "a,b,c")
	result := getEnvArray("TEST_ARRAY", []string{})
	if len(result) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result))
	}
	if result[0] != "a" || result[1] != "b" || result[2] != "c" {
		t.Errorf("expected [a, b, c], got %v", result)
	}
}

func TestGetEnvArray_Single(t *testing.T) {
	t.Setenv("TEST_ARRAY", "only")
	result := getEnvArray("TEST_ARRAY", []string{})
	if len(result) != 1 || result[0] != "only" {
		t.Errorf("expected ['only'], got %v", result)
	}
}

func TestGetEnvArray_Unset(t *testing.T) {
	os.Unsetenv("TEST_ARRAY_UNSET")
	defaults := []string{"x", "y"}
	result := getEnvArray("TEST_ARRAY_UNSET", defaults)
	if len(result) != 2 || result[0] != "x" {
		t.Errorf("expected default [x, y], got %v", result)
	}
}

/** getEnvJSON */

func TestGetEnvJSON_Valid(t *testing.T) {
	t.Setenv("TEST_JSON", `{"key":"value","nested":{"a":1}}`)
	result := getEnvJSON("TEST_JSON", map[string]interface{}{})
	if result["key"] != "value" {
		t.Errorf("expected 'value', got %v", result["key"])
	}
}

func TestGetEnvJSON_Invalid(t *testing.T) {
	t.Setenv("TEST_JSON", "not json")
	defaults := map[string]interface{}{"default": true}
	result := getEnvJSON("TEST_JSON", defaults)
	if result["default"] != true {
		t.Errorf("expected default map, got %v", result)
	}
}

func TestGetEnvJSON_Unset(t *testing.T) {
	os.Unsetenv("TEST_JSON_UNSET")
	defaults := map[string]interface{}{"d": "val"}
	result := getEnvJSON("TEST_JSON_UNSET", defaults)
	if result["d"] != "val" {
		t.Errorf("expected default map, got %v", result)
	}
}

/** InitConfig + GetStringValue/GetIntValue/GetBoolValue */

func TestInitConfig_GetStringValue(t *testing.T) {
	// Reset singleton for test
	resetConfigSingleton()

	t.Setenv("TEST_SVC_NAME", "my-service")

	InitConfig([]ConfigDefinition{
		{Key: "service_name", Env: "TEST_SVC_NAME", Type: "string", Default: ""},
	})

	val, err := GetStringValue("service_name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "my-service" {
		t.Errorf("expected 'my-service', got %q", val)
	}

	// Reset after test
	resetConfigSingleton()
}

func TestInitConfig_GetIntValue(t *testing.T) {
	resetConfigSingleton()

	t.Setenv("TEST_PORT", "8080")

	InitConfig([]ConfigDefinition{
		{Key: "port", Env: "TEST_PORT", Type: "int", Default: 3000},
	})

	val, err := GetIntValue("port")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 8080 {
		t.Errorf("expected 8080, got %d", val)
	}

	resetConfigSingleton()
}

func TestInitConfig_GetBoolValue(t *testing.T) {
	resetConfigSingleton()

	t.Setenv("TEST_DEBUG", "true")

	InitConfig([]ConfigDefinition{
		{Key: "debug", Env: "TEST_DEBUG", Type: "bool", Default: false},
	})

	val, err := GetBoolValue("debug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !val {
		t.Error("expected true")
	}

	resetConfigSingleton()
}

func TestGetStringValue_MissingKey(t *testing.T) {
	resetConfigSingleton()

	InitConfig([]ConfigDefinition{})

	_, err := GetStringValue("nonexistent")
	if err == nil {
		t.Error("expected error for missing key")
	}

	resetConfigSingleton()
}

func TestGetIntValue_MissingKey(t *testing.T) {
	resetConfigSingleton()

	InitConfig([]ConfigDefinition{})

	_, err := GetIntValue("nonexistent")
	if err == nil {
		t.Error("expected error for missing key")
	}

	resetConfigSingleton()
}

func TestGetBoolValue_MissingKey(t *testing.T) {
	resetConfigSingleton()

	InitConfig([]ConfigDefinition{})

	_, err := GetBoolValue("nonexistent")
	if err == nil {
		t.Error("expected error for missing key")
	}

	resetConfigSingleton()
}

func TestGetStringValue_WrongType(t *testing.T) {
	resetConfigSingleton()

	t.Setenv("TEST_NUM", "42")

	InitConfig([]ConfigDefinition{
		{Key: "num", Env: "TEST_NUM", Type: "int", Default: 0},
	})

	_, err := GetStringValue("num")
	if err == nil {
		t.Error("expected error when getting int as string")
	}

	resetConfigSingleton()
}

func TestGetConfigValue_NotInitialized_Panics(t *testing.T) {
	resetConfigSingleton()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when config not initialized")
		}
	}()

	GetConfigValue("any")
}

/** GetMyApiKey Security Tests */

func TestGetMyApiKey_Configured(t *testing.T) {
	resetConfigSingleton()

	t.Setenv("TEST_MY_API_KEY", "secure-key-abc123")

	InitConfig([]ConfigDefinition{
		{Key: "my_api_key", Env: "TEST_MY_API_KEY", Type: "string", Default: ""},
	})

	key := GetMyApiKey()
	if key != "secure-key-abc123" {
		t.Errorf("expected 'secure-key-abc123', got %q", key)
	}

	resetConfigSingleton()
}

func TestGetMyApiKey_NotConfigured_ReturnsEmpty(t *testing.T) {
	resetConfigSingleton()

	t.Setenv("TEST_MY_API_KEY_EMPTY", "")

	InitConfig([]ConfigDefinition{
		{Key: "my_api_key", Env: "TEST_MY_API_KEY_EMPTY", Type: "string", Default: ""},
	})

	key := GetMyApiKey()
	if key != "" {
		t.Errorf("SECURITY: expected empty string when key not configured, got %q", key)
	}

	resetConfigSingleton()
}

func TestGetMyApiKey_NeverReturnsInsecureKey(t *testing.T) {
	resetConfigSingleton()

	InitConfig([]ConfigDefinition{
		{Key: "my_api_key", Env: "NONEXISTENT_KEY_XYZ", Type: "string", Default: ""},
	})

	key := GetMyApiKey()
	if key == "insecure_key" {
		t.Error("SECURITY: GetMyApiKey must NEVER return 'insecure_key'")
	}

	resetConfigSingleton()
}
