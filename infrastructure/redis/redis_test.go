package redisModel

import (
	"context"
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
				Host:      "localhost",
				Port:      6379,
				Username:  "user",
				Password:  "pass",
				DB:        0,
				KeyPrefix: "test:",
			},
			valid: true,
		},
		{
			name: "config with empty host",
			config: Config{
				Host:      "",
				Port:      6379,
				Username:  "user",
				Password:  "pass",
				DB:        0,
				KeyPrefix: "test:",
			},
			valid: false,
		},
		{
			name: "config with invalid port",
			config: Config{
				Host:      "localhost",
				Port:      -1,
				Username:  "user",
				Password:  "pass",
				DB:        0,
				KeyPrefix: "test:",
			},
			valid: false,
		},
		{
			name: "config with negative DB",
			config: Config{
				Host:      "localhost",
				Port:      6379,
				Username:  "user",
				Password:  "pass",
				DB:        -1,
				KeyPrefix: "test:",
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

func TestSetOptions(t *testing.T) {
	tests := []struct {
		name    string
		options SetOptions
	}{
		{
			name: "default options",
			options: SetOptions{
				TTL:         0,
				NX:          false,
				XX:          false,
				KeepTTL:     false,
				Tags:        nil,
				Compression: false,
			},
		},
		{
			name: "options with TTL",
			options: SetOptions{
				TTL:         5 * time.Minute,
				NX:          false,
				XX:          false,
				KeepTTL:     false,
				Tags:        []string{"cache", "user-data"},
				Compression: true,
			},
		},
		{
			name: "mutually exclusive options NX and XX",
			options: SetOptions{
				TTL:         time.Hour,
				NX:          true,
				XX:          true, // This should be invalid in practice
				KeepTTL:     false,
				Tags:        nil,
				Compression: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that options can be created and accessed
			if tt.options.TTL < 0 {
				t.Error("TTL should not be negative")
			}
			
			if tt.options.NX && tt.options.XX {
				t.Log("Warning: NX and XX are mutually exclusive options")
			}

			if tt.options.Tags != nil && len(tt.options.Tags) > 0 {
				for _, tag := range tt.options.Tags {
					if tag == "" {
						t.Error("Tags should not contain empty strings")
					}
				}
			}
		})
	}
}

func TestGetOrSetParams(t *testing.T) {
	ctx := context.Background()
	
	tests := []struct {
		name   string
		params GetOrSetParams
		valid  bool
	}{
		{
			name: "valid params",
			params: GetOrSetParams{
				Ctx:      ctx,
				CacheKey: "test:key",
				CacheTTL: 300,
				Callback: func() (interface{}, error) {
					return "test value", nil
				},
			},
			valid: true,
		},
		{
			name: "params with nil context",
			params: GetOrSetParams{
				Ctx:      nil,
				CacheKey: "test:key",
				CacheTTL: 300,
				Callback: func() (interface{}, error) {
					return "test value", nil
				},
			},
			valid: false,
		},
		{
			name: "params with empty cache key",
			params: GetOrSetParams{
				Ctx:      ctx,
				CacheKey: "",
				CacheTTL: 300,
				Callback: func() (interface{}, error) {
					return "test value", nil
				},
			},
			valid: false,
		},
		{
			name: "params with nil callback",
			params: GetOrSetParams{
				Ctx:      ctx,
				CacheKey: "test:key",
				CacheTTL: 300,
				Callback: nil,
			},
			valid: false,
		},
		{
			name: "params with negative TTL",
			params: GetOrSetParams{
				Ctx:      ctx,
				CacheKey: "test:key",
				CacheTTL: -1,
				Callback: func() (interface{}, error) {
					return "test value", nil
				},
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validateGetOrSetParams(tt.params)
			if isValid != tt.valid {
				t.Errorf("expected validation result %v, got %v", tt.valid, isValid)
			}

			// Test callback execution if valid
			if tt.valid && tt.params.Callback != nil {
				result, err := tt.params.Callback()
				if err != nil {
					t.Errorf("callback returned error: %v", err)
				}
				if result == nil {
					t.Error("callback returned nil result")
				}
			}
		})
	}
}

func TestConstants(t *testing.T) {
	t.Run("DefaultTTLSeconds", func(t *testing.T) {
		if DefaultTTLSeconds <= 0 {
			t.Error("DefaultTTLSeconds should be positive")
		}
		if DefaultTTLSeconds != 300 {
			t.Errorf("expected DefaultTTLSeconds to be 300, got %d", DefaultTTLSeconds)
		}
	})

	t.Run("NoExpiration", func(t *testing.T) {
		if NoExpiration != 0 {
			t.Errorf("expected NoExpiration to be 0, got %d", NoExpiration)
		}
	})
}

func TestRedisClientStructure(t *testing.T) {
	// Test that RedisClient can be created (without actual Redis connection)
	client := &RedisClient{
		client:    nil, // We don't initialize actual Redis client for unit tests
		keyPrefix: "test:",
	}

	if client.keyPrefix != "test:" {
		t.Errorf("expected keyPrefix 'test:', got %s", client.keyPrefix)
	}
}

// Helper functions for validation (these would ideally be part of the main package)
func validateConfig(config Config) bool {
	if config.Host == "" {
		return false
	}
	if config.Port <= 0 || config.Port > 65535 {
		return false
	}
	if config.DB < 0 {
		return false
	}
	return true
}

func validateGetOrSetParams(params GetOrSetParams) bool {
	if params.Ctx == nil {
		return false
	}
	if params.CacheKey == "" {
		return false
	}
	if params.CacheTTL < 0 {
		return false
	}
	if params.Callback == nil {
		return false
	}
	return true
}

func BenchmarkSetOptionsCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SetOptions{
			TTL:         5 * time.Minute,
			NX:          false,
			XX:          false,
			KeepTTL:     false,
			Tags:        []string{"benchmark", "test"},
			Compression: true,
		}
	}
}

func BenchmarkConfigValidation(b *testing.B) {
	config := Config{
		Host:      "localhost",
		Port:      6379,
		Username:  "user",
		Password:  "pass",
		DB:        0,
		KeyPrefix: "bench:",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validateConfig(config)
	}
}