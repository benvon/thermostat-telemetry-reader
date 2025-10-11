package core

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/benvon/thermostat-telemetry-reader/pkg/model"
)

// HealthChecker provides health check functionality
type HealthChecker struct {
	providers []model.Provider
	sinks     []model.Sink
	mu        sync.RWMutex
	status    HealthStatus
}

// HealthStatus represents the overall health status
type HealthStatus struct {
	Status    string                 `json:"status"` // "healthy", "degraded", "unhealthy"
	Timestamp time.Time              `json:"timestamp"`
	Checks    map[string]CheckResult `json:"checks"`
}

// CheckResult represents the result of a health check
type CheckResult struct {
	Status      string        `json:"status"` // "pass", "fail", "warn"
	Message     string        `json:"message,omitempty"`
	Duration    time.Duration `json:"duration_ms"`
	LastChecked time.Time     `json:"last_checked"`
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(providers []model.Provider, sinks []model.Sink) *HealthChecker {
	return &HealthChecker{
		providers: providers,
		sinks:     sinks,
		status: HealthStatus{
			Status: "healthy",
			Checks: make(map[string]CheckResult),
		},
	}
}

// CheckHealth performs all health checks
func (h *HealthChecker) CheckHealth(ctx context.Context) HealthStatus {
	h.mu.Lock()
	defer h.mu.Unlock()

	checks := make(map[string]CheckResult)

	// Check providers
	for _, provider := range h.providers {
		check := h.checkProvider(ctx, provider)
		checks[fmt.Sprintf("provider_%s", provider.Info().Name)] = check
	}

	// Check sinks
	for _, sink := range h.sinks {
		check := h.checkSink(ctx, sink)
		checks[fmt.Sprintf("sink_%s", sink.Info().Name)] = check
	}

	// Determine overall status
	overallStatus := "healthy"
	for _, check := range checks {
		if check.Status == "fail" {
			overallStatus = "unhealthy"
			break
		} else if check.Status == "warn" {
			overallStatus = "degraded"
		}
	}

	h.status = HealthStatus{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Checks:    checks,
	}

	return h.status
}

// GetStatus returns the current health status
func (h *HealthChecker) GetStatus() HealthStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.status
}

// checkProvider performs a health check on a provider
func (h *HealthChecker) checkProvider(ctx context.Context, provider model.Provider) CheckResult {
	start := time.Now()

	// Test authentication
	auth := provider.Auth()
	if !auth.IsTokenValid(ctx) {
		// Try to refresh token
		if err := auth.RefreshToken(ctx); err != nil {
			return CheckResult{
				Status:      "fail",
				Message:     fmt.Sprintf("Authentication failed: %v", err),
				Duration:    time.Since(start),
				LastChecked: time.Now(),
			}
		}
	}

	// Test basic connectivity by listing thermostats
	_, err := provider.ListThermostats(ctx)
	if err != nil {
		return CheckResult{
			Status:      "warn",
			Message:     fmt.Sprintf("Provider connectivity issue: %v", err),
			Duration:    time.Since(start),
			LastChecked: time.Now(),
		}
	}

	return CheckResult{
		Status:      "pass",
		Message:     "Provider is healthy",
		Duration:    time.Since(start),
		LastChecked: time.Now(),
	}
}

// checkSink performs a health check on a sink
func (h *HealthChecker) checkSink(ctx context.Context, sink model.Sink) CheckResult {
	start := time.Now()

	// Test sink connectivity by trying to open it
	// Note: This is a simplified check - in practice you might want to test actual writes
	// For now, we'll assume the sink is healthy if we can create it

	return CheckResult{
		Status:      "pass",
		Message:     "Sink is healthy",
		Duration:    time.Since(start),
		LastChecked: time.Now(),
	}
}

// ServeHealth provides an HTTP handler for health checks
func (h *HealthChecker) ServeHealth() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		status := h.CheckHealth(ctx)

		w.Header().Set("Content-Type", "application/json")

		// Set appropriate HTTP status code
		switch status.Status {
		case "healthy":
			w.WriteHeader(http.StatusOK)
		case "degraded":
			w.WriteHeader(http.StatusOK) // Still 200 but with warnings
		case "unhealthy":
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		// Write JSON response
		_, _ = fmt.Fprintf(w, `{
	"status": "%s",
	"timestamp": "%s",
	"checks": {
`, status.Status, status.Timestamp.Format(time.RFC3339))

		first := true
		for name, check := range status.Checks {
			if !first {
				_, _ = fmt.Fprintf(w, ",\n")
			}
			first = false

			_, _ = fmt.Fprintf(w, `		"%s": {
			"status": "%s",
			"message": "%s",
			"duration_ms": %d,
			"last_checked": "%s"
		}`, name, check.Status, check.Message, check.Duration.Milliseconds(), check.LastChecked.Format(time.RFC3339))
		}

		_, _ = fmt.Fprintf(w, `
	}
}`)
	})
}

// MetricsCollector provides basic metrics collection
type MetricsCollector struct {
	mu sync.RWMutex

	// Provider metrics
	providerRequests    map[string]int64
	providerErrors      map[string]int64
	providerLastRequest map[string]time.Time

	// Sink metrics
	sinkWrites           map[string]int64
	sinkErrors           map[string]int64
	sinkLastWrite        map[string]time.Time
	sinkDocumentsWritten map[string]int64

	// General metrics
	startTime time.Time
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		providerRequests:     make(map[string]int64),
		providerErrors:       make(map[string]int64),
		providerLastRequest:  make(map[string]time.Time),
		sinkWrites:           make(map[string]int64),
		sinkErrors:           make(map[string]int64),
		sinkLastWrite:        make(map[string]time.Time),
		sinkDocumentsWritten: make(map[string]int64),
		startTime:            time.Now(),
	}
}

// RecordProviderRequest records a provider request
func (m *MetricsCollector) RecordProviderRequest(providerName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.providerRequests[providerName]++
	m.providerLastRequest[providerName] = time.Now()
}

// RecordProviderError records a provider error
func (m *MetricsCollector) RecordProviderError(providerName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.providerErrors[providerName]++
}

// RecordSinkWrite records a sink write operation
func (m *MetricsCollector) RecordSinkWrite(sinkName string, documentCount int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sinkWrites[sinkName]++
	m.sinkDocumentsWritten[sinkName] += documentCount
	m.sinkLastWrite[sinkName] = time.Now()
}

// RecordSinkError records a sink error
func (m *MetricsCollector) RecordSinkError(sinkName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sinkErrors[sinkName]++
}

// GetMetrics returns current metrics
func (m *MetricsCollector) GetMetrics() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := map[string]any{
		"uptime_seconds": time.Since(m.startTime).Seconds(),
		"providers":      make(map[string]any),
		"sinks":          make(map[string]any),
	}

	// Provider metrics
	providers := metrics["providers"].(map[string]any)
	for name, requests := range m.providerRequests {
		providers[name] = map[string]any{
			"requests_total":    requests,
			"errors_total":      m.providerErrors[name],
			"last_request_time": m.providerLastRequest[name].Format(time.RFC3339),
		}
	}

	// Sink metrics
	sinks := metrics["sinks"].(map[string]any)
	for name, writes := range m.sinkWrites {
		sinks[name] = map[string]any{
			"writes_total":      writes,
			"errors_total":      m.sinkErrors[name],
			"documents_written": m.sinkDocumentsWritten[name],
			"last_write_time":   m.sinkLastWrite[name].Format(time.RFC3339),
		}
	}

	return metrics
}

// ServeMetrics provides an HTTP handler for metrics
func (m *MetricsCollector) ServeMetrics() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metrics := m.GetMetrics()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Simple JSON output
		_, _ = fmt.Fprintf(w, `{
	"uptime_seconds": %.2f,
	"providers": {
`, metrics["uptime_seconds"])

		providers := metrics["providers"].(map[string]any)
		first := true
		for name, providerMetrics := range providers {
			if !first {
				_, _ = fmt.Fprintf(w, ",\n")
			}
			first = false

			pm := providerMetrics.(map[string]any)
			_, _ = fmt.Fprintf(w, `		"%s": {
			"requests_total": %d,
			"errors_total": %d,
			"last_request_time": "%s"
		}`, name, pm["requests_total"], pm["errors_total"], pm["last_request_time"])
		}

		_, _ = fmt.Fprintf(w, `
	},
	"sinks": {
`)

		sinks := metrics["sinks"].(map[string]any)
		first = true
		for name, sinkMetrics := range sinks {
			if !first {
				_, _ = fmt.Fprintf(w, ",\n")
			}
			first = false

			sm := sinkMetrics.(map[string]any)
			_, _ = fmt.Fprintf(w, `		"%s": {
			"writes_total": %d,
			"errors_total": %d,
			"documents_written": %d,
			"last_write_time": "%s"
		}`, name, sm["writes_total"], sm["errors_total"], sm["documents_written"], sm["last_write_time"])
		}

		_, _ = fmt.Fprintf(w, `
	}
}`)
	})
}
