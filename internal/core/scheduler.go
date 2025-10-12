package core

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/benvon/thermostat-telemetry-reader/pkg/model"
)

// OffsetStore manages persistence of polling offsets
type OffsetStore interface {
	// GetLastRuntimeTime returns the last runtime timestamp for a thermostat
	GetLastRuntimeTime(ctx context.Context, thermostatID string) (time.Time, error)

	// SetLastRuntimeTime sets the last runtime timestamp for a thermostat
	SetLastRuntimeTime(ctx context.Context, thermostatID string, timestamp time.Time) error

	// GetLastSnapshotTime returns the last snapshot timestamp for a thermostat
	GetLastSnapshotTime(ctx context.Context, thermostatID string) (time.Time, error)

	// SetLastSnapshotTime sets the last snapshot timestamp for a thermostat
	SetLastSnapshotTime(ctx context.Context, thermostatID string, timestamp time.Time) error
}

// MemoryOffsetStore is an in-memory implementation of OffsetStore for testing
type MemoryOffsetStore struct {
	mu                sync.RWMutex
	lastRuntimeTimes  map[string]time.Time
	lastSnapshotTimes map[string]time.Time
}

// NewMemoryOffsetStore creates a new in-memory offset store
func NewMemoryOffsetStore() *MemoryOffsetStore {
	return &MemoryOffsetStore{
		lastRuntimeTimes:  make(map[string]time.Time),
		lastSnapshotTimes: make(map[string]time.Time),
	}
}

// GetLastRuntimeTime returns the last runtime timestamp for a thermostat
func (s *MemoryOffsetStore) GetLastRuntimeTime(ctx context.Context, thermostatID string) (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastRuntimeTimes[thermostatID], nil
}

// SetLastRuntimeTime sets the last runtime timestamp for a thermostat
func (s *MemoryOffsetStore) SetLastRuntimeTime(ctx context.Context, thermostatID string, timestamp time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastRuntimeTimes[thermostatID] = timestamp
	return nil
}

// GetLastSnapshotTime returns the last snapshot timestamp for a thermostat
func (s *MemoryOffsetStore) GetLastSnapshotTime(ctx context.Context, thermostatID string) (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSnapshotTimes[thermostatID], nil
}

// SetLastSnapshotTime sets the last snapshot timestamp for a thermostat
func (s *MemoryOffsetStore) SetLastSnapshotTime(ctx context.Context, thermostatID string, timestamp time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSnapshotTimes[thermostatID] = timestamp
	return nil
}

// Scheduler manages the polling of thermostats and data collection
type Scheduler struct {
	providers      []model.Provider
	sinks          []model.Sink
	normalizer     *Normalizer
	offsetStore    OffsetStore
	pollInterval   time.Duration
	backfillWindow time.Duration
	idGenerator    model.DocumentIDGenerator
	metrics        *MetricsCollector
	logger         *slog.Logger
}

// NewScheduler creates a new scheduler
func NewScheduler(
	providers []model.Provider,
	sinks []model.Sink,
	normalizer *Normalizer,
	offsetStore OffsetStore,
	pollInterval, backfillWindow time.Duration,
	metrics *MetricsCollector,
	logger *slog.Logger,
) *Scheduler {
	return &Scheduler{
		providers:      providers,
		sinks:          sinks,
		normalizer:     normalizer,
		offsetStore:    offsetStore,
		pollInterval:   pollInterval,
		backfillWindow: backfillWindow,
		idGenerator:    model.NewIDGenerator(),
		metrics:        metrics,
		logger:         logger,
	}
}

// Start begins the polling scheduler
func (s *Scheduler) Start(ctx context.Context) error {
	s.logger.Info("Starting thermostat telemetry scheduler",
		"poll_interval", s.pollInterval,
		"backfill_window", s.backfillWindow,
		"providers", len(s.providers),
		"sinks", len(s.sinks))

	// Perform initial backfill for all thermostats
	if err := s.performInitialBackfill(ctx); err != nil {
		s.logger.Error("Initial backfill failed", "error", err)
		return fmt.Errorf("initial backfill: %w", err)
	}

	// Start the main polling loop
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Scheduler stopping due to context cancellation")
			return ctx.Err()
		case <-ticker.C:
			if err := s.pollAllThermostats(ctx); err != nil {
				s.logger.Error("Polling cycle failed", "error", err)
				// Continue polling even if one cycle fails
			}
		}
	}
}

// performInitialBackfill performs backfill for all thermostats
func (s *Scheduler) performInitialBackfill(ctx context.Context) error {
	s.logger.Info("Performing initial backfill")

	now := time.Now()
	backfillStart := now.Add(-s.backfillWindow)

	for _, provider := range s.providers {
		thermostats, err := provider.ListThermostats(ctx)
		if err != nil {
			s.logger.Error("Failed to list thermostats", "provider", provider.Info().Name, "error", err)
			continue
		}

		for _, thermostat := range thermostats {
			if err := s.backfillThermostat(ctx, provider, thermostat, backfillStart, now); err != nil {
				s.logger.Error("Failed to backfill thermostat",
					"provider", provider.Info().Name,
					"thermostat", thermostat.ID,
					"error", err)
			}
		}
	}

	return nil
}

// backfillThermostat performs backfill for a single thermostat
func (s *Scheduler) backfillThermostat(ctx context.Context, provider model.Provider, thermostat model.ThermostatRef, from, to time.Time) error {
	s.logger.Info("Backfilling thermostat",
		"thermostat", thermostat.ID,
		"from", from,
		"to", to)

	// Record provider request
	s.metrics.RecordProviderRequest(provider.Info().Name)

	// Get runtime data for the backfill period
	runtimeData, err := provider.GetRuntime(ctx, thermostat, from, to)
	if err != nil {
		s.metrics.RecordProviderError(provider.Info().Name)
		return fmt.Errorf("getting runtime data: %w", err)
	}

	// Normalize and write runtime data
	var docs []model.Doc
	for _, runtime := range runtimeData {
		canonical, err := s.normalizer.NormalizeRuntime5m(runtime, provider.Info().Name)
		if err != nil {
			s.logger.Error("Failed to normalize runtime data", "error", err)
			continue
		}

		// Generate document ID
		docID, err := s.idGenerator.GenerateRuntime5mID(canonical)
		if err != nil {
			s.logger.Error("Failed to generate document ID for runtime_5m", "error", err)
			continue
		}

		docs = append(docs, model.Doc{
			ID:   docID,
			Type: "runtime_5m",
			Body: canonical,
		})
	}

	// Write to all sinks
	if err := s.writeToAllSinks(ctx, docs); err != nil {
		return fmt.Errorf("writing backfill data: %w", err)
	}

	// Update offset
	if len(runtimeData) > 0 {
		lastRuntime := runtimeData[len(runtimeData)-1].EventTime
		if err := s.offsetStore.SetLastRuntimeTime(ctx, thermostat.ID, lastRuntime); err != nil {
			s.logger.Error("Failed to update runtime offset", "error", err)
		}
	}

	return nil
}

// pollAllThermostats polls all thermostats from all providers
func (s *Scheduler) pollAllThermostats(ctx context.Context) error {
	s.logger.Debug("Starting polling cycle")

	for _, provider := range s.providers {
		if err := s.pollProvider(ctx, provider); err != nil {
			s.logger.Error("Failed to poll provider", "provider", provider.Info().Name, "error", err)
		}
	}

	return nil
}

// pollProvider polls all thermostats from a single provider
func (s *Scheduler) pollProvider(ctx context.Context, provider model.Provider) error {
	thermostats, err := provider.ListThermostats(ctx)
	if err != nil {
		return fmt.Errorf("listing thermostats: %w", err)
	}

	for _, thermostat := range thermostats {
		if err := s.pollThermostat(ctx, provider, thermostat); err != nil {
			s.logger.Error("Failed to poll thermostat",
				"provider", provider.Info().Name,
				"thermostat", thermostat.ID,
				"error", err)
		}
	}

	return nil
}

// pollThermostat polls a single thermostat
func (s *Scheduler) pollThermostat(ctx context.Context, provider model.Provider, thermostat model.ThermostatRef) error {
	// Record provider request
	s.metrics.RecordProviderRequest(provider.Info().Name)

	// Check if we need to fetch new data
	summary, err := provider.GetSummary(ctx, thermostat)
	if err != nil {
		s.metrics.RecordProviderError(provider.Info().Name)
		return fmt.Errorf("getting summary: %w", err)
	}

	// Get last snapshot time
	lastSnapshot, err := s.offsetStore.GetLastSnapshotTime(ctx, thermostat.ID)
	if err != nil {
		s.logger.Warn("Failed to get last snapshot time, using zero time", "thermostat", thermostat.ID)
		lastSnapshot = time.Time{}
	}

	// Fetch snapshot if revision changed or snapshot is stale (≥15 min)
	shouldFetchSnapshot := summary.Revision != "" &&
		(lastSnapshot.IsZero() || time.Since(lastSnapshot) >= 15*time.Minute)

	if shouldFetchSnapshot {
		if err := s.fetchAndProcessSnapshot(ctx, provider, thermostat); err != nil {
			s.logger.Error("Failed to fetch snapshot", "thermostat", thermostat.ID, "error", err)
		}
	}

	// Get last runtime time
	lastRuntime, err := s.offsetStore.GetLastRuntimeTime(ctx, thermostat.ID)
	if err != nil {
		s.logger.Warn("Failed to get last runtime time, using zero time", "thermostat", thermostat.ID)
		lastRuntime = time.Time{}
	}

	// Fetch runtime data if we have a last runtime time
	if !lastRuntime.IsZero() {
		if err := s.fetchAndProcessRuntime(ctx, provider, thermostat, lastRuntime); err != nil {
			s.logger.Error("Failed to fetch runtime data", "thermostat", thermostat.ID, "error", err)
		}
	}

	return nil
}

// fetchAndProcessSnapshot fetches and processes a thermostat snapshot
func (s *Scheduler) fetchAndProcessSnapshot(ctx context.Context, provider model.Provider, thermostat model.ThermostatRef) error {
	s.logger.Debug("Fetching snapshot", "thermostat", thermostat.ID)

	// Record provider request
	s.metrics.RecordProviderRequest(provider.Info().Name)

	snapshot, err := provider.GetSnapshot(ctx, thermostat, time.Time{})
	if err != nil {
		s.metrics.RecordProviderError(provider.Info().Name)
		return fmt.Errorf("getting snapshot: %w", err)
	}

	// Normalize snapshot
	canonical := s.normalizer.NormalizeDeviceSnapshot(snapshot, provider.Info().Name)

	// Generate document ID
	docID, err := s.idGenerator.GenerateDeviceSnapshotID(canonical)
	if err != nil {
		return fmt.Errorf("generating document ID for device_snapshot: %w", err)
	}

	doc := model.Doc{
		ID:   docID,
		Type: "device_snapshot",
		Body: canonical,
	}

	// Write to all sinks
	if err := s.writeToAllSinks(ctx, []model.Doc{doc}); err != nil {
		return fmt.Errorf("writing snapshot: %w", err)
	}

	// Update offset
	if err := s.offsetStore.SetLastSnapshotTime(ctx, thermostat.ID, snapshot.CollectedAt); err != nil {
		s.logger.Error("Failed to update snapshot offset", "error", err)
	}

	return nil
}

// fetchAndProcessRuntime fetches and processes runtime data
func (s *Scheduler) fetchAndProcessRuntime(ctx context.Context, provider model.Provider, thermostat model.ThermostatRef, lastRuntime time.Time) error {
	s.logger.Debug("Fetching runtime data", "thermostat", thermostat.ID, "since", lastRuntime)

	// Record provider request
	s.metrics.RecordProviderRequest(provider.Info().Name)

	now := time.Now()
	runtimeData, err := provider.GetRuntime(ctx, thermostat, lastRuntime, now)
	if err != nil {
		s.metrics.RecordProviderError(provider.Info().Name)
		return fmt.Errorf("getting runtime data: %w", err)
	}

	if len(runtimeData) == 0 {
		s.logger.Debug("No new runtime data", "thermostat", thermostat.ID)
		return nil
	}

	// Normalize and write runtime data, and detect transitions
	var docs []model.Doc
	var prevState *model.State

	for _, runtime := range runtimeData {
		canonical, err := s.normalizer.NormalizeRuntime5m(runtime, provider.Info().Name)
		if err != nil {
			s.logger.Error("Failed to normalize runtime data", "error", err)
			continue
		}

		// Generate document ID
		docID, err := s.idGenerator.GenerateRuntime5mID(canonical)
		if err != nil {
			s.logger.Error("Failed to generate document ID for runtime_5m", "error", err)
			continue
		}

		docs = append(docs, model.Doc{
			ID:   docID,
			Type: "runtime_5m",
			Body: canonical,
		})

		// Check for state transitions (compare with previous runtime row)
		currentState := model.State{
			Mode:     canonical.Mode,
			SetHeatC: canonical.SetHeatC,
			SetCoolC: canonical.SetCoolC,
			Climate:  canonical.Climate,
		}

		if prevState != nil && s.hasStateChanged(*prevState, currentState) {
			// Generate transition document
			transition := s.normalizer.NormalizeTransition(
				thermostat,
				canonical.EventTime,
				*prevState,
				currentState,
				model.EventInfo{
					Kind: s.inferTransitionKind(*prevState, currentState),
				},
				provider.Info().Name,
				nil,
			)

			transitionID, err := s.idGenerator.GenerateTransitionID(transition)
			if err != nil {
				s.logger.Error("Failed to generate document ID for transition", "error", err)
			} else {
				docs = append(docs, model.Doc{
					ID:   transitionID,
					Type: "transition",
					Body: transition,
				})
			}
		}

		// Store current state for next iteration
		prevState = &currentState
	}

	// Write to all sinks
	if err := s.writeToAllSinks(ctx, docs); err != nil {
		return fmt.Errorf("writing runtime data: %w", err)
	}

	// Update offset
	if len(runtimeData) > 0 {
		lastRuntimeTime := runtimeData[len(runtimeData)-1].EventTime
		if err := s.offsetStore.SetLastRuntimeTime(ctx, thermostat.ID, lastRuntimeTime); err != nil {
			s.logger.Error("Failed to update runtime offset", "error", err)
		}
	}

	return nil
}

// writeToAllSinks writes documents to all configured sinks
func (s *Scheduler) writeToAllSinks(ctx context.Context, docs []model.Doc) error {
	if len(docs) == 0 {
		return nil
	}

	for _, sink := range s.sinks {
		result, err := sink.Write(ctx, docs)
		if err != nil {
			s.logger.Error("Failed to write to sink",
				"sink", sink.Info().Name,
				"error", err)
			s.metrics.RecordSinkError(sink.Info().Name)
			continue
		}

		// Record metrics
		s.metrics.RecordSinkWrite(sink.Info().Name, int64(result.SuccessCount))

		s.logger.Debug("Wrote to sink",
			"sink", sink.Info().Name,
			"success_count", result.SuccessCount,
			"error_count", result.ErrorCount)

		if result.ErrorCount > 0 {
			s.logger.Warn("Some documents failed to write",
				"sink", sink.Info().Name,
				"errors", result.Errors)
			s.metrics.RecordSinkError(sink.Info().Name)
		}
	}

	return nil
}

// hasStateChanged determines if the thermostat state has changed significantly
func (s *Scheduler) hasStateChanged(prev, current model.State) bool {
	// Check mode change
	if prev.Mode != current.Mode {
		return true
	}

	// Check climate change
	if prev.Climate != current.Climate {
		return true
	}

	// Check setpoint changes (with tolerance for floating point comparison)
	const tolerance = 0.1
	if !floatsEqual(prev.SetHeatC, current.SetHeatC, tolerance) {
		return true
	}
	if !floatsEqual(prev.SetCoolC, current.SetCoolC, tolerance) {
		return true
	}

	return false
}

// floatsEqual compares two float pointers within a tolerance
func floatsEqual(a, b *float64, tolerance float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	diff := *a - *b
	if diff < 0 {
		diff = -diff
	}
	return diff < tolerance
}

// inferTransitionKind infers the kind of transition based on state changes
func (s *Scheduler) inferTransitionKind(prev, current model.State) string {
	// Mode changes are manual or schedule-driven
	if prev.Mode != current.Mode {
		return "manual"
	}

	// Climate changes typically indicate schedule or hold events
	if prev.Climate != current.Climate {
		if current.Climate == "Away" || current.Climate == "Vacation" {
			return "vacation"
		}
		return "schedule"
	}

	// Setpoint changes without mode/climate change are usually holds
	const tolerance = 0.1
	if !floatsEqual(prev.SetHeatC, current.SetHeatC, tolerance) ||
		!floatsEqual(prev.SetCoolC, current.SetCoolC, tolerance) {
		return "hold"
	}

	return "unknown"
}
