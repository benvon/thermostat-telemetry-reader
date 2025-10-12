package core

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestSQLiteOffsetStore(t *testing.T) {
	t.Parallel()

	// Create a temporary database file
	tmpFile, err := os.CreateTemp("", "offset_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Create the offset store
	store, err := NewSQLiteOffsetStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create offset store: %v", err)
	}
	defer func() {
		_ = store.Close()
	}()

	ctx := context.Background()
	thermostatID := "test-thermostat-123"

	t.Run("GetLastRuntimeTime returns zero time when not set", func(t *testing.T) {
		ts, err := store.GetLastRuntimeTime(ctx, thermostatID)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !ts.IsZero() {
			t.Errorf("Expected zero time, got %v", ts)
		}
	})

	t.Run("SetLastRuntimeTime and GetLastRuntimeTime", func(t *testing.T) {
		expectedTime := time.Now().UTC().Truncate(time.Second)

		err := store.SetLastRuntimeTime(ctx, thermostatID, expectedTime)
		if err != nil {
			t.Fatalf("Failed to set runtime time: %v", err)
		}

		retrievedTime, err := store.GetLastRuntimeTime(ctx, thermostatID)
		if err != nil {
			t.Fatalf("Failed to get runtime time: %v", err)
		}

		if !retrievedTime.Equal(expectedTime) {
			t.Errorf("Expected %v, got %v", expectedTime, retrievedTime)
		}
	})

	t.Run("GetLastSnapshotTime returns zero time when not set", func(t *testing.T) {
		ts, err := store.GetLastSnapshotTime(ctx, "another-thermostat")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !ts.IsZero() {
			t.Errorf("Expected zero time, got %v", ts)
		}
	})

	t.Run("SetLastSnapshotTime and GetLastSnapshotTime", func(t *testing.T) {
		expectedTime := time.Now().UTC().Truncate(time.Second)

		err := store.SetLastSnapshotTime(ctx, thermostatID, expectedTime)
		if err != nil {
			t.Fatalf("Failed to set snapshot time: %v", err)
		}

		retrievedTime, err := store.GetLastSnapshotTime(ctx, thermostatID)
		if err != nil {
			t.Fatalf("Failed to get snapshot time: %v", err)
		}

		if !retrievedTime.Equal(expectedTime) {
			t.Errorf("Expected %v, got %v", expectedTime, retrievedTime)
		}
	})

	t.Run("Update existing runtime time", func(t *testing.T) {
		firstTime := time.Now().UTC().Truncate(time.Second)
		secondTime := firstTime.Add(10 * time.Minute)

		err := store.SetLastRuntimeTime(ctx, thermostatID, firstTime)
		if err != nil {
			t.Fatalf("Failed to set first time: %v", err)
		}

		err = store.SetLastRuntimeTime(ctx, thermostatID, secondTime)
		if err != nil {
			t.Fatalf("Failed to set second time: %v", err)
		}

		retrievedTime, err := store.GetLastRuntimeTime(ctx, thermostatID)
		if err != nil {
			t.Fatalf("Failed to get runtime time: %v", err)
		}

		if !retrievedTime.Equal(secondTime) {
			t.Errorf("Expected %v, got %v", secondTime, retrievedTime)
		}
	})

	t.Run("Multiple thermostats", func(t *testing.T) {
		id1 := "thermostat-1"
		id2 := "thermostat-2"
		time1 := time.Now().UTC().Truncate(time.Second)
		time2 := time1.Add(1 * time.Hour)

		err := store.SetLastRuntimeTime(ctx, id1, time1)
		if err != nil {
			t.Fatalf("Failed to set time for id1: %v", err)
		}

		err = store.SetLastRuntimeTime(ctx, id2, time2)
		if err != nil {
			t.Fatalf("Failed to set time for id2: %v", err)
		}

		retrieved1, err := store.GetLastRuntimeTime(ctx, id1)
		if err != nil {
			t.Fatalf("Failed to get time for id1: %v", err)
		}

		retrieved2, err := store.GetLastRuntimeTime(ctx, id2)
		if err != nil {
			t.Fatalf("Failed to get time for id2: %v", err)
		}

		if !retrieved1.Equal(time1) {
			t.Errorf("Expected %v for id1, got %v", time1, retrieved1)
		}

		if !retrieved2.Equal(time2) {
			t.Errorf("Expected %v for id2, got %v", time2, retrieved2)
		}
	})
}
