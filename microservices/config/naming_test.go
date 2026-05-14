package configuration

import (
	"testing"
)

// initTestConfig initializes the config singleton with the provided go_env
// value. Tests that need a different env value first call resetConfigSingleton
// (declared in config_test_helper_test.go) and then this function.
func initTestConfig(t *testing.T, goEnv string) {
	t.Helper()
	t.Setenv("GO_ENV", goEnv)
	InitConfig([]ConfigDefinition{
		{Key: "go_env", Env: "GO_ENV", Type: "string", Default: "dev"},
	})
}

func TestGetEnv_DefaultWhenUnset(t *testing.T) {
	resetConfigSingleton()
	t.Setenv("GO_ENV", "")
	InitConfig([]ConfigDefinition{
		{Key: "go_env", Env: "GO_ENV", Type: "string", Default: "dev"},
	})
	got := GetEnv()
	if got != "dev" {
		t.Errorf("expected 'dev' when GO_ENV is empty, got %q", got)
	}
}

func TestGetEnv_ExplicitValue(t *testing.T) {
	resetConfigSingleton()
	initTestConfig(t, "prod")
	got := GetEnv()
	if got != "prod" {
		t.Errorf("expected 'prod', got %q", got)
	}
}

func TestStreamName(t *testing.T) {
	tests := []struct {
		name    string
		goEnv   string
		service string
		context string
		want    string
	}{
		{"default env", "dev", "ASSETS", "HEARTBEAT", "DEV-MAPEXOS-ASSETS-HEARTBEAT"},
		{"prod env", "prod", "ASSETS", "HEARTBEAT", "PROD-MAPEXOS-ASSETS-HEARTBEAT"},
		{"qa env", "qa", "EVENTS", "SAVE", "QA-MAPEXOS-EVENTS-SAVE"},
		{"mixed-case input uppercased", "dev", "assets", "Heartbeat", "DEV-MAPEXOS-ASSETS-HEARTBEAT"},
		{"empty context omits trailing dash", "dev", "DLQ", "", "DEV-MAPEXOS-DLQ"},
		{"multi-token context preserved", "dev", "ASSETS", "HEALTH-MONITOR", "DEV-MAPEXOS-ASSETS-HEALTH-MONITOR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetConfigSingleton()
			initTestConfig(t, tt.goEnv)
			got := StreamName(tt.service, tt.context)
			if got != tt.want {
				t.Errorf("StreamName(%q, %q) with GO_ENV=%q = %q, want %q",
					tt.service, tt.context, tt.goEnv, got, tt.want)
			}
		})
	}
}

func TestSubject(t *testing.T) {
	tests := []struct {
		name    string
		goEnv   string
		service string
		action  string
		want    string
	}{
		{"default env", "dev", "events", "save", "dev.mapexos.events.save"},
		{"prod env", "prod", "events", "save", "prod.mapexos.events.save"},
		{"mixed-case input lowercased", "dev", "Events", "Save", "dev.mapexos.events.save"},
		{"dotted action preserved", "dev", "mapexos", "fanout.asset.invalidate", "dev.mapexos.mapexos.fanout.asset.invalidate"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetConfigSingleton()
			initTestConfig(t, tt.goEnv)
			got := Subject(tt.service, tt.action)
			if got != tt.want {
				t.Errorf("Subject(%q, %q) with GO_ENV=%q = %q, want %q",
					tt.service, tt.action, tt.goEnv, got, tt.want)
			}
		})
	}
}

func TestDurable(t *testing.T) {
	tests := []struct {
		name    string
		goEnv   string
		service string
		context string
		want    string
	}{
		{"default env", "dev", "events", "save", "dev-events-save-consumer"},
		{"prod env", "prod", "events", "save", "prod-events-save-consumer"},
		{"mixed-case input lowercased", "dev", "Events", "Save", "dev-events-save-consumer"},
		{"multi-token context", "dev", "assets", "mqtt-presence", "dev-assets-mqtt-presence-consumer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetConfigSingleton()
			initTestConfig(t, tt.goEnv)
			got := Durable(tt.service, tt.context)
			if got != tt.want {
				t.Errorf("Durable(%q, %q) with GO_ENV=%q = %q, want %q",
					tt.service, tt.context, tt.goEnv, got, tt.want)
			}
		})
	}
}
