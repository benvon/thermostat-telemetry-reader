package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestViperEnvVarBinding(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		envVars  map[string]string
		validate func(*testing.T, *Config)
	}{
		{
			name: "environment variable override",
			config: `
ttr:
  log_level: "info"
  poll_interval: "2m"
  backfill_window: "24h"
providers:
  - name: "ecobee"
    enabled: true
    settings:
      client_id: "default-client-id"
sinks:
  - name: "elasticsearch"
    enabled: true
    settings:
      url: "http://localhost:9200"
`,
			envVars: map[string]string{
				"TTR_LOG_LEVEL":                  "debug",
				"PROVIDERS_0_SETTINGS_CLIENT_ID": "env-client-id",
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.TTR.LogLevel != "debug" {
					t.Errorf("Expected log_level to be overridden by env var, got %s", cfg.TTR.LogLevel)
				}
				if cfg.Providers[0].Settings["client_id"] != "env-client-id" {
					t.Errorf("Expected client_id to be overridden by env var, got %v", cfg.Providers[0].Settings["client_id"])
				}
			},
		},
		{
			name: "multiple providers and sinks with env overrides",
			config: `
ttr:
  log_level: "info"
  poll_interval: "5m"
  backfill_window: "168h"
providers:
  - name: "ecobee"
    enabled: true
    settings:
      client_id: "ecobee-default"
      refresh_token: "ecobee-token-default"
  - name: "nest"
    enabled: true
    settings:
      client_id: "nest-default"
      api_key: "nest-key-default"
sinks:
  - name: "elasticsearch"
    enabled: true
    settings:
      url: "http://localhost:9200"
      api_key: "es-default"
  - name: "prometheus"
    enabled: true
    settings:
      url: "http://localhost:9090"
`,
			envVars: map[string]string{
				"PROVIDERS_0_SETTINGS_CLIENT_ID":     "ecobee-env",
				"PROVIDERS_0_SETTINGS_REFRESH_TOKEN": "ecobee-token-env",
				"PROVIDERS_1_SETTINGS_CLIENT_ID":     "nest-env",
				"PROVIDERS_1_SETTINGS_API_KEY":       "nest-key-env",
				"SINKS_0_SETTINGS_API_KEY":           "es-env",
				"SINKS_1_SETTINGS_URL":               "http://prom-env:9090",
			},
			validate: func(t *testing.T, cfg *Config) {
				// Verify first provider overrides
				if cfg.Providers[0].Settings["client_id"] != "ecobee-env" {
					t.Errorf("Expected providers[0].client_id to be overridden, got %v", cfg.Providers[0].Settings["client_id"])
				}
				if cfg.Providers[0].Settings["refresh_token"] != "ecobee-token-env" {
					t.Errorf("Expected providers[0].refresh_token to be overridden, got %v", cfg.Providers[0].Settings["refresh_token"])
				}

				// Verify second provider overrides
				if cfg.Providers[1].Settings["client_id"] != "nest-env" {
					t.Errorf("Expected providers[1].client_id to be overridden, got %v", cfg.Providers[1].Settings["client_id"])
				}
				if cfg.Providers[1].Settings["api_key"] != "nest-key-env" {
					t.Errorf("Expected providers[1].api_key to be overridden, got %v", cfg.Providers[1].Settings["api_key"])
				}

				// Verify first sink overrides
				if cfg.Sinks[0].Settings["api_key"] != "es-env" {
					t.Errorf("Expected sinks[0].api_key to be overridden, got %v", cfg.Sinks[0].Settings["api_key"])
				}

				// Verify second sink overrides
				if cfg.Sinks[1].Settings["url"] != "http://prom-env:9090" {
					t.Errorf("Expected sinks[1].url to be overridden, got %v", cfg.Sinks[1].Settings["url"])
				}
			},
		},
		{
			name: "default values from Viper",
			config: `
ttr:
  poll_interval: "10m"
  backfill_window: "48h"
providers:
  - name: "ecobee"
    enabled: true
    settings:
      client_id: "test-client-id"
sinks:
  - name: "elasticsearch"
    enabled: true
    settings:
      url: "http://localhost:9200"
`,
			envVars: map[string]string{},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.TTR.Timezone != "UTC" {
					t.Errorf("Expected default timezone UTC, got %s", cfg.TTR.Timezone)
				}
				if cfg.TTR.LogLevel != "info" {
					t.Errorf("Expected default log_level info, got %s", cfg.TTR.LogLevel)
				}
				if cfg.TTR.HealthPort != 8080 {
					t.Errorf("Expected default health_port 8080, got %d", cfg.TTR.HealthPort)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			// Create temporary config file
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "test-config.yaml")
			t.Setenv("TTR_CONFIG_ROOT", tempDir)
			if err := os.WriteFile(configPath, []byte(tt.config), 0644); err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			// Load configuration
			config, err := LoadConfig(configPath)
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			// Run validation
			tt.validate(t, config)
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")
	t.Setenv("TTR_CONFIG_ROOT", tempDir)

	configContent := `
ttr:
  timezone: "America/New_York"
  poll_interval: "10m"
  backfill_window: "48h"
  log_level: "debug"

providers:
  - name: "ecobee"
    enabled: true
    settings:
      client_id: "test-client-id"
      refresh_token: "test-refresh-token"

sinks:
  - name: "elasticsearch"
    enabled: true
    settings:
      url: "http://localhost:9200"
      api_key: "test-api-key"
      index_prefix: "test"
      create_templates: false
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load configuration
	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify TTR config
	if config.TTR.Timezone != "America/New_York" {
		t.Errorf("Expected timezone America/New_York, got %s", config.TTR.Timezone)
	}

	if config.TTR.PollInterval != 10*time.Minute {
		t.Errorf("Expected poll interval 10m, got %v", config.TTR.PollInterval)
	}

	if config.TTR.BackfillWindow != 48*time.Hour {
		t.Errorf("Expected backfill window 48h, got %v", config.TTR.BackfillWindow)
	}

	if config.TTR.LogLevel != "debug" {
		t.Errorf("Expected log level debug, got %s", config.TTR.LogLevel)
	}

	// Verify providers
	if len(config.Providers) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(config.Providers))
	}

	provider := config.Providers[0]
	if provider.Name != "ecobee" {
		t.Errorf("Expected provider name ecobee, got %s", provider.Name)
	}

	if !provider.Enabled {
		t.Error("Expected provider to be enabled")
	}

	// Verify sinks
	if len(config.Sinks) != 1 {
		t.Errorf("Expected 1 sink, got %d", len(config.Sinks))
	}

	sink := config.Sinks[0]
	if sink.Name != "elasticsearch" {
		t.Errorf("Expected sink name elasticsearch, got %s", sink.Name)
	}

	if !sink.Enabled {
		t.Error("Expected sink to be enabled")
	}
}

func TestLoadConfigWithDefaults(t *testing.T) {
	// Create a minimal config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "minimal-config.yaml")
	t.Setenv("TTR_CONFIG_ROOT", tempDir)

	configContent := `
providers:
  - name: "ecobee"
    enabled: true
    settings:
      client_id: "test-client-id"
      refresh_token: "test-refresh-token"

sinks:
  - name: "elasticsearch"
    enabled: true
    settings:
      url: "http://localhost:9200"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load configuration
	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify defaults are set
	if config.TTR.Timezone != "UTC" {
		t.Errorf("Expected default timezone UTC, got %s", config.TTR.Timezone)
	}

	if config.TTR.PollInterval != 5*time.Minute {
		t.Errorf("Expected default poll interval 5m, got %v", config.TTR.PollInterval)
	}

	if config.TTR.BackfillWindow != 168*time.Hour {
		t.Errorf("Expected default backfill window 168h, got %v", config.TTR.BackfillWindow)
	}

	if config.TTR.LogLevel != "info" {
		t.Errorf("Expected default log level info, got %s", config.TTR.LogLevel)
	}

	if config.TTR.HealthPort != 8080 {
		t.Errorf("Expected default health port 8080, got %d", config.TTR.HealthPort)
	}

	if config.TTR.MetricsPort != 9090 {
		t.Errorf("Expected default metrics port 9090, got %d", config.TTR.MetricsPort)
	}
}

func TestLoadConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: `
providers:
  - name: "ecobee"
    enabled: true
    settings:
      client_id: "test"
      refresh_token: "test"

sinks:
  - name: "elasticsearch"
    enabled: true
    settings:
      url: "http://localhost:9200"
`,
			expectError: false,
		},
		{
			name: "no providers",
			config: `
providers: []

sinks:
  - name: "elasticsearch"
    enabled: true
    settings:
      url: "http://localhost:9200"
`,
			expectError: true,
			errorMsg:    "at least one provider must be enabled",
		},
		{
			name: "no sinks",
			config: `
providers:
  - name: "ecobee"
    enabled: true
    settings:
      client_id: "test"
      refresh_token: "test"

sinks: []
`,
			expectError: true,
			errorMsg:    "at least one sink must be enabled",
		},
		{
			name: "invalid log level",
			config: `
ttr:
  log_level: "invalid"

providers:
  - name: "ecobee"
    enabled: true
    settings:
      client_id: "test"
      refresh_token: "test"

sinks:
  - name: "elasticsearch"
    enabled: true
    settings:
      url: "http://localhost:9200"
`,
			expectError: true,
			errorMsg:    "invalid log_level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "test-config.yaml")
			t.Setenv("TTR_CONFIG_ROOT", tempDir)

			if err := os.WriteFile(configPath, []byte(tt.config), 0644); err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			_, err := LoadConfig(configPath)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGetProviderConfig(t *testing.T) {
	config := &Config{
		Providers: []ProviderConfig{
			{Name: "ecobee", Enabled: true},
			{Name: "nest", Enabled: false},
		},
	}

	// Test existing provider
	provider, err := config.GetProviderConfig("ecobee")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if provider.Name != "ecobee" {
		t.Errorf("Expected provider name ecobee, got %s", provider.Name)
	}

	// Test non-existing provider
	_, err = config.GetProviderConfig("unknown")
	if err == nil {
		t.Error("Expected error for unknown provider")
	}
}

func TestGetSinkConfig(t *testing.T) {
	config := &Config{
		Sinks: []SinkConfig{
			{Name: "elasticsearch", Enabled: true},
			{Name: "mongodb", Enabled: false},
		},
	}

	// Test existing sink
	sink, err := config.GetSinkConfig("elasticsearch")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if sink.Name != "elasticsearch" {
		t.Errorf("Expected sink name elasticsearch, got %s", sink.Name)
	}

	// Test non-existing sink
	_, err = config.GetSinkConfig("unknown")
	if err == nil {
		t.Error("Expected error for unknown sink")
	}
}

func TestGetEnabledProviders(t *testing.T) {
	config := &Config{
		Providers: []ProviderConfig{
			{Name: "ecobee", Enabled: true},
			{Name: "nest", Enabled: false},
			{Name: "honeywell", Enabled: true},
		},
	}

	enabled := config.GetEnabledProviders()
	if len(enabled) != 2 {
		t.Errorf("Expected 2 enabled providers, got %d", len(enabled))
	}

	names := make(map[string]bool)
	for _, provider := range enabled {
		names[provider.Name] = true
	}

	if !names["ecobee"] {
		t.Error("Expected ecobee to be enabled")
	}
	if !names["honeywell"] {
		t.Error("Expected honeywell to be enabled")
	}
	if names["nest"] {
		t.Error("Expected nest to be disabled")
	}
}

func TestGetEnabledSinks(t *testing.T) {
	config := &Config{
		Sinks: []SinkConfig{
			{Name: "elasticsearch", Enabled: true},
			{Name: "mongodb", Enabled: false},
			{Name: "s3", Enabled: true},
		},
	}

	enabled := config.GetEnabledSinks()
	if len(enabled) != 2 {
		t.Errorf("Expected 2 enabled sinks, got %d", len(enabled))
	}

	names := make(map[string]bool)
	for _, sink := range enabled {
		names[sink.Name] = true
	}

	if !names["elasticsearch"] {
		t.Error("Expected elasticsearch to be enabled")
	}
	if !names["s3"] {
		t.Error("Expected s3 to be enabled")
	}
	if names["mongodb"] {
		t.Error("Expected mongodb to be disabled")
	}
}

func TestLoadConfigRejectsPathTraversal(t *testing.T) {
	rootDir := t.TempDir()
	outsideDir := t.TempDir()
	t.Setenv("TTR_CONFIG_ROOT", rootDir)

	evilConfig := `
providers:
  - name: "ecobee"
    enabled: true
    settings:
      client_id: "test"
      refresh_token: "test"

sinks:
  - name: "elasticsearch"
    enabled: true
    settings:
      url: "http://localhost:9200"
`

	absoluteEvilPath := filepath.Join(outsideDir, "evil.yaml")
	if err := os.WriteFile(absoluteEvilPath, []byte(evilConfig), 0644); err != nil {
		t.Fatalf("Failed to write evil config: %v", err)
	}

	if _, err := LoadConfig(absoluteEvilPath); err == nil {
		t.Fatal("expected error when loading config outside allowed root, got nil")
	}

	if _, err := LoadConfig("../evil.yaml"); err == nil {
		t.Fatal("expected error when loading config with relative traversal, got nil")
	}
}
