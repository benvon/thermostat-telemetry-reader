package core

import (
	"testing"
	"time"

	"github.com/benvon/thermostat-telemetry-reader/pkg/model"
)

func TestNewNormalizer(t *testing.T) {
	tests := []struct {
		name      string
		timezone  string
		expectErr bool
	}{
		{
			name:      "valid timezone",
			timezone:  "UTC",
			expectErr: false,
		},
		{
			name:      "valid timezone with location",
			timezone:  "America/New_York",
			expectErr: false,
		},
		{
			name:      "invalid timezone",
			timezone:  "Invalid/Timezone",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalizer, err := NewNormalizer(tt.timezone)
			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if normalizer == nil {
					t.Error("Expected normalizer to be created")
				}
			}
		})
	}
}

func TestNormalizeRuntime5m(t *testing.T) {
	normalizer, err := NewNormalizer("UTC")
	if err != nil {
		t.Fatalf("Failed to create normalizer: %v", err)
	}

	now := time.Now()
	// Providers should convert temperatures to Celsius before calling the normalizer
	// Using realistic Celsius temperatures
	runtime := model.RuntimeRow{
		ThermostatRef: model.ThermostatRef{
			ID:          "test-thermostat",
			Name:        "Test Thermostat",
			Provider:    "ecobee",
			HouseholdID: "test-household",
		},
		EventTime:       now,
		Mode:            "heat",
		Climate:         "home",
		SetHeatC:        floatPtr(20.0), // 20.0°C
		SetCoolC:        floatPtr(25.0), // 25.0°C
		AvgTempC:        floatPtr(22.5), // 22.5°C
		OutdoorTempC:    floatPtr(15.0), // 15.0°C
		OutdoorHumidity: intPtr(60),
		Equipment: map[string]bool{
			"compHeat1": true,
			"fan":       false,
		},
		Sensors: map[string]float64{
			"sensor1": 22.5, // 22.5°C
			"sensor2": 22.0, // 22.0°C
		},
	}

	canonical, err := normalizer.NormalizeRuntime5m(runtime, "ecobee")
	if err != nil {
		t.Fatalf("Failed to normalize runtime: %v", err)
	}

	// Verify canonical format
	if canonical.Type != "runtime_5m" {
		t.Errorf("Expected type runtime_5m, got %s", canonical.Type)
	}
	if canonical.ThermostatID != "test-thermostat" {
		t.Errorf("Expected thermostat ID test-thermostat, got %s", canonical.ThermostatID)
	}
	if canonical.ThermostatName != "Test Thermostat" {
		t.Errorf("Expected thermostat name Test Thermostat, got %s", canonical.ThermostatName)
	}
	if canonical.HouseholdID != "test-household" {
		t.Errorf("Expected household ID test-household, got %s", canonical.HouseholdID)
	}
	if canonical.Mode != "heat" {
		t.Errorf("Expected mode heat, got %s", canonical.Mode)
	}
	if canonical.Climate != "Home" {
		t.Errorf("Expected climate Home, got %s", canonical.Climate)
	}

	// Use tolerance for floating point temperature comparisons
	const epsilon = 0.01
	if canonical.SetHeatC == nil {
		t.Error("Expected SetHeatC to not be nil")
	} else if *canonical.SetHeatC < 20.0-epsilon || *canonical.SetHeatC > 20.0+epsilon {
		t.Errorf("Expected SetHeatC 20.0, got %f", *canonical.SetHeatC)
	}
	if canonical.SetCoolC == nil {
		t.Error("Expected SetCoolC to not be nil")
	} else if *canonical.SetCoolC < 25.0-epsilon || *canonical.SetCoolC > 25.0+epsilon {
		t.Errorf("Expected SetCoolC 25.0, got %f", *canonical.SetCoolC)
	}
	if canonical.AvgTempC == nil {
		t.Error("Expected AvgTempC to not be nil")
	} else if *canonical.AvgTempC < 22.5-epsilon || *canonical.AvgTempC > 22.5+epsilon {
		t.Errorf("Expected AvgTempC 22.5, got %f", *canonical.AvgTempC)
	}
	if canonical.OutdoorTempC == nil {
		t.Error("Expected OutdoorTempC to not be nil")
	} else if *canonical.OutdoorTempC < 15.0-epsilon || *canonical.OutdoorTempC > 15.0+epsilon {
		t.Errorf("Expected OutdoorTempC 15.0, got %f", *canonical.OutdoorTempC)
	}
	if canonical.OutdoorHumidity == nil || *canonical.OutdoorHumidity != 60 {
		t.Errorf("Expected OutdoorHumidity 60, got %v", canonical.OutdoorHumidity)
	}
	if canonical.Equipment["compHeat1"] != true {
		t.Error("Expected compHeat1 to be true")
	}
	if canonical.Equipment["fan"] != false {
		t.Error("Expected fan to be false")
	}
	// Sensor temperature assertions (using same epsilon)
	if canonical.Sensors["sensor1"] < 22.5-epsilon || canonical.Sensors["sensor1"] > 22.5+epsilon {
		t.Errorf("Expected sensor1 22.5, got %f", canonical.Sensors["sensor1"])
	}
	if canonical.Sensors["sensor2"] < 22.0-epsilon || canonical.Sensors["sensor2"] > 22.0+epsilon {
		t.Errorf("Expected sensor2 22.0, got %f", canonical.Sensors["sensor2"])
	}
}

func TestNormalizeMode(t *testing.T) {
	normalizer, err := NewNormalizer("UTC")
	if err != nil {
		t.Fatalf("Failed to create normalizer: %v", err)
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"heat", "heat"},
		{"heating", "heat"},
		{"cool", "cool"},
		{"cooling", "cool"},
		{"auto", "auto"},
		{"automatic", "auto"},
		{"off", "off"},
		{"disabled", "off"},
		{"", "off"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizer.normalizeMode(tt.input)
			if result != tt.expected {
				t.Errorf("Expected mode %s for input %s, got %s", tt.expected, tt.input, result)
			}
		})
	}
}

func TestNormalizeClimate(t *testing.T) {
	normalizer, err := NewNormalizer("UTC")
	if err != nil {
		t.Fatalf("Failed to create normalizer: %v", err)
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"home", "Home"},
		{"Home", "Home"},
		{"HOME", "Home"},
		{"away", "Away"},
		{"Away", "Away"},
		{"AWAY", "Away"},
		{"sleep", "Sleep"},
		{"Sleep", "Sleep"},
		{"SLEEP", "Sleep"},
		{"sleeping", "Sleep"},
		{"vacation", "Vacation"},
		{"Vacation", "Vacation"},
		{"VACATION", "Vacation"},
		{"", "Home"},
		{"custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizer.normalizeClimate(tt.input)
			if result != tt.expected {
				t.Errorf("Expected climate %s for input %s, got %s", tt.expected, tt.input, result)
			}
		})
	}
}

func TestNormalizeEventKind(t *testing.T) {
	normalizer, err := NewNormalizer("UTC")
	if err != nil {
		t.Fatalf("Failed to create normalizer: %v", err)
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"hold", "hold"},
		{"temp_hold", "hold"},
		{"temporary_hold", "hold"},
		{"vacation", "vacation"},
		{"vacation_hold", "vacation"},
		{"resume", "resume"},
		{"resume_schedule", "resume"},
		{"schedule", "schedule"},
		{"scheduled", "schedule"},
		{"manual", "manual"},
		{"manual_override", "manual"},
		{"", "unknown"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizer.normalizeEventKind(tt.input)
			if result != tt.expected {
				t.Errorf("Expected event kind %s for input %s, got %s", tt.expected, tt.input, result)
			}
		})
	}
}

func TestInferEventKindFromName(t *testing.T) {
	normalizer, err := NewNormalizer("UTC")
	if err != nil {
		t.Fatalf("Failed to create normalizer: %v", err)
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"Hold Temperature", "hold"},
		{"Vacation Mode", "vacation"},
		{"Resume Schedule", "resume"},
		{"Scheduled Change", "schedule"},
		{"Manual Override", "manual"},
		{"Unknown Event", "unknown"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizer.inferEventKindFromName(tt.input)
			if result != tt.expected {
				t.Errorf("Expected event kind %s for name %s, got %s", tt.expected, tt.input, result)
			}
		})
	}
}

func TestNormalizeTransition(t *testing.T) {
	normalizer, err := NewNormalizer("UTC")
	if err != nil {
		t.Fatalf("Failed to create normalizer: %v", err)
	}

	now := time.Now()
	thermostatRef := model.ThermostatRef{
		ID:       "test-thermostat",
		Name:     "Test Thermostat",
		Provider: "ecobee",
	}

	prevState := model.State{
		Mode:     "heat",
		SetHeatC: floatPtr(20.0),
		Climate:  "home",
	}

	nextState := model.State{
		Mode:     "cool",
		SetCoolC: floatPtr(25.0),
		Climate:  "away",
	}

	eventInfo := model.EventInfo{
		Kind: "manual",
		Name: "Manual Override",
		Data: map[string]any{"reason": "user_action"},
	}

	transition := normalizer.NormalizeTransition(
		thermostatRef,
		now,
		prevState,
		nextState,
		eventInfo,
		"ecobee",
		map[string]any{"raw_data": "test"},
	)

	// Verify transition format
	if transition.Type != "transition" {
		t.Errorf("Expected type transition, got %s", transition.Type)
	}
	if transition.ThermostatID != "test-thermostat" {
		t.Errorf("Expected thermostat ID test-thermostat, got %s", transition.ThermostatID)
	}
	if transition.ThermostatName != "Test Thermostat" {
		t.Errorf("Expected thermostat name Test Thermostat, got %s", transition.ThermostatName)
	}
	if transition.Prev.Mode != "heat" {
		t.Errorf("Expected prev mode heat, got %s", transition.Prev.Mode)
	}
	if transition.Next.Mode != "cool" {
		t.Errorf("Expected next mode cool, got %s", transition.Next.Mode)
	}
	if transition.Event.Kind != "manual" {
		t.Errorf("Expected event kind manual, got %s", transition.Event.Kind)
	}
	if transition.Event.Name != "Manual Override" {
		t.Errorf("Expected event name Manual Override, got %s", transition.Event.Name)
	}
}

func TestNormalizeDeviceSnapshot(t *testing.T) {
	normalizer, err := NewNormalizer("UTC")
	if err != nil {
		t.Fatalf("Failed to create normalizer: %v", err)
	}

	now := time.Now()
	snapshot := model.Snapshot{
		ThermostatRef: model.ThermostatRef{
			ID:       "test-thermostat",
			Name:     "Test Thermostat",
			Provider: "ecobee",
		},
		CollectedAt:  now,
		Program:      map[string]any{"name": "test_program"},
		EventsActive: []any{map[string]any{"type": "hold"}},
	}

	canonical := normalizer.NormalizeDeviceSnapshot(snapshot, "ecobee")

	// Verify snapshot format
	if canonical.Type != "device_snapshot" {
		t.Errorf("Expected type device_snapshot, got %s", canonical.Type)
	}
	if canonical.ThermostatID != "test-thermostat" {
		t.Errorf("Expected thermostat ID test-thermostat, got %s", canonical.ThermostatID)
	}
	if canonical.ThermostatName != "Test Thermostat" {
		t.Errorf("Expected thermostat name Test Thermostat, got %s", canonical.ThermostatName)
	}
	if canonical.Program == nil {
		t.Error("Expected program to be set")
	}
	if len(canonical.EventsActive) != 1 {
		t.Errorf("Expected 1 active event, got %d", len(canonical.EventsActive))
	}
}

func TestNormalizeEquipment(t *testing.T) {
	normalizer, err := NewNormalizer("UTC")
	if err != nil {
		t.Fatalf("Failed to create normalizer: %v", err)
	}

	t.Run("nil equipment", func(t *testing.T) {
		result := normalizer.normalizeEquipment(nil)
		if result != nil {
			t.Error("Expected nil result for nil input")
		}
	})

	t.Run("empty equipment", func(t *testing.T) {
		result := normalizer.normalizeEquipment(map[string]bool{})
		if len(result) != 0 {
			t.Errorf("Expected empty map, got %d entries", len(result))
		}
	})

	t.Run("normalized keys", func(t *testing.T) {
		input := map[string]bool{
			"compheat1": true,
			"Fan":       true,
		}
		result := normalizer.normalizeEquipment(input)

		if result["compHeat1"] != true {
			t.Error("Expected compHeat1 to be normalized and true")
		}
		if result["fan"] != true {
			t.Error("Expected fan to be normalized and true")
		}
	})

	t.Run("unknown equipment key triggers warning", func(t *testing.T) {
		input := map[string]bool{
			"unknownEquipment": true,
		}
		result := normalizer.normalizeEquipment(input)

		// Should still preserve unknown keys
		if result["unknownEquipment"] != true {
			t.Error("Expected unknown key to be preserved")
		}
	})
}

func TestNormalizeSensors(t *testing.T) {
	normalizer, err := NewNormalizer("UTC")
	if err != nil {
		t.Fatalf("Failed to create normalizer: %v", err)
	}

	t.Run("nil sensors", func(t *testing.T) {
		result := normalizer.normalizeSensors(nil)
		if result != nil {
			t.Error("Expected nil result for nil input")
		}
	})

	t.Run("empty sensors", func(t *testing.T) {
		result := normalizer.normalizeSensors(map[string]float64{})
		if len(result) != 0 {
			t.Errorf("Expected empty map, got %d entries", len(result))
		}
	})

	t.Run("temperature pass-through", func(t *testing.T) {
		// Providers should provide temperatures already in Celsius
		input := map[string]float64{
			"sensor1": 22.2,
			"sensor2": 19.5,
			"sensor3": 25.0,
		}
		result := normalizer.normalizeSensors(input)

		// Should pass through temperatures unchanged
		if result["sensor1"] != 22.2 {
			t.Errorf("Expected sensor1 22.2°C, got %.1f°C", result["sensor1"])
		}
		if result["sensor2"] != 19.5 {
			t.Errorf("Expected sensor2 19.5°C, got %.1f°C", result["sensor2"])
		}
		if result["sensor3"] != 25.0 {
			t.Errorf("Expected sensor3 25.0°C, got %.1f°C", result["sensor3"])
		}
	})
}

func TestPassThroughTemperature(t *testing.T) {
	normalizer, err := NewNormalizer("UTC")
	if err != nil {
		t.Fatalf("Failed to create normalizer: %v", err)
	}

	tests := []struct {
		name     string
		input    *float64
		expected *float64
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "zero temperature",
			input:    floatPtr(0.0),
			expected: floatPtr(0.0),
		},
		{
			name:     "positive temperature",
			input:    floatPtr(22.2),
			expected: floatPtr(22.2),
		},
		{
			name:     "negative temperature",
			input:    floatPtr(-15.5),
			expected: floatPtr(-15.5),
		},
		{
			name:     "high precision temperature",
			input:    floatPtr(20.123456789),
			expected: floatPtr(20.123456789),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.passThroughTemperature(tt.input)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %v", *result)
				}
				return
			}

			if result == nil {
				t.Errorf("Expected %f, got nil", *tt.expected)
				return
			}

			if *result != *tt.expected {
				t.Errorf("Expected %f, got %f", *tt.expected, *result)
			}

			// Verify it's the same pointer (pass-through behavior)
			if result != tt.input {
				t.Error("Expected result to be the same pointer as input (pass-through)")
			}
		})
	}
}

func TestConvertToUTC(t *testing.T) {
	normalizer, err := NewNormalizer("America/New_York")
	if err != nil {
		t.Fatalf("Failed to create normalizer: %v", err)
	}

	t.Run("zero time", func(t *testing.T) {
		result := normalizer.convertToUTC(time.Time{})
		if !result.IsZero() {
			t.Error("Expected zero time to remain zero")
		}
	})

	t.Run("converts to UTC", func(t *testing.T) {
		// Create a time in EST
		est, _ := time.LoadLocation("America/New_York")
		localTime := time.Date(2024, 1, 15, 10, 0, 0, 0, est)

		result := normalizer.convertToUTC(localTime)

		if result.Location() != time.UTC {
			t.Error("Expected result to be in UTC")
		}

		// Verify the time is correct (EST is UTC-5)
		expectedHour := 15 // 10 AM EST = 3 PM UTC
		if result.Hour() != expectedHour {
			t.Errorf("Expected hour %d, got %d", expectedHour, result.Hour())
		}
	})
}

// Helper functions for creating pointers
func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}
