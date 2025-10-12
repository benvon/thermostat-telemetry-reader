package model

import (
	"time"
)

// Runtime5m represents 5-minute runtime telemetry data
type Runtime5m struct {
	Type            string             `json:"type"` // "runtime_5m"
	ThermostatID    string             `json:"thermostat_id"`
	ThermostatName  string             `json:"thermostat_name"`
	HouseholdID     string             `json:"household_id,omitempty"`
	EventTime       time.Time          `json:"event_time"` // bin start
	Mode            string             `json:"mode"`       // heat/cool/auto/off
	Climate         string             `json:"climate"`    // Home/Away/Sleep/...
	SetHeatC        *float64           `json:"set_heat_c,omitempty"`
	SetCoolC        *float64           `json:"set_cool_c,omitempty"`
	AvgTempC        *float64           `json:"avg_temp_c,omitempty"`
	OutdoorTempC    *float64           `json:"outdoor_temp_c,omitempty"`
	OutdoorHumidity *int               `json:"outdoor_humidity_pct,omitempty"`
	Equipment       map[string]bool    `json:"equip,omitempty"`    // compHeat1, compHeat2, compCool1, compCool2, fan
	Sensors         map[string]float64 `json:"sensors,omitempty"`  // sensor_id: temp_c
	Provider        map[string]any     `json:"provider,omitempty"` // provider-specific data
}

// Transition represents a state change event
type Transition struct {
	Type           string         `json:"type"` // "transition"
	EventTime      time.Time      `json:"event_time"`
	ThermostatID   string         `json:"thermostat_id"`
	ThermostatName string         `json:"thermostat_name"`
	Prev           State          `json:"prev"`
	Next           State          `json:"next"`
	Event          EventInfo      `json:"event"`
	Provider       map[string]any `json:"provider,omitempty"`
}

// State represents thermostat state at a point in time
type State struct {
	Mode     string   `json:"mode"`
	SetHeatC *float64 `json:"set_heat_c,omitempty"`
	SetCoolC *float64 `json:"set_cool_c,omitempty"`
	Climate  string   `json:"climate"`
}

// EventInfo contains information about what triggered the transition
type EventInfo struct {
	Kind string         `json:"kind"` // hold/vacation/resume/schedule/manual/unknown
	Name string         `json:"name,omitempty"`
	Data map[string]any `json:"data,omitempty"`
}

// DeviceSnapshot represents current device state
type DeviceSnapshot struct {
	Type           string         `json:"type"` // "device_snapshot"
	CollectedAt    time.Time      `json:"collected_at"`
	ThermostatID   string         `json:"thermostat_id"`
	ThermostatName string         `json:"thermostat_name"`
	Program        any            `json:"program,omitempty"`       // provider metadata
	EventsActive   []any          `json:"events_active,omitempty"` // active holds/vacations
	Provider       map[string]any `json:"provider,omitempty"`
}

// EquipmentState represents the state of HVAC equipment
type EquipmentState struct {
	CompHeat1 bool `json:"compHeat1,omitempty"`
	CompHeat2 bool `json:"compHeat2,omitempty"`
	CompCool1 bool `json:"compCool1,omitempty"`
	CompCool2 bool `json:"compCool2,omitempty"`
	Fan       bool `json:"fan,omitempty"`
}

// ToEquipmentMap converts EquipmentState to a map for JSON serialization
func (e EquipmentState) ToEquipmentMap() map[string]bool {
	result := make(map[string]bool)
	if e.CompHeat1 {
		result["compHeat1"] = true
	}
	if e.CompHeat2 {
		result["compHeat2"] = true
	}
	if e.CompCool1 {
		result["compCool1"] = true
	}
	if e.CompCool2 {
		result["compCool2"] = true
	}
	if e.Fan {
		result["fan"] = true
	}
	return result
}

// FromEquipmentMap creates EquipmentState from a map
func FromEquipmentMap(m map[string]bool) EquipmentState {
	return EquipmentState{
		CompHeat1: m["compHeat1"],
		CompHeat2: m["compHeat2"],
		CompCool1: m["compCool1"],
		CompCool2: m["compCool2"],
		Fan:       m["fan"],
	}
}

// DocumentIDGenerator generates deterministic document IDs
type DocumentIDGenerator interface {
	// GenerateRuntime5mID generates ID for runtime_5m documents
	GenerateRuntime5mID(doc *Runtime5m) (string, error)

	// GenerateTransitionID generates ID for transition documents
	GenerateTransitionID(doc *Transition) (string, error)

	// GenerateDeviceSnapshotID generates ID for device_snapshot documents
	GenerateDeviceSnapshotID(doc *DeviceSnapshot) (string, error)
}
