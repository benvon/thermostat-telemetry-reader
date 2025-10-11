package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
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
func LoadConfig(configPath string) (*Config, error) {
	// Read the configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", configPath, err)
	}

	// Substitute environment variables
	content := substituteEnvVars(string(data))

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal([]byte(content), &config); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Set defaults
	setDefaults(&config)

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &config, nil
}

// substituteEnvVars replaces ${VAR} and ${VAR:-default} patterns with environment variable values
func substituteEnvVars(content string) string {
	// Replace ${VAR} patterns
	content = strings.ReplaceAll(content, "${", "")
	content = strings.ReplaceAll(content, "}", "")

	// For now, implement a simple substitution
	// In a production system, you might want to use a more sophisticated templating library
	return content
}

// setDefaults sets default values for configuration fields
func setDefaults(config *Config) {
	if config.TTR.Timezone == "" {
		config.TTR.Timezone = "UTC"
	}
	if config.TTR.PollInterval == 0 {
		config.TTR.PollInterval = 5 * time.Minute
	}
	if config.TTR.BackfillWindow == 0 {
		config.TTR.BackfillWindow = 168 * time.Hour // 7 days
	}
	if config.TTR.LogLevel == "" {
		config.TTR.LogLevel = "info"
	}
	if config.TTR.HealthPort == 0 {
		config.TTR.HealthPort = 8080
	}
	if config.TTR.MetricsPort == 0 {
		config.TTR.MetricsPort = 9090
	}
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
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing example config: %w", err)
	}

	return nil
}
