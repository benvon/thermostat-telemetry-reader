package elasticsearch

import (
	"strings"
	"testing"
	"time"

	"github.com/benvon/thermostat-telemetry-reader/pkg/model"
)

func TestGenerateRuntime5mID(t *testing.T) {
	gen := NewIDGenerator()

	// Create a test document with known values
	doc := &model.Runtime5m{
		Type:           "runtime_5m",
		ThermostatID:   "test-thermostat-123",
		ThermostatName: "Living Room",
		HouseholdID:    "house-456",
		EventTime:      time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Mode:           "heat",
		Climate:        "Home",
		SetHeatC:       floatPtr(20.0),
		SetCoolC:       floatPtr(25.0),
	}

	id, err := gen.GenerateRuntime5mID(doc)
	if err != nil {
		t.Fatalf("Failed to generate ID: %v", err)
	}

	// Verify ID contains expected components
	if !strings.HasPrefix(id, "test-thermostat-123:") {
		t.Errorf("ID should start with thermostat ID, got: %s", id)
	}

	if !strings.Contains(id, "2024-01-15T10:30:00Z") {
		t.Errorf("ID should contain formatted timestamp, got: %s", id)
	}

	if !strings.Contains(id, "runtime_5m") {
		t.Errorf("ID should contain document type, got: %s", id)
	}

	// Should end with a hash
	lastPart := id[strings.LastIndex(id, ":")+1:]
	if len(lastPart) != 16 {
		t.Errorf("Expected 16-character hash at end, got %d characters: %s", len(lastPart), lastPart)
	}

	// Verify determinism - same input should produce same ID
	id2, err := gen.GenerateRuntime5mID(doc)
	if err != nil {
		t.Fatalf("Failed to generate ID: %v", err)
	}
	if id != id2 {
		t.Errorf("IDs should be deterministic. First: %s, Second: %s", id, id2)
	}

	// Verify different data produces different ID
	doc2 := *doc
	doc2.Mode = "cool"
	id3, err := gen.GenerateRuntime5mID(&doc2)
	if err != nil {
		t.Fatalf("Failed to generate ID: %v", err)
	}
	if id == id3 {
		t.Error("Different documents should produce different IDs")
	}
}

func TestGenerateTransitionID(t *testing.T) {
	gen := NewIDGenerator()

	doc := &model.Transition{
		Type:           "transition",
		EventTime:      time.Date(2024, 1, 15, 14, 45, 0, 0, time.UTC),
		ThermostatID:   "test-thermostat-456",
		ThermostatName: "Bedroom",
		Prev: model.State{
			Mode:     "auto",
			SetHeatC: floatPtr(19.0),
			SetCoolC: floatPtr(24.0),
			Climate:  "Home",
		},
		Next: model.State{
			Mode:     "heat",
			SetHeatC: floatPtr(21.0),
			SetCoolC: floatPtr(24.0),
			Climate:  "Home",
		},
	}

	id, err := gen.GenerateTransitionID(doc)
	if err != nil {
		t.Fatalf("Failed to generate ID: %v", err)
	}

	// Verify ID contains expected components
	if !strings.HasPrefix(id, "test-thermostat-456:") {
		t.Errorf("ID should start with thermostat ID, got: %s", id)
	}

	if !strings.Contains(id, "2024-01-15T14:45:00Z") {
		t.Errorf("ID should contain formatted timestamp, got: %s", id)
	}

	// Should end with a hash
	lastPart := id[strings.LastIndex(id, ":")+1:]
	if len(lastPart) != 16 {
		t.Errorf("Expected 16-character hash at end, got %d characters: %s", len(lastPart), lastPart)
	}

	// Verify determinism
	id2, err := gen.GenerateTransitionID(doc)
	if err != nil {
		t.Fatalf("Failed to generate ID: %v", err)
	}
	if id != id2 {
		t.Errorf("IDs should be deterministic. First: %s, Second: %s", id, id2)
	}

	// Verify different transitions produce different IDs
	doc2 := *doc
	doc2.Next.Mode = "cool"
	id3, err := gen.GenerateTransitionID(&doc2)
	if err != nil {
		t.Fatalf("Failed to generate ID: %v", err)
	}
	if id == id3 {
		t.Error("Different transitions should produce different IDs")
	}
}

func TestGenerateDeviceSnapshotID(t *testing.T) {
	gen := NewIDGenerator()

	doc := &model.DeviceSnapshot{
		Type:           "device_snapshot",
		CollectedAt:    time.Date(2024, 1, 15, 16, 0, 0, 0, time.UTC),
		ThermostatID:   "test-thermostat-789",
		ThermostatName: "Kitchen",
		Program:        map[string]any{"name": "Comfort"},
		EventsActive:   []any{map[string]any{"type": "hold"}},
	}

	id, err := gen.GenerateDeviceSnapshotID(doc)
	if err != nil {
		t.Fatalf("Failed to generate ID: %v", err)
	}

	// Verify ID contains expected components
	if !strings.HasPrefix(id, "test-thermostat-789:") {
		t.Errorf("ID should start with thermostat ID, got: %s", id)
	}

	if !strings.Contains(id, "2024-01-15T16:00:00Z") {
		t.Errorf("ID should contain formatted timestamp, got: %s", id)
	}

	// Verify determinism
	id2, err := gen.GenerateDeviceSnapshotID(doc)
	if err != nil {
		t.Fatalf("Failed to generate ID: %v", err)
	}
	if id != id2 {
		t.Errorf("IDs should be deterministic. First: %s, Second: %s", id, id2)
	}

	// Verify different times produce different IDs
	doc2 := *doc
	doc2.CollectedAt = time.Date(2024, 1, 15, 17, 0, 0, 0, time.UTC)
	id3, err := gen.GenerateDeviceSnapshotID(&doc2)
	if err != nil {
		t.Fatalf("Failed to generate ID: %v", err)
	}
	if id == id3 {
		t.Error("Different collection times should produce different IDs")
	}
}

func TestGetIndexName(t *testing.T) {
	sink := NewSink("http://localhost:9200", "test-key", "test-prefix", false)

	tests := []struct {
		name     string
		docType  string
		expected string
	}{
		{
			name:     "runtime_5m type",
			docType:  "runtime_5m",
			expected: "test-prefix-runtime_5m-",
		},
		{
			name:     "transition type",
			docType:  "transition",
			expected: "test-prefix-transition-",
		},
		{
			name:     "device_snapshot type",
			docType:  "device_snapshot",
			expected: "test-prefix-device_snapshot-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sink.getIndexName(tt.docType)

			// Verify prefix and type
			if !strings.HasPrefix(result, tt.expected) {
				t.Errorf("Expected index name to start with %q, got %q", tt.expected, result)
			}

			// Verify date suffix format (YYYY.MM.DD)
			datePart := strings.TrimPrefix(result, tt.expected)
			if len(datePart) != 10 {
				t.Errorf("Expected date part to be 10 characters (YYYY.MM.DD), got %d: %s", len(datePart), datePart)
			}
		})
	}
}

// Helper function
func floatPtr(f float64) *float64 {
	return &f
}
