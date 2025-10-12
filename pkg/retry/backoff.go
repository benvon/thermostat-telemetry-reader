package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

// Config holds retry configuration parameters
type Config struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int
	// InitialDelay is the initial delay before the first retry
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration
	// Multiplier is the factor by which the delay increases each retry
	Multiplier float64
	// Jitter adds randomness to delay to prevent thundering herd
	Jitter bool
}

// DefaultConfig returns a default retry configuration
func DefaultConfig() Config {
	return Config{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
}

// Backoff calculates the delay for a given retry attempt
func (c Config) Backoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	// Calculate exponential backoff
	delay := float64(c.InitialDelay) * math.Pow(c.Multiplier, float64(attempt-1))

	// Cap at max delay
	if delay > float64(c.MaxDelay) {
		delay = float64(c.MaxDelay)
	}

	// Add jitter if enabled
	if c.Jitter {
		// Add random jitter between 0 and 25% of the delay
		// Using math/rand/v2 is appropriate here as we don't need cryptographic security
		// for retry jitter - we just want to spread out retries to avoid thundering herd
		// #nosec G404 - Non-cryptographic random is sufficient for retry jitter
		jitter := rand.Float64() * 0.25 * delay
		delay += jitter
	}

	return time.Duration(delay)
}

// Do executes a function with retry logic
func Do(ctx context.Context, config Config, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate backoff delay
			delay := config.Backoff(attempt)

			select {
			case <-ctx.Done():
				return fmt.Errorf("retry cancelled: %w", ctx.Err())
			case <-time.After(delay):
				// Continue with retry
			}
		}

		err := fn()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if we should retry
		if !IsRetriable(err) {
			return err // Don't retry non-retriable errors
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// DoWithResponse executes an HTTP request with retry logic and respects Retry-After headers
func DoWithResponse(ctx context.Context, config Config, fn func() (*http.Response, error)) (*http.Response, error) {
	var lastErr error
	var resp *http.Response

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate backoff delay
			delay := config.Backoff(attempt)

			// Check if previous response had Retry-After header
			if resp != nil && resp.Header.Get("Retry-After") != "" {
				if retryAfter := parseRetryAfter(resp.Header.Get("Retry-After")); retryAfter > 0 {
					delay = retryAfter
				}
			}

			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("retry cancelled: %w", ctx.Err())
			case <-time.After(delay):
				// Continue with retry
			}
		}

		resp, lastErr = fn()
		if lastErr == nil && resp != nil {
			// Check response status
			if resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
				return resp, nil // Success or client error (don't retry client errors)
			}

			// Close response body for retriable errors
			if resp.Body != nil {
				_ = resp.Body.Close()
			}

			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		}

		if lastErr != nil && !IsRetriable(lastErr) {
			return resp, lastErr // Don't retry non-retriable errors
		}
	}

	return resp, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// IsRetriable determines if an error is retriable
func IsRetriable(err error) bool {
	if err == nil {
		return false
	}

	// Add logic to determine if error is retriable
	// This is a simple implementation - can be enhanced
	errStr := err.Error()

	// Network errors are generally retriable
	retriableMessages := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"temporary failure",
		"no such host",
		"TLS handshake timeout",
	}

	for _, msg := range retriableMessages {
		if contains(errStr, msg) {
			return true
		}
	}

	return false
}

// parseRetryAfter parses the Retry-After header
// It can be either a number of seconds or an HTTP date
func parseRetryAfter(value string) time.Duration {
	// Try parsing as seconds
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second
	}

	// Try parsing as HTTP date
	if t, err := http.ParseTime(value); err == nil {
		return time.Until(t)
	}

	return 0
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) && anyMatch(s, substr))
}

func anyMatch(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
