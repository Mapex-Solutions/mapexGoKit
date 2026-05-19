package configuration

import (
	"fmt"
	"strings"
	"testing"
)

// --- FindSensitiveDefaultsInUse (pure, env-agnostic) ---

func TestFind_ReturnsViolationsForSensitiveDefaults(t *testing.T) {
	defs := []ConfigDefinition{
		{Key: "auth_secret", Env: "AUTH_SECRET", Type: "string", Default: "dev-secret", Sensitive: true},
		{Key: "nats_password", Env: "NATS_PASSWORD", Type: "string", Default: "service_secret", Sensitive: true},
	}
	current := map[string]interface{}{
		"auth_secret":   "dev-secret",
		"nats_password": "service_secret",
	}

	violations := FindSensitiveDefaultsInUse(defs, current)
	if len(violations) != 2 {
		t.Fatalf("expected 2 violations, got %d: %v", len(violations), violations)
	}

	got := map[string]string{}
	for _, v := range violations {
		got[v.Key] = v.Env
	}
	if got["auth_secret"] != "AUTH_SECRET" {
		t.Errorf("expected AUTH_SECRET env for auth_secret, got %q", got["auth_secret"])
	}
	if got["nats_password"] != "NATS_PASSWORD" {
		t.Errorf("expected NATS_PASSWORD env for nats_password, got %q", got["nats_password"])
	}
}

func TestFind_IgnoresNonSensitive(t *testing.T) {
	defs := []ConfigDefinition{
		{Key: "http_port", Env: "HTTP_PORT", Type: "int", Default: 5000, Sensitive: false},
		{Key: "service_name", Env: "SERVICE_NAME", Type: "string", Default: "assets", Sensitive: false},
	}
	current := map[string]interface{}{
		"http_port":    5000,
		"service_name": "assets",
	}

	if v := FindSensitiveDefaultsInUse(defs, current); v != nil {
		t.Errorf("non-sensitive keys must be ignored, got %v", v)
	}
}

func TestFind_IgnoresOverriddenValues(t *testing.T) {
	defs := []ConfigDefinition{
		{Key: "auth_secret", Env: "AUTH_SECRET", Type: "string", Default: "dev-secret", Sensitive: true},
	}
	current := map[string]interface{}{"auth_secret": "prod-real-secret-9f4a"}

	if v := FindSensitiveDefaultsInUse(defs, current); v != nil {
		t.Errorf("overridden sensitive keys must not be flagged, got %v", v)
	}
}

func TestFind_HandlesAllTypes(t *testing.T) {
	defs := []ConfigDefinition{
		{Key: "str_key", Env: "STR_KEY", Type: "string", Default: "default-str", Sensitive: true},
		{Key: "int_key", Env: "INT_KEY", Type: "int", Default: 42, Sensitive: true},
		{Key: "bool_key", Env: "BOOL_KEY", Type: "bool", Default: true, Sensitive: true},
		{Key: "arr_key", Env: "ARR_KEY", Type: "array", Default: []string{"a", "b"}, Sensitive: true},
		{Key: "json_key", Env: "JSON_KEY", Type: "json", Default: map[string]interface{}{"k": "v"}, Sensitive: true},
	}
	current := map[string]interface{}{
		"str_key":  "default-str",
		"int_key":  42,
		"bool_key": true,
		"arr_key":  []string{"a", "b"},
		"json_key": map[string]interface{}{"k": "v"},
	}

	if v := FindSensitiveDefaultsInUse(defs, current); len(v) != 5 {
		t.Fatalf("expected 5 violations across all types, got %d: %v", len(v), v)
	}
}

// --- IsDevEnv ---

func TestIsDevEnv(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"dev", true},
		{"development", true},
		{"DEV", false},                     // case-sensitive on purpose
		{"local", false},                   // not an alias — explicit decision
		{"prod", false},
		{"production", false},
		{"staging", false},
		{"qa", false},
		{"test", false},
		{"develpoment", false},             // typo defaults to fatal
		{"dev ", false},                    // trailing space is suspicious
	}
	for _, c := range cases {
		if got := IsDevEnv(c.in); got != c.want {
			t.Errorf("IsDevEnv(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// --- paintRed ---

func TestPaintRed_AddsAnsiCodes(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	got := paintRed("hi")
	if !strings.Contains(got, "\033[1;31m") || !strings.Contains(got, "\033[0m") {
		t.Errorf("expected ANSI bold-red wrapping, got %q", got)
	}
	if !strings.Contains(got, "hi") {
		t.Errorf("text must be preserved, got %q", got)
	}
}

func TestPaintRed_HonorsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := paintRed("hi")
	if got != "hi" {
		t.Errorf("NO_COLOR set: expected plain text, got %q", got)
	}
}

// --- InitConfig integration: dev warns, non-dev fatals ---

func TestInitConfig_DevWithDefaults_Warns_NotFatal(t *testing.T) {
	resetConfigSingleton()
	t.Cleanup(resetConfigSingleton)

	var warnMsg string
	fatalCalled := false
	origWarn, origFatal := logWarnf, logFatalf
	logWarnf = func(format string, args ...interface{}) {
		warnMsg = fmt.Sprintf(format, args...)
	}
	logFatalf = func(format string, args ...interface{}) {
		fatalCalled = true
	}
	t.Cleanup(func() { logWarnf, logFatalf = origWarn, origFatal })

	t.Setenv("FAKE_GO_ENV_DEV", "dev")

	InitConfig([]ConfigDefinition{
		{Key: "go_env", Env: "FAKE_GO_ENV_DEV", Type: "string", Default: "dev"},
		{Key: "auth_secret", Env: "FAKE_AUTH_SECRET_UNSET_DEV", Type: "string", Default: "dev-secret", Sensitive: true},
	})

	if fatalCalled {
		t.Error("logFatalf must NOT be invoked in dev")
	}
	if !strings.Contains(warnMsg, "SECURITY WARNING") {
		t.Errorf("warn must include SECURITY WARNING tag, got: %s", warnMsg)
	}
	if !strings.Contains(warnMsg, "FAKE_AUTH_SECRET_UNSET_DEV") {
		t.Errorf("warn must list the violating env var, got: %s", warnMsg)
	}
	if instance == nil {
		t.Error("instance must be created in dev — warn is non-fatal")
	}
}

func TestInitConfig_DevelopmentAlias_Warns(t *testing.T) {
	resetConfigSingleton()
	t.Cleanup(resetConfigSingleton)

	var warnCalled, fatalCalled bool
	origWarn, origFatal := logWarnf, logFatalf
	logWarnf = func(format string, args ...interface{}) { warnCalled = true }
	logFatalf = func(format string, args ...interface{}) { fatalCalled = true }
	t.Cleanup(func() { logWarnf, logFatalf = origWarn, origFatal })

	t.Setenv("FAKE_GO_ENV_DEVELOPMENT", "development")

	InitConfig([]ConfigDefinition{
		{Key: "go_env", Env: "FAKE_GO_ENV_DEVELOPMENT", Type: "string", Default: "dev"},
		{Key: "auth_secret", Env: "FAKE_AUTH_SECRET_DEVELOPMENT", Type: "string", Default: "dev-secret", Sensitive: true},
	})

	if fatalCalled {
		t.Error("development alias must NOT fatal")
	}
	if !warnCalled {
		t.Error("development alias must trigger warn")
	}
}

func TestInitConfig_EmptyEnvBehavesLikeDev(t *testing.T) {
	resetConfigSingleton()
	t.Cleanup(resetConfigSingleton)

	var warnCalled, fatalCalled bool
	origWarn, origFatal := logWarnf, logFatalf
	logWarnf = func(format string, args ...interface{}) { warnCalled = true }
	logFatalf = func(format string, args ...interface{}) { fatalCalled = true }
	t.Cleanup(func() { logWarnf, logFatalf = origWarn, origFatal })

	// No go_env definition at all → config["go_env"] is nil → goEnv string is ""

	InitConfig([]ConfigDefinition{
		{Key: "auth_secret", Env: "FAKE_AUTH_SECRET_EMPTYENV", Type: "string", Default: "dev-secret", Sensitive: true},
	})

	if fatalCalled {
		t.Error("empty GO_ENV must NOT fatal (acts as dev)")
	}
	if !warnCalled {
		t.Error("empty GO_ENV must warn (still sensitive defaults present)")
	}
}

func TestInitConfig_FatalsOnProdDefaults(t *testing.T) {
	resetConfigSingleton()
	t.Cleanup(resetConfigSingleton)

	var fatalMsg string
	warnCalled := false
	origWarn, origFatal := logWarnf, logFatalf
	logWarnf = func(format string, args ...interface{}) { warnCalled = true }
	logFatalf = func(format string, args ...interface{}) {
		fatalMsg = fmt.Sprintf(format, args...)
	}
	t.Cleanup(func() { logWarnf, logFatalf = origWarn, origFatal })

	t.Setenv("FAKE_GO_ENV_PROD", "prod")

	InitConfig([]ConfigDefinition{
		{Key: "go_env", Env: "FAKE_GO_ENV_PROD", Type: "string", Default: "dev"},
		{Key: "auth_secret", Env: "FAKE_AUTH_SECRET_UNSET_PROD", Type: "string", Default: "dev-secret", Sensitive: true},
	})

	if warnCalled {
		t.Error("logWarnf must NOT be invoked in prod")
	}
	if fatalMsg == "" {
		t.Fatal("logFatalf must be invoked in prod with sensitive default")
	}
	if !strings.Contains(fatalMsg, "[SECURITY]") {
		t.Errorf("fatal must include [SECURITY] tag, got: %s", fatalMsg)
	}
	if !strings.Contains(fatalMsg, "FAKE_AUTH_SECRET_UNSET_PROD") {
		t.Errorf("fatal must name violating env var, got: %s", fatalMsg)
	}
	if instance != nil {
		t.Error("instance must remain nil after fatal — guard runs before assignment")
	}
}

func TestInitConfig_TypoEnvFatals(t *testing.T) {
	resetConfigSingleton()
	t.Cleanup(resetConfigSingleton)

	fatalCalled := false
	origWarn, origFatal := logWarnf, logFatalf
	logWarnf = func(format string, args ...interface{}) {}
	logFatalf = func(format string, args ...interface{}) { fatalCalled = true }
	t.Cleanup(func() { logWarnf, logFatalf = origWarn, origFatal })

	t.Setenv("FAKE_GO_ENV_TYPO", "develpoment") // intentional typo

	InitConfig([]ConfigDefinition{
		{Key: "go_env", Env: "FAKE_GO_ENV_TYPO", Type: "string", Default: "dev"},
		{Key: "auth_secret", Env: "FAKE_AUTH_SECRET_TYPO", Type: "string", Default: "dev-secret", Sensitive: true},
	})

	if !fatalCalled {
		t.Error("typo in GO_ENV must fatal — fail-closed posture")
	}
}

func TestInitConfig_ProdSucceedsWithOverrides(t *testing.T) {
	resetConfigSingleton()
	t.Cleanup(resetConfigSingleton)

	fatalCalled, warnCalled := false, false
	origWarn, origFatal := logWarnf, logFatalf
	logWarnf = func(format string, args ...interface{}) { warnCalled = true }
	logFatalf = func(format string, args ...interface{}) { fatalCalled = true }
	t.Cleanup(func() { logWarnf, logFatalf = origWarn, origFatal })

	t.Setenv("FAKE_GO_ENV_OK", "prod")
	t.Setenv("FAKE_AUTH_SECRET_OK", "prod-real-9f4a")

	InitConfig([]ConfigDefinition{
		{Key: "go_env", Env: "FAKE_GO_ENV_OK", Type: "string", Default: "dev"},
		{Key: "auth_secret", Env: "FAKE_AUTH_SECRET_OK", Type: "string", Default: "dev-secret", Sensitive: true},
	})

	if fatalCalled || warnCalled {
		t.Error("prod with overrides must not warn or fatal")
	}
	if instance == nil {
		t.Fatal("instance must be created when validation passes")
	}
	if val, _ := GetStringValue("auth_secret"); val != "prod-real-9f4a" {
		t.Errorf("expected overridden value, got %q", val)
	}
}
