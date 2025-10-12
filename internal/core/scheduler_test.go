package core

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/benvon/thermostat-telemetry-reader/pkg/model"
)

func TestMemoryOffsetStore(t *testing.T) {
	t.Run("runtime time operations", func(t *testing.T) {
		store := NewMemoryOffsetStore()
		ctx := testContext(t)

		thermostatID := "test-therm-001"
		testTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

		// Initially should return zero time
		lastTime, err := store.GetLastRuntimeTime(ctx, thermostatID)
		if err != nil {
			t.Fatalf("GetLastRuntimeTime failed: %v", err)
		}
		if !lastTime.IsZero() {
			t.Errorf("Expected zero time initially, got %v", lastTime)
		}

		// Set a time
		err = store.SetLastRuntimeTime(ctx, thermostatID, testTime)
		if err != nil {
			t.Fatalf("SetLastRuntimeTime failed: %v", err)
		}

		// Verify we can retrieve it
		lastTime, err = store.GetLastRuntimeTime(ctx, thermostatID)
		if err != nil {
			t.Fatalf("GetLastRuntimeTime after set failed: %v", err)
		}
		if !lastTime.Equal(testTime) {
			t.Errorf("Expected %v, got %v", testTime, lastTime)
		}

		// Update to a newer time
		newerTime := testTime.Add(5 * time.Minute)
		err = store.SetLastRuntimeTime(ctx, thermostatID, newerTime)
		if err != nil {
			t.Fatalf("SetLastRuntimeTime update failed: %v", err)
		}

		lastTime, err = store.GetLastRuntimeTime(ctx, thermostatID)
		if err != nil {
			t.Fatalf("GetLastRuntimeTime after update failed: %v", err)
		}
		if !lastTime.Equal(newerTime) {
			t.Errorf("Expected %v, got %v", newerTime, lastTime)
		}
	})

	t.Run("snapshot time operations", func(t *testing.T) {
		store := NewMemoryOffsetStore()
		ctx := testContext(t)

		thermostatID := "test-therm-002"
		testTime := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)

		// Initially should return zero time
		lastTime, err := store.GetLastSnapshotTime(ctx, thermostatID)
		if err != nil {
			t.Fatalf("GetLastSnapshotTime failed: %v", err)
		}
		if !lastTime.IsZero() {
			t.Errorf("Expected zero time initially, got %v", lastTime)
		}

		// Set a time
		err = store.SetLastSnapshotTime(ctx, thermostatID, testTime)
		if err != nil {
			t.Fatalf("SetLastSnapshotTime failed: %v", err)
		}

		// Verify we can retrieve it
		lastTime, err = store.GetLastSnapshotTime(ctx, thermostatID)
		if err != nil {
			t.Fatalf("GetLastSnapshotTime after set failed: %v", err)
		}
		if !lastTime.Equal(testTime) {
			t.Errorf("Expected %v, got %v", testTime, lastTime)
		}
	})

	t.Run("multiple thermostats", func(t *testing.T) {
		store := NewMemoryOffsetStore()
		ctx := testContext(t)

		therm1 := "therm-001"
		therm2 := "therm-002"
		time1 := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		time2 := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)

		// Set different times for different thermostats
		_ = store.SetLastRuntimeTime(ctx, therm1, time1)
		_ = store.SetLastRuntimeTime(ctx, therm2, time2)

		// Verify they're stored independently
		result1, _ := store.GetLastRuntimeTime(ctx, therm1)
		result2, _ := store.GetLastRuntimeTime(ctx, therm2)

		if !result1.Equal(time1) {
			t.Errorf("Therm1: expected %v, got %v", time1, result1)
		}
		if !result2.Equal(time2) {
			t.Errorf("Therm2: expected %v, got %v", time2, result2)
		}
	})
}

func TestNewScheduler(t *testing.T) {
	// Use mock implementations from health_test.go (same package)
	provider := &mockProvider{name: "ecobee", tokenValid: true}
	sink := &mockSink{name: "elasticsearch"}

	normalizer, err := NewNormalizer("UTC")
	if err != nil {
		t.Fatalf("Failed to create normalizer: %v", err)
	}

	offsetStore := NewMemoryOffsetStore()
	metrics := NewMetricsCollector()
	logger := slog.Default()

	scheduler := NewScheduler(
		[]model.Provider{provider},
		[]model.Sink{sink},
		normalizer,
		offsetStore,
		5*time.Minute,
		24*time.Hour,
		metrics,
		logger,
	)

	if scheduler == nil {
		t.Fatal("Expected non-nil scheduler")
	}

	if len(scheduler.providers) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(scheduler.providers))
	}

	if len(scheduler.sinks) != 1 {
		t.Errorf("Expected 1 sink, got %d", len(scheduler.sinks))
	}

	if scheduler.pollInterval != 5*time.Minute {
		t.Errorf("Expected 5m poll interval, got %v", scheduler.pollInterval)
	}

	if scheduler.backfillWindow != 24*time.Hour {
		t.Errorf("Expected 24h backfill window, got %v", scheduler.backfillWindow)
	}

	if scheduler.metrics == nil {
		t.Error("Expected non-nil metrics collector")
	}

	if scheduler.idGenerator == nil {
		t.Error("Expected non-nil ID generator")
	}
}

// Helper function
func testContext(_ *testing.T) context.Context {
	return context.Background()
}
