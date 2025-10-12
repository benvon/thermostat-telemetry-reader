package model

import (
	"context"
	"time"
)

// ProviderInfo contains metadata about a provider implementation
type ProviderInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// SinkInfo contains metadata about a sink implementation
type SinkInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// ThermostatRef identifies a thermostat across providers
type ThermostatRef struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	HouseholdID string `json:"household_id,omitempty"`
}

// AuthManager handles authentication for providers
type AuthManager interface {
	// RefreshToken refreshes the authentication token if needed
	RefreshToken(ctx context.Context) error

	// GetAccessToken returns the current access token
	GetAccessToken(ctx context.Context) (string, error)

	// IsTokenValid checks if the current token is valid
	IsTokenValid(ctx context.Context) bool
}

// Summary contains high-level thermostat information for change detection
type Summary struct {
	ThermostatRef ThermostatRef `json:"thermostat_ref"`
	Revision      string        `json:"revision"`
	LastUpdate    time.Time     `json:"last_update"`
}

// Snapshot contains current thermostat state and active events
type Snapshot struct {
	ThermostatRef ThermostatRef `json:"thermostat_ref"`
	CollectedAt   time.Time     `json:"collected_at"`
	Program       any           `json:"program,omitempty"`
	EventsActive  []any         `json:"events_active,omitempty"`
}

// RuntimeRow contains 5-minute runtime data
type RuntimeRow struct {
	ThermostatRef   ThermostatRef      `json:"thermostat_ref"`
	EventTime       time.Time          `json:"event_time"`
	Mode            string             `json:"mode"`
	Climate         string             `json:"climate"`
	SetHeatC        *float64           `json:"set_heat_c,omitempty"`
	SetCoolC        *float64           `json:"set_cool_c,omitempty"`
	AvgTempC        *float64           `json:"avg_temp_c,omitempty"`
	OutdoorTempC    *float64           `json:"outdoor_temp_c,omitempty"`
	OutdoorHumidity *int               `json:"outdoor_humidity_pct,omitempty"`
	Equipment       map[string]bool    `json:"equip,omitempty"`
	Sensors         map[string]float64 `json:"sensors,omitempty"`
}

// Provider defines the interface for thermostat data providers
type Provider interface {
	// Info returns metadata about the provider
	Info() ProviderInfo

	// ListThermostats returns all thermostats available to this provider
	ListThermostats(ctx context.Context) ([]ThermostatRef, error)

	// GetSummary returns high-level information for change detection
	GetSummary(ctx context.Context, tr ThermostatRef) (Summary, error)

	// GetSnapshot returns current thermostat state
	GetSnapshot(ctx context.Context, tr ThermostatRef, since time.Time) (Snapshot, error)

	// GetRuntime returns historical runtime data for the specified time range
	GetRuntime(ctx context.Context, tr ThermostatRef, from, to time.Time) ([]RuntimeRow, error)

	// Auth returns the authentication manager for this provider
	Auth() AuthManager
}

// Doc represents a document to be written to a sink
type Doc struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Body any    `json:"body"`
}

// WriteResult contains information about a write operation
type WriteResult struct {
	SuccessCount int      `json:"success_count"`
	ErrorCount   int      `json:"error_count"`
	Errors       []string `json:"errors,omitempty"`
}

// Sink defines the interface for data storage sinks
type Sink interface {
	// Info returns metadata about the sink
	Info() SinkInfo

	// Open initializes the sink connection
	Open(ctx context.Context) error

	// Write writes documents to the sink
	Write(ctx context.Context, docs []Doc) (WriteResult, error)

	// Close closes the sink connection
	Close(ctx context.Context) error
}
