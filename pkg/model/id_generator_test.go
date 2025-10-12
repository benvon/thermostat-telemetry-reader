package model

import (
	"testing"
	"time"
)

func TestIDGenerator_GenerateRuntime5mID(t *testing.T) {
	t.Parallel()

	gen := NewIDGenerator()

	t.Run("generates deterministic ID", func(t *testing.T) {
		eventTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		temp := 21.5

		doc := &Runtime5m{
			Type:           "runtime_5m",
			ThermostatID:   "test-123",
			ThermostatName: "Living Room",
			EventTime:      eventTime,
			Mode:           "heat",
			Climate:        "Home",
			AvgTempC:       &temp,
		}

		id1, err := gen.GenerateRuntime5mID(doc)
		if err != nil {
			t.Fatalf("Failed to generate ID: %v", err)
		}

		id2, err := gen.GenerateRuntime5mID(doc)
		if err != nil {
			t.Fatalf("Failed to generate ID: %v", err)
		}

		if id1 != id2 {
			t.Errorf("IDs should be deterministic: %s != %s", id1, id2)
		}
	})

	t.Run("different documents produce different IDs", func(t *testing.T) {
		eventTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		temp1 := 21.5
		temp2 := 22.0

		doc1 := &Runtime5m{
			Type:         "runtime_5m",
			ThermostatID: "test-123",
			EventTime:    eventTime,
			AvgTempC:     &temp1,
		}

		doc2 := &Runtime5m{
			Type:         "runtime_5m",
			ThermostatID: "test-123",
			EventTime:    eventTime,
			AvgTempC:     &temp2,
		}

		id1, err := gen.GenerateRuntime5mID(doc1)
		if err != nil {
			t.Fatalf("Failed to generate ID: %v", err)
		}

		id2, err := gen.GenerateRuntime5mID(doc2)
		if err != nil {
			t.Fatalf("Failed to generate ID: %v", err)
		}

		if id1 == id2 {
			t.Errorf("Different documents should produce different IDs")
		}
	})

	t.Run("handles nil document", func(t *testing.T) {
		_, err := gen.GenerateRuntime5mID(nil)
		if err == nil {
			t.Error("Expected error for nil document")
		}
	})
}

func TestIDGenerator_GenerateTransitionID(t *testing.T) {
	t.Parallel()

	gen := NewIDGenerator()

	t.Run("generates deterministic ID", func(t *testing.T) {
		eventTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		heatTemp := 20.0
		coolTemp := 24.0

		doc := &Transition{
			Type:           "transition",
			EventTime:      eventTime,
			ThermostatID:   "test-123",
			ThermostatName: "Living Room",
			Prev: State{
				Mode:     "heat",
				SetHeatC: &heatTemp,
				Climate:  "Home",
			},
			Next: State{
				Mode:     "cool",
				SetCoolC: &coolTemp,
				Climate:  "Away",
			},
		}

		id1, err := gen.GenerateTransitionID(doc)
		if err != nil {
			t.Fatalf("Failed to generate ID: %v", err)
		}

		id2, err := gen.GenerateTransitionID(doc)
		if err != nil {
			t.Fatalf("Failed to generate ID: %v", err)
		}

		if id1 != id2 {
			t.Errorf("IDs should be deterministic: %s != %s", id1, id2)
		}
	})

	t.Run("handles nil document", func(t *testing.T) {
		_, err := gen.GenerateTransitionID(nil)
		if err == nil {
			t.Error("Expected error for nil document")
		}
	})
}

func TestIDGenerator_GenerateDeviceSnapshotID(t *testing.T) {
	t.Parallel()

	gen := NewIDGenerator()

	t.Run("generates deterministic ID", func(t *testing.T) {
		collectedAt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

		doc := &DeviceSnapshot{
			Type:           "device_snapshot",
			CollectedAt:    collectedAt,
			ThermostatID:   "test-123",
			ThermostatName: "Living Room",
		}

		id1, err := gen.GenerateDeviceSnapshotID(doc)
		if err != nil {
			t.Fatalf("Failed to generate ID: %v", err)
		}

		id2, err := gen.GenerateDeviceSnapshotID(doc)
		if err != nil {
			t.Fatalf("Failed to generate ID: %v", err)
		}

		if id1 != id2 {
			t.Errorf("IDs should be deterministic: %s != %s", id1, id2)
		}

		expected := "test-123:2024-01-15T10:30:00Z"
		if id1 != expected {
			t.Errorf("Expected ID %s, got %s", expected, id1)
		}
	})

	t.Run("handles nil document", func(t *testing.T) {
		_, err := gen.GenerateDeviceSnapshotID(nil)
		if err == nil {
			t.Error("Expected error for nil document")
		}
	})
}
