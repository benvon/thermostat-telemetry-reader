package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Configuration keys - centralized to keep flags/env/file aligned
const (
	keyTTRTimezone       = "ttr.timezone"
	keyTTRPollInterval   = "ttr.poll_interval"
	keyTTRBackfillWindow = "ttr.backfill_window"
	keyTTRLogLevel       = "ttr.log_level"
	keyTTRHealthPort     = "ttr.health_port"
	keyTTRMetricsPort    = "ttr.metrics_port"
)

// Environment variable names
const (
	envTTRTimezone       = "TTR_TIMEZONE"
	envTTRPollInterval   = "TTR_POLL_INTERVAL"
	envTTRBackfillWindow = "TTR_BACKFILL_WINDOW"
	envTTRLogLevel       = "TTR_LOG_LEVEL"
	envTTRHealthPort     = "TTR_HEALTH_PORT"
	envTTRMetricsPort    = "TTR_METRICS_PORT"
)

// Config represents the complete application configuration
type Config struct {
	TTR       TTRConfig        `yaml:"ttr"`
	Providers []ProviderConfig `yaml:"providers"`
	Sinks     []SinkConfig     `yaml:"sinks"`
}

// TTRConfig contains core application settings
type TTRConfig struct {
	Timezone       string        `yaml:"timezone"`
	PollInterval   time.Duration `yaml:"poll_interval"`
	BackfillWindow time.Duration `yaml:"backfill_window"`
	LogLevel       string        `yaml:"log_level"`
	HealthPort     int           `yaml:"health_port"`
	MetricsPort    int           `yaml:"metrics_port"`
}

// ProviderConfig contains provider-specific configuration
type ProviderConfig struct {
	Name     string         `yaml:"name"`
	Enabled  bool           `yaml:"enabled"`
	Settings map[string]any `yaml:"settings,omitempty"`
}

// SinkConfig contains sink-specific configuration
type SinkConfig struct {
	Name     string         `yaml:"name"`
	Enabled  bool           `yaml:"enabled"`
	Settings map[string]any `yaml:"settings,omitempty"`
}

// LoadConfig loads configuration from a YAML file with environment variable substitution
//
// Configuration Precedence (highest to lowest):
//  1. Environment variables (TTR_LOG_LEVEL, TTR_POLL_INTERVAL, etc.)
//  2. Configuration file values
//  3. Default values
//
// Environment Variable Mapping:
//   - TTR_TIMEZONE       → ttr.timezone
//   - TTR_LOG_LEVEL      → ttr.log_level
//   - TTR_POLL_INTERVAL  → ttr.poll_interval
//   - TTR_BACKFILL_WINDOW → ttr.backfill_window
//   - TTR_HEALTH_PORT    → ttr.health_port
//   - TTR_METRICS_PORT   → ttr.metrics_port
//
// For nested provider/sink settings:
//   - PROVIDERS_0_SETTINGS_CLIENT_ID → providers[0].settings.client_id
//   - SINKS_0_SETTINGS_API_KEY       → sinks[0].settings.api_key
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// Set configuration file
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Enable automatic environment variable binding
	v.AutomaticEnv()

	// Replace dots and dashes in env var names with underscores
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// Bind specific environment variables for core settings
	bindCoreEnvVars(v)

	// Read configuration file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", configPath, err)
	}

	// Parse YAML directly first to get the basic structure
	config, err := parseYAMLConfig(configPath)
	if err != nil {
		return nil, err
	}

	// Set defaults in Viper
	setViperDefaults(v)

	// Apply configuration overrides from environment variables
	applyTTRConfigOverrides(v, &config.TTR)
	applyProviderEnvOverrides(config.Providers)
	applySinkEnvOverrides(config.Sinks)

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return config, nil
}

// bindCoreEnvVars binds core TTR environment variables to Viper
func bindCoreEnvVars(v *viper.Viper) {
	_ = v.BindEnv(keyTTRTimezone, envTTRTimezone)
	_ = v.BindEnv(keyTTRPollInterval, envTTRPollInterval)
	_ = v.BindEnv(keyTTRBackfillWindow, envTTRBackfillWindow)
	_ = v.BindEnv(keyTTRLogLevel, envTTRLogLevel)
	_ = v.BindEnv(keyTTRHealthPort, envTTRHealthPort)
	_ = v.BindEnv(keyTTRMetricsPort, envTTRMetricsPort)
}

// parseYAMLConfig reads and parses the YAML configuration file
func parseYAMLConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file for YAML parsing: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing YAML config: %w", err)
	}

	return &config, nil
}

// applyTTRConfigOverrides applies environment variable overrides to TTR config
func applyTTRConfigOverrides(v *viper.Viper, ttr *TTRConfig) {
	// Handle durations with environment variable overrides
	applyDurationOverride(v, keyTTRPollInterval, &ttr.PollInterval, 5*time.Minute)
	applyDurationOverride(v, keyTTRBackfillWindow, &ttr.BackfillWindow, 168*time.Hour)

	// Handle string overrides with defaults
	applyStringOverride(v, keyTTRTimezone, &ttr.Timezone, "UTC")
	applyStringOverride(v, keyTTRLogLevel, &ttr.LogLevel, "info")

	// Handle int overrides with defaults
	applyIntOverride(v, keyTTRHealthPort, &ttr.HealthPort, 8080)
	applyIntOverride(v, keyTTRMetricsPort, &ttr.MetricsPort, 9090)
}

// applyDurationOverride applies a duration override from environment variable or uses default
func applyDurationOverride(v *viper.Viper, key string, target *time.Duration, defaultVal time.Duration) {
	if strVal := v.GetString(key); strVal != "" {
		if dur, err := time.ParseDuration(strVal); err == nil {
			*target = dur
			return
		}
	}
	if *target == 0 {
		*target = defaultVal
	}
}

// applyStringOverride applies a string override from environment variable or uses default
func applyStringOverride(v *viper.Viper, key string, target *string, defaultVal string) {
	if v.IsSet(key) {
		*target = v.GetString(key)
	} else if *target == "" {
		*target = defaultVal
	}
}

// applyIntOverride applies an int override from environment variable or uses default
func applyIntOverride(v *viper.Viper, key string, target *int, defaultVal int) {
	if v.IsSet(key) {
		*target = v.GetInt(key)
	} else if *target == 0 {
		*target = defaultVal
	}
}

// applyProviderEnvOverrides applies environment variable overrides to provider settings
// Supports environment variables like: PROVIDERS_0_SETTINGS_CLIENT_ID, PROVIDERS_1_SETTINGS_REFRESH_TOKEN, etc.
func applyProviderEnvOverrides(providers []ProviderConfig) {
	commonSettings := []string{"client_id", "refresh_token", "api_key", "api_secret"}

	for i := range providers {
		if providers[i].Settings == nil {
			providers[i].Settings = make(map[string]any)
		}

		envPrefix := fmt.Sprintf("PROVIDERS_%d_SETTINGS_", i)
		applySettingsEnvOverrides(providers[i].Settings, envPrefix, commonSettings)
	}
}

// applySinkEnvOverrides applies environment variable overrides to sink settings
// Supports environment variables like: SINKS_0_SETTINGS_API_KEY, SINKS_1_SETTINGS_URL, etc.
func applySinkEnvOverrides(sinks []SinkConfig) {
	commonSettings := []string{"api_key", "url", "username", "password"}

	for i := range sinks {
		if sinks[i].Settings == nil {
			sinks[i].Settings = make(map[string]any)
		}

		envPrefix := fmt.Sprintf("SINKS_%d_SETTINGS_", i)
		applySettingsEnvOverrides(sinks[i].Settings, envPrefix, commonSettings)
	}
}

// applySettingsEnvOverrides applies environment variable overrides to a settings map
func applySettingsEnvOverrides(settings map[string]any, envPrefix string, commonSettings []string) {
	// Check existing settings keys
	for key := range settings {
		envKey := envPrefix + strings.ToUpper(key)
		if envVal := os.Getenv(envKey); envVal != "" {
			settings[key] = envVal
		}
	}

	// Check common settings that might not be in the config yet
	for _, setting := range commonSettings {
		envKey := envPrefix + strings.ToUpper(setting)
		if envVal := os.Getenv(envKey); envVal != "" {
			settings[setting] = envVal
		}
	}
}

// PrintEffectiveConfig prints the effective configuration for observability
// Note: Sensitive values (like API keys, tokens) are redacted for security
func (c *Config) PrintEffectiveConfig() {
	fmt.Println("=== Effective Configuration ===")
	fmt.Printf("TTR Settings:\n")
	fmt.Printf("  Timezone: %s\n", c.TTR.Timezone)
	fmt.Printf("  Poll Interval: %v\n", c.TTR.PollInterval)
	fmt.Printf("  Backfill Window: %v\n", c.TTR.BackfillWindow)
	fmt.Printf("  Log Level: %s\n", c.TTR.LogLevel)
	fmt.Printf("  Health Port: %d\n", c.TTR.HealthPort)
	fmt.Printf("  Metrics Port: %d\n", c.TTR.MetricsPort)

	fmt.Printf("Providers (%d configured):\n", len(c.Providers))
	for i, provider := range c.Providers {
		fmt.Printf("  [%d] %s (enabled: %v)\n", i, provider.Name, provider.Enabled)
		for key, value := range provider.Settings {
			// Redact sensitive values
			if isSensitiveKey(key) {
				fmt.Printf("    %s: [REDACTED]\n", key)
			} else {
				fmt.Printf("    %s: %v\n", key, value)
			}
		}
	}

	fmt.Printf("Sinks (%d configured):\n", len(c.Sinks))
	for i, sink := range c.Sinks {
		fmt.Printf("  [%d] %s (enabled: %v)\n", i, sink.Name, sink.Enabled)
		for key, value := range sink.Settings {
			// Redact sensitive values
			if isSensitiveKey(key) {
				fmt.Printf("    %s: [REDACTED]\n", key)
			} else {
				fmt.Printf("    %s: %v\n", key, value)
			}
		}
	}
	fmt.Println("===============================")
}

// isSensitiveKey checks if a configuration key contains sensitive information
func isSensitiveKey(key string) bool {
	sensitiveKeys := []string{
		"api_key", "token", "password", "secret", "key", "client_secret",
		"refresh_token", "access_token", "auth_token", "bearer_token",
	}

	keyLower := strings.ToLower(key)
	for _, sensitive := range sensitiveKeys {
		if strings.Contains(keyLower, sensitive) {
			return true
		}
	}
	return false
}

// GetEnvironmentVariableHelp returns documentation for environment variables
// This can be used in CLI help text or documentation generation
func GetEnvironmentVariableHelp() string {
	return `Environment Variables:
  TTR_TIMEZONE        Set timezone (default: UTC)
  TTR_LOG_LEVEL       Set log level: debug, info, warn, error (default: info)
  TTR_POLL_INTERVAL   Set polling interval, e.g., "5m", "30s" (default: 5m)
  TTR_BACKFILL_WINDOW Set backfill window, e.g., "168h", "7d" (default: 168h)
  TTR_HEALTH_PORT     Set health check port (default: 8080)
  TTR_METRICS_PORT    Set metrics port (default: 9090)

Provider/Sink Settings (supports multiple indices):
  PROVIDERS_{N}_SETTINGS_{KEY}  Override provider N setting (e.g., PROVIDERS_0_SETTINGS_CLIENT_ID)
  SINKS_{N}_SETTINGS_{KEY}      Override sink N setting (e.g., SINKS_0_SETTINGS_API_KEY)
  
  Common provider settings: CLIENT_ID, REFRESH_TOKEN, API_KEY, API_SECRET
  Common sink settings: API_KEY, URL, USERNAME, PASSWORD

Examples:
  PROVIDERS_0_SETTINGS_CLIENT_ID=abc123
  PROVIDERS_1_SETTINGS_REFRESH_TOKEN=xyz789
  SINKS_0_SETTINGS_API_KEY=secret123
  SINKS_1_SETTINGS_URL=https://es.example.com

Configuration Precedence (highest to lowest):
  1. Environment variables
  2. Configuration file values  
  3. Built-in defaults`
}

// setViperDefaults sets default values in Viper before unmarshaling
func setViperDefaults(v *viper.Viper) {
	v.SetDefault(keyTTRTimezone, "UTC")
	v.SetDefault(keyTTRPollInterval, 5*time.Minute)
	v.SetDefault(keyTTRBackfillWindow, 168*time.Hour)
	v.SetDefault(keyTTRLogLevel, "info")
	v.SetDefault(keyTTRHealthPort, 8080)
	v.SetDefault(keyTTRMetricsPort, 9090)
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	if config.TTR.PollInterval < time.Minute {
		return fmt.Errorf("poll_interval must be at least 1 minute")
	}
	if config.TTR.BackfillWindow < time.Hour {
		return fmt.Errorf("backfill_window must be at least 1 hour")
	}

	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[config.TTR.LogLevel] {
		return fmt.Errorf("invalid log_level: %s, must be one of: debug, info, warn, error", config.TTR.LogLevel)
	}

	// Check that at least one provider is enabled
	hasEnabledProvider := false
	for _, provider := range config.Providers {
		if provider.Enabled {
			hasEnabledProvider = true
			break
		}
	}
	if !hasEnabledProvider {
		return fmt.Errorf("at least one provider must be enabled")
	}

	// Check that at least one sink is enabled
	hasEnabledSink := false
	for _, sink := range config.Sinks {
		if sink.Enabled {
			hasEnabledSink = true
			break
		}
	}
	if !hasEnabledSink {
		return fmt.Errorf("at least one sink must be enabled")
	}

	return nil
}

// GetProviderConfig returns the configuration for a specific provider
func (c *Config) GetProviderConfig(name string) (*ProviderConfig, error) {
	for _, provider := range c.Providers {
		if provider.Name == name {
			return &provider, nil
		}
	}
	return nil, fmt.Errorf("provider %s not found in configuration", name)
}

// GetSinkConfig returns the configuration for a specific sink
func (c *Config) GetSinkConfig(name string) (*SinkConfig, error) {
	for _, sink := range c.Sinks {
		if sink.Name == name {
			return &sink, nil
		}
	}
	return nil, fmt.Errorf("sink %s not found in configuration", name)
}

// GetEnabledProviders returns all enabled provider configurations
func (c *Config) GetEnabledProviders() []ProviderConfig {
	var enabled []ProviderConfig
	for _, provider := range c.Providers {
		if provider.Enabled {
			enabled = append(enabled, provider)
		}
	}
	return enabled
}

// GetEnabledSinks returns all enabled sink configurations
func (c *Config) GetEnabledSinks() []SinkConfig {
	var enabled []SinkConfig
	for _, sink := range c.Sinks {
		if sink.Enabled {
			enabled = append(enabled, sink)
		}
	}
	return enabled
}

// CreateExampleConfig creates an example configuration file
func CreateExampleConfig(path string) error {
	config := Config{
		TTR: TTRConfig{
			Timezone:       "America/Chicago",
			PollInterval:   5 * time.Minute,
			BackfillWindow: 168 * time.Hour,
			LogLevel:       "info",
			HealthPort:     8080,
			MetricsPort:    9090,
		},
		Providers: []ProviderConfig{
			{
				Name:    "ecobee",
				Enabled: true,
				Settings: map[string]any{
					"client_id":     "${ECOBEE_CLIENT_ID}",
					"refresh_token": "${ECOBEE_REFRESH_TOKEN}",
				},
			},
		},
		Sinks: []SinkConfig{
			{
				Name:    "elasticsearch",
				Enabled: true,
				Settings: map[string]any{
					"url":              "https://es.example:9200",
					"api_key":          "${ELASTIC_API_KEY}",
					"index_prefix":     "ttr",
					"create_templates": true,
				},
			},
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshaling example config: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing example config: %w", err)
	}

	return nil
}
