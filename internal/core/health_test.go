package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/benvon/thermostat-telemetry-reader/pkg/model"
)

func TestMetricsCollector(t *testing.T) {
	t.Run("provider metrics", func(t *testing.T) {
		metrics := NewMetricsCollector()

		// Initially should have no metrics
		initialMetrics := metrics.GetMetrics()
		providers := initialMetrics["providers"].(map[string]any)
		if len(providers) != 0 {
			t.Errorf("Expected no providers initially, got %d", len(providers))
		}

		// Record some provider requests
		metrics.RecordProviderRequest("ecobee")
		metrics.RecordProviderRequest("ecobee")
		metrics.RecordProviderRequest("nest")

		// Verify counts
		currentMetrics := metrics.GetMetrics()
		providers = currentMetrics["providers"].(map[string]any)

		if len(providers) != 2 {
			t.Errorf("Expected 2 providers, got %d", len(providers))
		}

		ecobeeMetrics := providers["ecobee"].(map[string]any)
		if ecobeeMetrics["requests_total"] != int64(2) {
			t.Errorf("Expected 2 ecobee requests, got %v", ecobeeMetrics["requests_total"])
		}

		nestMetrics := providers["nest"].(map[string]any)
		if nestMetrics["requests_total"] != int64(1) {
			t.Errorf("Expected 1 nest request, got %v", nestMetrics["requests_total"])
		}
	})

	t.Run("provider errors", func(t *testing.T) {
		metrics := NewMetricsCollector()

		metrics.RecordProviderRequest("ecobee")
		metrics.RecordProviderRequest("ecobee")
		metrics.RecordProviderError("ecobee")

		currentMetrics := metrics.GetMetrics()
		providers := currentMetrics["providers"].(map[string]any)
		ecobeeMetrics := providers["ecobee"].(map[string]any)

		if ecobeeMetrics["requests_total"] != int64(2) {
			t.Errorf("Expected 2 requests, got %v", ecobeeMetrics["requests_total"])
		}
		if ecobeeMetrics["errors_total"] != int64(1) {
			t.Errorf("Expected 1 error, got %v", ecobeeMetrics["errors_total"])
		}
	})

	t.Run("sink metrics", func(t *testing.T) {
		metrics := NewMetricsCollector()

		// Record some sink writes
		metrics.RecordSinkWrite("elasticsearch", 10)
		metrics.RecordSinkWrite("elasticsearch", 5)
		metrics.RecordSinkWrite("prometheus", 3)

		currentMetrics := metrics.GetMetrics()
		sinks := currentMetrics["sinks"].(map[string]any)

		if len(sinks) != 2 {
			t.Errorf("Expected 2 sinks, got %d", len(sinks))
		}

		esMetrics := sinks["elasticsearch"].(map[string]any)
		if esMetrics["writes_total"] != int64(2) {
			t.Errorf("Expected 2 elasticsearch writes, got %v", esMetrics["writes_total"])
		}
		if esMetrics["documents_written"] != int64(15) {
			t.Errorf("Expected 15 documents written, got %v", esMetrics["documents_written"])
		}

		promMetrics := sinks["prometheus"].(map[string]any)
		if promMetrics["writes_total"] != int64(1) {
			t.Errorf("Expected 1 prometheus write, got %v", promMetrics["writes_total"])
		}
		if promMetrics["documents_written"] != int64(3) {
			t.Errorf("Expected 3 documents written, got %v", promMetrics["documents_written"])
		}
	})

	t.Run("sink errors", func(t *testing.T) {
		metrics := NewMetricsCollector()

		metrics.RecordSinkWrite("elasticsearch", 5)
		metrics.RecordSinkError("elasticsearch")
		metrics.RecordSinkError("elasticsearch")

		currentMetrics := metrics.GetMetrics()
		sinks := currentMetrics["sinks"].(map[string]any)
		esMetrics := sinks["elasticsearch"].(map[string]any)

		if esMetrics["writes_total"] != int64(1) {
			t.Errorf("Expected 1 write, got %v", esMetrics["writes_total"])
		}
		if esMetrics["errors_total"] != int64(2) {
			t.Errorf("Expected 2 errors, got %v", esMetrics["errors_total"])
		}
	})

	t.Run("uptime calculation", func(t *testing.T) {
		metrics := NewMetricsCollector()

		// Sleep a tiny bit to ensure uptime > 0
		time.Sleep(10 * time.Millisecond)

		currentMetrics := metrics.GetMetrics()
		uptime := currentMetrics["uptime_seconds"].(float64)

		if uptime <= 0 {
			t.Errorf("Expected positive uptime, got %f", uptime)
		}
	})

	t.Run("concurrent access", func(t *testing.T) {
		metrics := NewMetricsCollector()

		// Simulate concurrent access
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func() {
				metrics.RecordProviderRequest("ecobee")
				metrics.RecordProviderError("ecobee")
				metrics.RecordSinkWrite("elasticsearch", 1)
				metrics.RecordSinkError("elasticsearch")
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		// Verify counts
		currentMetrics := metrics.GetMetrics()
		providers := currentMetrics["providers"].(map[string]any)
		ecobeeMetrics := providers["ecobee"].(map[string]any)

		if ecobeeMetrics["requests_total"] != int64(10) {
			t.Errorf("Expected 10 requests, got %v", ecobeeMetrics["requests_total"])
		}
		if ecobeeMetrics["errors_total"] != int64(10) {
			t.Errorf("Expected 10 errors, got %v", ecobeeMetrics["errors_total"])
		}

		sinks := currentMetrics["sinks"].(map[string]any)
		esMetrics := sinks["elasticsearch"].(map[string]any)

		if esMetrics["writes_total"] != int64(10) {
			t.Errorf("Expected 10 writes, got %v", esMetrics["writes_total"])
		}
		if esMetrics["errors_total"] != int64(10) {
			t.Errorf("Expected 10 errors, got %v", esMetrics["errors_total"])
		}
	})
}

func TestNewMetricsCollector(t *testing.T) {
	metrics := NewMetricsCollector()

	if metrics.providerRequests == nil {
		t.Error("providerRequests map should be initialized")
	}
	if metrics.providerErrors == nil {
		t.Error("providerErrors map should be initialized")
	}
	if metrics.providerLastRequest == nil {
		t.Error("providerLastRequest map should be initialized")
	}
	if metrics.sinkWrites == nil {
		t.Error("sinkWrites map should be initialized")
	}
	if metrics.sinkErrors == nil {
		t.Error("sinkErrors map should be initialized")
	}
	if metrics.sinkLastWrite == nil {
		t.Error("sinkLastWrite map should be initialized")
	}
	if metrics.sinkDocumentsWritten == nil {
		t.Error("sinkDocumentsWritten map should be initialized")
	}
	if metrics.startTime.IsZero() {
		t.Error("startTime should be set")
	}
}

// Mock implementations for testing

type mockProvider struct {
	name         string
	shouldFail   bool
	tokenValid   bool
	refreshFails bool
}

func (m *mockProvider) Info() model.ProviderInfo {
	return model.ProviderInfo{
		Name:        m.name,
		Version:     "test-1.0",
		Description: "Mock provider for testing",
	}
}

func (m *mockProvider) ListThermostats(ctx context.Context) ([]model.ThermostatRef, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("mock provider error")
	}
	return []model.ThermostatRef{
		{ID: "therm-1", Name: "Test", Provider: m.name},
	}, nil
}

func (m *mockProvider) GetSummary(ctx context.Context, tr model.ThermostatRef) (model.Summary, error) {
	if m.shouldFail {
		return model.Summary{}, fmt.Errorf("mock error")
	}
	return model.Summary{}, nil
}

func (m *mockProvider) GetSnapshot(ctx context.Context, tr model.ThermostatRef, since time.Time) (model.Snapshot, error) {
	if m.shouldFail {
		return model.Snapshot{}, fmt.Errorf("mock error")
	}
	return model.Snapshot{}, nil
}

func (m *mockProvider) GetRuntime(ctx context.Context, tr model.ThermostatRef, from, to time.Time) ([]model.RuntimeRow, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("mock error")
	}
	return []model.RuntimeRow{}, nil
}

func (m *mockProvider) Auth() model.AuthManager {
	return &mockAuth{
		valid:        m.tokenValid,
		refreshFails: m.refreshFails,
	}
}

type mockAuth struct {
	valid        bool
	refreshFails bool
}

func (a *mockAuth) RefreshToken(ctx context.Context) error {
	if a.refreshFails {
		return fmt.Errorf("mock refresh error")
	}
	a.valid = true
	return nil
}

func (a *mockAuth) GetAccessToken(ctx context.Context) (string, error) {
	return "mock-token", nil
}

func (a *mockAuth) IsTokenValid(ctx context.Context) bool {
	return a.valid
}

type mockSink struct {
	name       string
	shouldFail bool
}

func (s *mockSink) Info() model.SinkInfo {
	return model.SinkInfo{
		Name:        s.name,
		Version:     "test-1.0",
		Description: "Mock sink for testing",
	}
}

func (s *mockSink) Open(ctx context.Context) error {
	if s.shouldFail {
		return fmt.Errorf("mock sink connection error")
	}
	return nil
}

func (s *mockSink) Write(ctx context.Context, docs []model.Doc) (model.WriteResult, error) {
	if s.shouldFail {
		return model.WriteResult{}, fmt.Errorf("mock write error")
	}
	return model.WriteResult{SuccessCount: len(docs)}, nil
}

func (s *mockSink) Close(ctx context.Context) error {
	return nil
}

// Health checker tests

func TestNewHealthChecker(t *testing.T) {
	provider := &mockProvider{name: "test-provider", tokenValid: true}
	sink := &mockSink{name: "test-sink"}

	checker := NewHealthChecker([]model.Provider{provider}, []model.Sink{sink})

	if checker == nil {
		t.Fatal("Expected non-nil health checker")
	}

	if len(checker.providers) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(checker.providers))
	}

	if len(checker.sinks) != 1 {
		t.Errorf("Expected 1 sink, got %d", len(checker.sinks))
	}

	if checker.status.Status != "healthy" {
		t.Errorf("Expected initial status 'healthy', got %s", checker.status.Status)
	}
}

func TestCheckHealth(t *testing.T) {
	t.Run("all healthy", func(t *testing.T) {
		provider := &mockProvider{name: "ecobee", tokenValid: true}
		sink := &mockSink{name: "elasticsearch"}

		checker := NewHealthChecker([]model.Provider{provider}, []model.Sink{sink})
		ctx := context.Background()

		status := checker.CheckHealth(ctx)

		if status.Status != "healthy" {
			t.Errorf("Expected status 'healthy', got %s", status.Status)
		}

		if len(status.Checks) != 2 {
			t.Errorf("Expected 2 checks, got %d", len(status.Checks))
		}

		providerCheck := status.Checks["provider_ecobee"]
		if providerCheck.Status != "pass" {
			t.Errorf("Expected provider check to pass, got %s", providerCheck.Status)
		}

		sinkCheck := status.Checks["sink_elasticsearch"]
		if sinkCheck.Status != "pass" {
			t.Errorf("Expected sink check to pass, got %s", sinkCheck.Status)
		}
	})

	t.Run("provider auth fails", func(t *testing.T) {
		provider := &mockProvider{name: "ecobee", tokenValid: false, refreshFails: true}
		sink := &mockSink{name: "elasticsearch"}

		checker := NewHealthChecker([]model.Provider{provider}, []model.Sink{sink})
		ctx := context.Background()

		status := checker.CheckHealth(ctx)

		if status.Status != "unhealthy" {
			t.Errorf("Expected status 'unhealthy', got %s", status.Status)
		}

		providerCheck := status.Checks["provider_ecobee"]
		if providerCheck.Status != "fail" {
			t.Errorf("Expected provider check to fail, got %s", providerCheck.Status)
		}
	})

	t.Run("provider connectivity warn", func(t *testing.T) {
		provider := &mockProvider{name: "ecobee", tokenValid: true, shouldFail: true}
		sink := &mockSink{name: "elasticsearch"}

		checker := NewHealthChecker([]model.Provider{provider}, []model.Sink{sink})
		ctx := context.Background()

		status := checker.CheckHealth(ctx)

		if status.Status != "degraded" {
			t.Errorf("Expected status 'degraded', got %s", status.Status)
		}

		providerCheck := status.Checks["provider_ecobee"]
		if providerCheck.Status != "warn" {
			t.Errorf("Expected provider check to warn, got %s", providerCheck.Status)
		}
	})

	t.Run("sink fails", func(t *testing.T) {
		provider := &mockProvider{name: "ecobee", tokenValid: true}
		sink := &mockSink{name: "elasticsearch", shouldFail: true}

		checker := NewHealthChecker([]model.Provider{provider}, []model.Sink{sink})
		ctx := context.Background()

		status := checker.CheckHealth(ctx)

		if status.Status != "unhealthy" {
			t.Errorf("Expected status 'unhealthy', got %s", status.Status)
		}

		sinkCheck := status.Checks["sink_elasticsearch"]
		if sinkCheck.Status != "fail" {
			t.Errorf("Expected sink check to fail, got %s", sinkCheck.Status)
		}
	})

	t.Run("multiple providers and sinks", func(t *testing.T) {
		providers := []model.Provider{
			&mockProvider{name: "ecobee", tokenValid: true},
			&mockProvider{name: "nest", tokenValid: true},
		}
		sinks := []model.Sink{
			&mockSink{name: "elasticsearch"},
			&mockSink{name: "prometheus"},
		}

		checker := NewHealthChecker(providers, sinks)
		ctx := context.Background()

		status := checker.CheckHealth(ctx)

		if status.Status != "healthy" {
			t.Errorf("Expected status 'healthy', got %s", status.Status)
		}

		if len(status.Checks) != 4 {
			t.Errorf("Expected 4 checks (2 providers + 2 sinks), got %d", len(status.Checks))
		}
	})
}

func TestGetStatus(t *testing.T) {
	provider := &mockProvider{name: "ecobee", tokenValid: true}
	sink := &mockSink{name: "elasticsearch"}

	checker := NewHealthChecker([]model.Provider{provider}, []model.Sink{sink})
	ctx := context.Background()

	// Update status by running a check
	checker.CheckHealth(ctx)

	// Get cached status
	status := checker.GetStatus()

	if status.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got %s", status.Status)
	}

	if len(status.Checks) != 2 {
		t.Errorf("Expected 2 checks in cached status, got %d", len(status.Checks))
	}
}
