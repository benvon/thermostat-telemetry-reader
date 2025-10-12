package core

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/benvon/thermostat-telemetry-reader/pkg/model"
)

// Normalizer converts provider-specific data to canonical format
type Normalizer struct {
	timezone        *time.Location
	modeMap         map[string]string
	climateMap      map[string]string
	equipmentKeyMap map[string]string
	eventKindMap    map[string]string
	logger          *slog.Logger
}

// NewNormalizer creates a new normalizer
func NewNormalizer(timezone string) (*Normalizer, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("loading timezone %s: %w", timezone, err)
	}

	// Use default logger if none provided
	logger := slog.Default()

	return &Normalizer{
		timezone: loc,
		logger:   logger,
		modeMap: map[string]string{
			"heat":      "heat",
			"heating":   "heat",
			"cool":      "cool",
			"cooling":   "cool",
			"auto":      "auto",
			"automatic": "auto",
			"off":       "off",
			"disabled":  "off",
		},
		climateMap: map[string]string{
			"home":     "Home",
			"Home":     "Home",
			"HOME":     "Home",
			"away":     "Away",
			"Away":     "Away",
			"AWAY":     "Away",
			"sleep":    "Sleep",
			"Sleep":    "Sleep",
			"SLEEP":    "Sleep",
			"sleeping": "Sleep",
			"vacation": "Vacation",
			"Vacation": "Vacation",
			"VACATION": "Vacation",
		},
		equipmentKeyMap: map[string]string{
			"compHeat1":   "compHeat1",
			"compheat1":   "compHeat1",
			"comp_heat_1": "compHeat1",
			"compHeat2":   "compHeat2",
			"compheat2":   "compHeat2",
			"comp_heat_2": "compHeat2",
			"compCool1":   "compCool1",
			"compcool1":   "compCool1",
			"comp_cool_1": "compCool1",
			"compCool2":   "compCool2",
			"compcool2":   "compCool2",
			"comp_cool_2": "compCool2",
			"fan":         "fan",
			"Fan":         "fan",
			"FAN":         "fan",
		},
		eventKindMap: map[string]string{
			"hold":            "hold",
			"temp_hold":       "hold",
			"temporary_hold":  "hold",
			"vacation":        "vacation",
			"vacation_hold":   "vacation",
			"resume":          "resume",
			"resume_schedule": "resume",
			"schedule":        "schedule",
			"scheduled":       "schedule",
			"manual":          "manual",
			"manual_override": "manual",
		},
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
		SetHeatC:        n.passThroughTemperature(providerData.SetHeatC),
		SetCoolC:        n.passThroughTemperature(providerData.SetCoolC),
		AvgTempC:        n.passThroughTemperature(providerData.AvgTempC),
		OutdoorTempC:    n.passThroughTemperature(providerData.OutdoorTempC),
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

	modeLower := strings.ToLower(mode)
	if normalized, ok := n.modeMap[modeLower]; ok {
		return normalized
	}

	// Log unmapped value for visibility
	n.logger.Warn("Unmapped mode value encountered",
		"original", mode,
		"lowercase", modeLower,
		"suggestion", "add to modeMap if this is a valid mode")

	return modeLower // Keep original if not recognized
}

// normalizeClimate converts provider-specific climate strings to canonical format
func (n *Normalizer) normalizeClimate(climate string) string {
	if climate == "" {
		return "Home"
	}

	if normalized, ok := n.climateMap[climate]; ok {
		return normalized
	}

	// Log unmapped value for visibility
	n.logger.Warn("Unmapped climate value encountered",
		"original", climate,
		"suggestion", "add to climateMap if this is a valid climate")

	return climate // Keep original if not recognized
}

// passThroughTemperature passes temperature values through unchanged
// The normalizer now assumes that providers have already converted temperatures to Celsius
func (n *Normalizer) passThroughTemperature(temp *float64) *float64 {
	// Simply pass through the temperature value
	// Providers are responsible for converting their temperature formats to Celsius
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
	if normalized, ok := n.equipmentKeyMap[key]; ok {
		return normalized
	}

	// Log unmapped value for visibility
	n.logger.Warn("Unmapped equipment key encountered",
		"original", key,
		"suggestion", "add to equipmentKeyMap if this is a valid equipment key")

	return key
}

// normalizeSensors ensures sensor data is properly formatted
func (n *Normalizer) normalizeSensors(sensors map[string]float64) map[string]float64 {
	if sensors == nil {
		return nil
	}

	// Pass through sensor temperatures (providers should already have converted to Celsius)
	normalized := make(map[string]float64)
	for sensorID, temp := range sensors {
		normalized[sensorID] = *n.passThroughTemperature(&temp)
	}

	return normalized
}

// normalizeState normalizes a thermostat state
func (n *Normalizer) normalizeState(state model.State) model.State {
	return model.State{
		Mode:     n.normalizeMode(state.Mode),
		SetHeatC: n.passThroughTemperature(state.SetHeatC),
		SetCoolC: n.passThroughTemperature(state.SetCoolC),
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
	if normalized, ok := n.eventKindMap[kindLower]; ok {
		return normalized
	}

	// Log unmapped value for visibility
	n.logger.Warn("Unmapped event kind encountered",
		"original", kind,
		"lowercase", kindLower,
		"suggestion", "add to eventKindMap if this is a valid event kind")

	return "unknown"
}

// inferEventKindFromName tries to infer event kind from the event name
func (n *Normalizer) inferEventKindFromName(name string) string {
	nameLower := strings.ToLower(name)

	if strings.Contains(nameLower, "hold") {
		if strings.Contains(nameLower, "vacation") {
			return "vacation"
		}
		return "hold"
	}

	if strings.Contains(nameLower, "vacation") {
		return "vacation"
	}

	if strings.Contains(nameLower, "resume") {
		return "resume"
	}

	if strings.Contains(nameLower, "schedule") {
		return "schedule"
	}

	if strings.Contains(nameLower, "manual") {
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
