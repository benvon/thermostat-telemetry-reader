package core

import (
	"context"
	"encoding/json"
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
	Status      string `json:"status"` // "pass", "fail", "warn"
	Message     string `json:"message,omitempty"`
	DurationMS  int64  `json:"duration_ms"`
	LastChecked string `json:"last_checked"`
}

// newCheckResult creates a CheckResult with proper formatting
func newCheckResult(status, message string, duration time.Duration) CheckResult {
	return CheckResult{
		Status:      status,
		Message:     message,
		DurationMS:  duration.Milliseconds(),
		LastChecked: time.Now().Format(time.RFC3339),
	}
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
			return newCheckResult("fail", fmt.Sprintf("Authentication failed: %v", err), time.Since(start))
		}
	}

	// Test basic connectivity by listing thermostats
	_, err := provider.ListThermostats(ctx)
	if err != nil {
		return newCheckResult("warn", fmt.Sprintf("Provider connectivity issue: %v", err), time.Since(start))
	}

	return newCheckResult("pass", "Provider is healthy", time.Since(start))
}

// checkSink performs a health check on a sink
func (h *HealthChecker) checkSink(ctx context.Context, sink model.Sink) CheckResult {
	start := time.Now()

	// Create a short-lived context for the health check
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Test sink connectivity by attempting to open it
	if err := sink.Open(checkCtx); err != nil {
		return newCheckResult("fail", fmt.Sprintf("Sink connectivity failed: %v", err), time.Since(start))
	}

	return newCheckResult("pass", "Sink is healthy", time.Since(start))
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

		// Encode response as JSON
		if err := json.NewEncoder(w).Encode(status); err != nil {
			// Log error but don't change status code (already written)
			errorResp := HealthStatus{
				Status:    "unhealthy",
				Timestamp: time.Now(),
				Checks:    map[string]CheckResult{},
			}
			_ = json.NewEncoder(w).Encode(errorResp)
		}
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

// Metrics represents the overall metrics structure
type Metrics struct {
	UptimeSeconds float64                    `json:"uptime_seconds"`
	Providers     map[string]ProviderMetrics `json:"providers"`
	Sinks         map[string]SinkMetrics     `json:"sinks"`
}

// ProviderMetrics represents metrics for a provider
type ProviderMetrics struct {
	RequestsTotal   int64  `json:"requests_total"`
	ErrorsTotal     int64  `json:"errors_total"`
	LastRequestTime string `json:"last_request_time"`
}

// SinkMetrics represents metrics for a sink
type SinkMetrics struct {
	WritesTotal      int64  `json:"writes_total"`
	ErrorsTotal      int64  `json:"errors_total"`
	DocumentsWritten int64  `json:"documents_written"`
	LastWriteTime    string `json:"last_write_time"`
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
func (m *MetricsCollector) GetMetrics() Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics := Metrics{
		UptimeSeconds: time.Since(m.startTime).Seconds(),
		Providers:     make(map[string]ProviderMetrics),
		Sinks:         make(map[string]SinkMetrics),
	}

	// Provider metrics
	for name, requests := range m.providerRequests {
		metrics.Providers[name] = ProviderMetrics{
			RequestsTotal:   requests,
			ErrorsTotal:     m.providerErrors[name],
			LastRequestTime: m.providerLastRequest[name].Format(time.RFC3339),
		}
	}

	// Sink metrics
	for name, writes := range m.sinkWrites {
		metrics.Sinks[name] = SinkMetrics{
			WritesTotal:      writes,
			ErrorsTotal:      m.sinkErrors[name],
			DocumentsWritten: m.sinkDocumentsWritten[name],
			LastWriteTime:    m.sinkLastWrite[name].Format(time.RFC3339),
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

		// Encode response as JSON
		if err := json.NewEncoder(w).Encode(metrics); err != nil {
			// Log error but don't change status code (already written)
			errorResp := struct {
				Error string `json:"error"`
			}{
				Error: "failed to encode metrics",
			}
			_ = json.NewEncoder(w).Encode(errorResp)
		}
	})
}
