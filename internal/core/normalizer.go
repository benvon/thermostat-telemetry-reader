package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/benvon/thermostat-telemetry-reader/pkg/model"
)

// Normalizer converts provider-specific data to canonical format
type Normalizer struct {
	timezone *time.Location
}

// NewNormalizer creates a new normalizer
func NewNormalizer(timezone string) (*Normalizer, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("loading timezone %s: %w", timezone, err)
	}

	return &Normalizer{
		timezone: loc,
	}, nil
}

// NormalizeRuntime5m converts provider runtime data to canonical format
func (n *Normalizer) NormalizeRuntime5m(providerData model.RuntimeRow, provider string) (*model.Runtime5m, error) {
	// Convert to canonical format
	canonical := &model.Runtime5m{
		Type:            "runtime_5m",
		ThermostatID:    providerData.ThermostatRef.ID,
		ThermostatName:  providerData.ThermostatRef.Name,
		HouseholdID:     providerData.ThermostatRef.HouseholdID,
		EventTime:       n.convertToUTC(providerData.EventTime),
		Mode:            n.normalizeMode(providerData.Mode),
		Climate:         n.normalizeClimate(providerData.Climate),
		SetHeatC:        n.convertTempToCelsius(providerData.SetHeatC),
		SetCoolC:        n.convertTempToCelsius(providerData.SetCoolC),
		AvgTempC:        n.convertTempToCelsius(providerData.AvgTempC),
		OutdoorTempC:    n.convertTempToCelsius(providerData.OutdoorTempC),
		OutdoorHumidity: providerData.OutdoorHumidity,
		Equipment:       n.normalizeEquipment(providerData.Equipment),
		Sensors:         n.normalizeSensors(providerData.Sensors),
		Provider:        n.createProviderData(provider, providerData),
	}

	return canonical, nil
}

// NormalizeTransition creates a transition document from state changes
func (n *Normalizer) NormalizeTransition(
	thermostatRef model.ThermostatRef,
	eventTime time.Time,
	prevState, nextState model.State,
	eventInfo model.EventInfo,
	provider string,
	providerData any,
) *model.Transition {
	return &model.Transition{
		Type:           "transition",
		EventTime:      n.convertToUTC(eventTime),
		ThermostatID:   thermostatRef.ID,
		ThermostatName: thermostatRef.Name,
		Prev:           n.normalizeState(prevState),
		Next:           n.normalizeState(nextState),
		Event:          n.normalizeEvent(eventInfo),
		Provider:       n.createProviderData(provider, providerData),
	}
}

// NormalizeDeviceSnapshot converts provider snapshot data to canonical format
func (n *Normalizer) NormalizeDeviceSnapshot(
	providerData model.Snapshot,
	provider string,
) *model.DeviceSnapshot {
	return &model.DeviceSnapshot{
		Type:           "device_snapshot",
		CollectedAt:    n.convertToUTC(providerData.CollectedAt),
		ThermostatID:   providerData.ThermostatRef.ID,
		ThermostatName: providerData.ThermostatRef.Name,
		Program:        providerData.Program,
		EventsActive:   providerData.EventsActive,
		Provider:       n.createProviderData(provider, providerData),
	}
}

// convertToUTC converts a time to UTC, preserving the original timezone info
func (n *Normalizer) convertToUTC(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.UTC()
}

// normalizeMode converts provider-specific mode strings to canonical format
func (n *Normalizer) normalizeMode(mode string) string {
	if mode == "" {
		return "off"
	}

	// Convert to lowercase for comparison
	modeLower := strings.ToLower(mode)

	// Map common variations to canonical values
	switch modeLower {
	case "heat", "heating":
		return "heat"
	case "cool", "cooling":
		return "cool"
	case "auto", "automatic":
		return "auto"
	case "off", "disabled":
		return "off"
	default:
		return modeLower // Keep original if not recognized
	}
}

// normalizeClimate converts provider-specific climate strings to canonical format
func (n *Normalizer) normalizeClimate(climate string) string {
	if climate == "" {
		return "Home"
	}

	// Map common climate variations
	switch climate {
	case "home", "Home", "HOME":
		return "Home"
	case "away", "Away", "AWAY":
		return "Away"
	case "sleep", "Sleep", "SLEEP", "sleeping":
		return "Sleep"
	case "vacation", "Vacation", "VACATION":
		return "Vacation"
	default:
		return climate // Keep original if not recognized
	}
}

// convertTempToCelsius converts temperature values to Celsius
func (n *Normalizer) convertTempToCelsius(temp *float64) *float64 {
	if temp == nil {
		return nil
	}

	// Assume input is already in Celsius for now
	// In a real implementation, you might need to convert from Fahrenheit
	// based on provider or configuration
	return temp
}

// normalizeEquipment ensures equipment state is properly formatted
func (n *Normalizer) normalizeEquipment(equipment map[string]bool) map[string]bool {
	if equipment == nil {
		return nil
	}

	normalized := make(map[string]bool)
	for key, value := range equipment {
		// Ensure consistent naming
		normalizedKey := n.normalizeEquipmentKey(key)
		normalized[normalizedKey] = value
	}

	return normalized
}

// normalizeEquipmentKey converts equipment key names to canonical format
func (n *Normalizer) normalizeEquipmentKey(key string) string {
	switch key {
	case "compHeat1", "compheat1", "comp_heat_1":
		return "compHeat1"
	case "compHeat2", "compheat2", "comp_heat_2":
		return "compHeat2"
	case "compCool1", "compcool1", "comp_cool_1":
		return "compCool1"
	case "compCool2", "compcool2", "comp_cool_2":
		return "compCool2"
	case "fan", "Fan", "FAN":
		return "fan"
	default:
		return key
	}
}

// normalizeSensors ensures sensor data is properly formatted
func (n *Normalizer) normalizeSensors(sensors map[string]float64) map[string]float64 {
	if sensors == nil {
		return nil
	}

	// Convert temperatures to Celsius if needed
	normalized := make(map[string]float64)
	for sensorID, temp := range sensors {
		normalized[sensorID] = *n.convertTempToCelsius(&temp)
	}

	return normalized
}

// normalizeState normalizes a thermostat state
func (n *Normalizer) normalizeState(state model.State) model.State {
	return model.State{
		Mode:     n.normalizeMode(state.Mode),
		SetHeatC: n.convertTempToCelsius(state.SetHeatC),
		SetCoolC: n.convertTempToCelsius(state.SetCoolC),
		Climate:  n.normalizeClimate(state.Climate),
	}
}

// normalizeEvent normalizes event information
func (n *Normalizer) normalizeEvent(event model.EventInfo) model.EventInfo {
	normalized := model.EventInfo{
		Kind: n.normalizeEventKind(event.Kind),
		Name: event.Name,
		Data: event.Data,
	}

	// If kind is unknown, try to infer from name
	if normalized.Kind == "unknown" && event.Name != "" {
		normalized.Kind = n.inferEventKindFromName(event.Name)
	}

	return normalized
}

// normalizeEventKind converts event kinds to canonical format
func (n *Normalizer) normalizeEventKind(kind string) string {
	if kind == "" {
		return "unknown"
	}

	kindLower := strings.ToLower(kind)
	switch kindLower {
	case "hold", "temp_hold", "temporary_hold":
		return "hold"
	case "vacation", "vacation_hold":
		return "vacation"
	case "resume", "resume_schedule":
		return "resume"
	case "schedule", "scheduled":
		return "schedule"
	case "manual", "manual_override":
		return "manual"
	default:
		return "unknown"
	}
}

// inferEventKindFromName tries to infer event kind from the event name
func (n *Normalizer) inferEventKindFromName(name string) string {
	nameLower := strings.ToLower(name)

	if contains(nameLower, "hold") {
		if contains(nameLower, "vacation") {
			return "vacation"
		}
		return "hold"
	}

	if contains(nameLower, "vacation") {
		return "vacation"
	}

	if contains(nameLower, "resume") {
		return "resume"
	}

	if contains(nameLower, "schedule") {
		return "schedule"
	}

	if contains(nameLower, "manual") {
		return "manual"
	}

	return "unknown"
}

// createProviderData creates a provider-specific data section
func (n *Normalizer) createProviderData(provider string, data any) map[string]any {
	return map[string]any{
		provider: data,
	}
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return indexOf(s, substr) >= 0
}

// indexOf finds the index of a substring in a string
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
