package mongoManager

import (
	"strings"
	"testing"
	"time"
)

func TestConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		valid  bool
	}{
		{
			name: "valid config",
			config: Config{
				URI:             "mongodb://localhost:27017",
				Database:        "testdb",
				EnableMonitor:   true,
				MonitorInterval: 10 * time.Second,
			},
			valid: true,
		},
		{
			name: "config with empty URI",
			config: Config{
				URI:             "",
				Database:        "testdb",
				EnableMonitor:   false,
				MonitorInterval: 10 * time.Second,
			},
			valid: false,
		},
		{
			name: "config with empty database name",
			config: Config{
				URI:             "mongodb://localhost:27017",
				Database:        "",
				EnableMonitor:   false,
				MonitorInterval: 10 * time.Second,
			},
			valid: false,
		},
		{
			name: "config with invalid URI format",
			config: Config{
				URI:             "invalid-uri",
				Database:        "testdb",
				EnableMonitor:   false,
				MonitorInterval: 10 * time.Second,
			},
			valid: false,
		},
		{
			name: "config with monitor enabled and zero interval",
			config: Config{
				URI:             "mongodb://localhost:27017",
				Database:        "testdb",
				EnableMonitor:   true,
				MonitorInterval: 0,
			},
			valid: false,
		},
		{
			name: "config with monitor disabled and zero interval",
			config: Config{
				URI:             "mongodb://localhost:27017",
				Database:        "testdb",
				EnableMonitor:   false,
				MonitorInterval: 0,
			},
			valid: true,
		},
		{
			name: "config with negative monitor interval",
			config: Config{
				URI:             "mongodb://localhost:27017",
				Database:        "testdb",
				EnableMonitor:   true,
				MonitorInterval: -5 * time.Second,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validateConfig(tt.config)
			if isValid != tt.valid {
				t.Errorf("expected validation result %v, got %v", tt.valid, isValid)
			}
		})
	}
}

func TestMongoManagerStructure(t *testing.T) {
	// Test that MongoManager can be created (without actual MongoDB connection)
	manager := &MongoManager{
		client:     nil, // We don't initialize actual MongoDB client for unit tests
		dbInstance: nil,
		dbName:     "testdb",
	}

	manager.isConnected.Store(false)

	if manager.dbName != "testdb" {
		t.Errorf("expected dbName 'testdb', got %s", manager.dbName)
	}

	if manager.isConnected.Load() {
		t.Error("expected isConnected to be false initially")
	}

	// Test setting connected state
	manager.isConnected.Store(true)
	if !manager.isConnected.Load() {
		t.Error("expected isConnected to be true after setting")
	}
}

func TestConfigURIValidation(t *testing.T) {
	validURIs := []string{
		"mongodb://localhost:27017",
		"mongodb://user:pass@localhost:27017",
		"mongodb://localhost:27017,localhost:27018",
		"mongodb+srv://cluster.mongodb.net",
		"mongodb://user:pass@cluster1.mongodb.net:27017,cluster2.mongodb.net:27017/mydb?replicaSet=myReplicaSet",
	}

	invalidURIs := []string{
		"",
		"localhost:27017",
		"http://localhost:27017",
		"mongodb://",
		"invalid-uri",
		"ftp://localhost:27017",
	}

	for _, uri := range validURIs {
		t.Run("valid_URI_"+uri, func(t *testing.T) {
			if !isValidMongoURI(uri) {
				t.Errorf("expected URI %s to be valid", uri)
			}
		})
	}

	for _, uri := range invalidURIs {
		t.Run("invalid_URI_"+uri, func(t *testing.T) {
			if isValidMongoURI(uri) {
				t.Errorf("expected URI %s to be invalid", uri)
			}
		})
	}
}

func TestConfigDatabaseNameValidation(t *testing.T) {
	validNames := []string{
		"testdb",
		"my_database",
		"db123",
		"valid-name",
		"Valid_Database_Name123",
	}

	invalidNames := []string{
		"",
		" ",
		"db with spaces",
		"db/with/slashes",
		"db\\with\\backslashes",
		"db\"with\"quotes",
		"db*with*asterisks",
		"db<with>brackets",
		"db|with|pipes",
		"db:with:colons",
	}

	for _, name := range validNames {
		t.Run("valid_name_"+name, func(t *testing.T) {
			if !isValidDatabaseName(name) {
				t.Errorf("expected database name %s to be valid", name)
			}
		})
	}

	for _, name := range invalidNames {
		t.Run("invalid_name_"+name, func(t *testing.T) {
			if isValidDatabaseName(name) {
				t.Errorf("expected database name %s to be invalid", name)
			}
		})
	}
}

func TestDefaultMonitorInterval(t *testing.T) {
	if DefaultMonitorInterval <= 0 {
		t.Error("DefaultMonitorInterval should be positive")
	}

	if DefaultMonitorInterval != 10 {
		t.Errorf("expected DefaultMonitorInterval to be 10, got %d", DefaultMonitorInterval)
	}
}

func TestMonitorIntervalValidation(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		enabled  bool
		valid    bool
	}{
		{
			name:     "positive interval with monitor enabled",
			interval: 5 * time.Second,
			enabled:  true,
			valid:    true,
		},
		{
			name:     "zero interval with monitor disabled",
			interval: 0,
			enabled:  false,
			valid:    true,
		},
		{
			name:     "zero interval with monitor enabled",
			interval: 0,
			enabled:  true,
			valid:    false,
		},
		{
			name:     "negative interval",
			interval: -5 * time.Second,
			enabled:  true,
			valid:    false,
		},
		{
			name:     "very small positive interval",
			interval: 1 * time.Millisecond,
			enabled:  true,
			valid:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validateMonitorInterval(tt.interval, tt.enabled)
			if isValid != tt.valid {
				t.Errorf("expected validation result %v, got %v", tt.valid, isValid)
			}
		})
	}
}

// Helper functions for validation (these would ideally be part of the main package)
func validateConfig(config Config) bool {
	if !isValidMongoURI(config.URI) {
		return false
	}
	if !isValidDatabaseName(config.Database) {
		return false
	}
	if !validateMonitorInterval(config.MonitorInterval, config.EnableMonitor) {
		return false
	}
	return true
}

func isValidMongoURI(uri string) bool {
	if uri == "" {
		return false
	}
	
	// Basic validation - should start with mongodb:// or mongodb+srv://
	if strings.HasPrefix(uri, "mongodb://") {
		// Make sure there's something after mongodb://
		if len(uri) <= len("mongodb://") {
			return false
		}
		return true
	}
	
	if strings.HasPrefix(uri, "mongodb+srv://") {
		// Make sure there's something after mongodb+srv://
		if len(uri) <= len("mongodb+srv://") {
			return false
		}
		return true
	}
	
	return false
}

func isValidDatabaseName(name string) bool {
	if name == "" {
		return false
	}
	
	// MongoDB database name restrictions
	invalidChars := []string{" ", "/", "\\", ".", "\"", "*", "<", ">", ":", "|", "?"}
	for _, char := range invalidChars {
		if strings.Contains(name, char) {
			return false
		}
	}
	
	return true
}

func validateMonitorInterval(interval time.Duration, enabled bool) bool {
	if !enabled {
		return true // If monitoring is disabled, interval doesn't matter
	}
	
	return interval > 0 // If monitoring is enabled, interval must be positive
}

func BenchmarkConfigValidation(b *testing.B) {
	config := Config{
		URI:             "mongodb://localhost:27017",
		Database:        "benchdb",
		EnableMonitor:   true,
		MonitorInterval: 10 * time.Second,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validateConfig(config)
	}
}

func BenchmarkMongoManagerCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager := &MongoManager{
			client:     nil,
			dbInstance: nil,
			dbName:     "benchdb",
		}
		manager.isConnected.Store(false)
	}
}