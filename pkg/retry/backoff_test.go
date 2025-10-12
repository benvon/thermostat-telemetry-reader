package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	t.Parallel()

	config := Config{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		Jitter:       false, // Disable jitter for predictable testing
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 0},
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 800 * time.Millisecond},
		{5, 1 * time.Second}, // Capped at MaxDelay
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			delay := config.Backoff(tt.attempt)
			if delay != tt.expected {
				t.Errorf("Attempt %d: expected %v, got %v", tt.attempt, tt.expected, delay)
			}
		})
	}
}

func TestBackoffWithJitter(t *testing.T) {
	t.Parallel()

	config := Config{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}

	// With jitter, we should get different delays
	delay1 := config.Backoff(2)
	delay2 := config.Backoff(2)

	// Both should be around 200ms but not exactly the same
	expected := 200 * time.Millisecond
	tolerance := 50 * time.Millisecond

	if delay1 < expected-tolerance || delay1 > expected+tolerance {
		t.Errorf("Delay 1 out of expected range: %v", delay1)
	}

	if delay2 < expected-tolerance || delay2 > expected+tolerance {
		t.Errorf("Delay 2 out of expected range: %v", delay2)
	}
}

func TestDo_Success(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	config.InitialDelay = 10 * time.Millisecond
	ctx := context.Background()

	callCount := 0
	fn := func() error {
		callCount++
		return nil
	}

	err := Do(ctx, config, fn)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}
}

func TestDo_RetryAndSuccess(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	config.InitialDelay = 10 * time.Millisecond
	config.MaxRetries = 3
	ctx := context.Background()

	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary failure in name resolution")
		}
		return nil
	}

	err := Do(ctx, config, fn)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}
}

func TestDo_MaxRetriesExceeded(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	config.InitialDelay = 10 * time.Millisecond
	config.MaxRetries = 2
	ctx := context.Background()

	callCount := 0
	fn := func() error {
		callCount++
		return errors.New("connection refused")
	}

	err := Do(ctx, config, fn)
	if err == nil {
		t.Error("Expected error, got nil")
	}

	// Should be called initial attempt + 2 retries = 3 times
	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}
}

func TestDo_NonRetriableError(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	config.InitialDelay = 10 * time.Millisecond
	ctx := context.Background()

	callCount := 0
	fn := func() error {
		callCount++
		return errors.New("not a retriable error")
	}

	err := Do(ctx, config, fn)
	if err == nil {
		t.Error("Expected error, got nil")
	}

	// Should only be called once (no retries for non-retriable errors)
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}
}

func TestDo_ContextCancellation(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	config.InitialDelay = 100 * time.Millisecond
	config.MaxRetries = 5

	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	fn := func() error {
		callCount++
		if callCount == 2 {
			cancel() // Cancel context after second call
		}
		return errors.New("connection refused")
	}

	err := Do(ctx, config, fn)
	if err == nil {
		t.Error("Expected error, got nil")
	}

	// Should stop retrying after context is cancelled
	if callCount > 3 {
		t.Errorf("Expected at most 3 calls, got %d", callCount)
	}
}

func TestIsRetriable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		retriable bool
	}{
		{"nil error", nil, false},
		{"timeout", errors.New("connection timeout"), true},
		{"connection refused", errors.New("connection refused"), true},
		{"temporary failure", errors.New("temporary failure in name resolution"), true},
		{"non-retriable", errors.New("invalid input"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetriable(tt.err)
			if result != tt.retriable {
				t.Errorf("Expected %v, got %v for error: %v", tt.retriable, result, tt.err)
			}
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    string
		minDelay time.Duration
		maxDelay time.Duration
	}{
		{"seconds", "120", 120 * time.Second, 120 * time.Second},
		{"zero", "0", 0, 0},
		{"invalid", "invalid", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := parseRetryAfter(tt.value)
			if delay < tt.minDelay || delay > tt.maxDelay {
				t.Errorf("Expected delay between %v and %v, got %v", tt.minDelay, tt.maxDelay, delay)
			}
		})
	}
}
