// Package circuitbreaker implements a three-state circuit breaker.
package circuitbreaker

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// State represents the circuit breaker state.
type State int32

const (
	StateClosed   State = iota // passing requests normally
	StateOpen                  // rejecting all requests
	StateHalfOpen              // allowing limited probe requests
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// ErrOpen is returned when the circuit is open.
var ErrOpen = errors.New("circuit breaker: circuit is open")

// Config holds circuit breaker parameters.
type Config struct {
	// FailureThreshold is the fraction of failures (0-1) that triggers the open state.
	// Evaluated over the last window of requests.
	FailureThreshold float64
	// SuccessThreshold is the number of consecutive successes required to close
	// a half-open circuit.
	SuccessThreshold int
	// Timeout is the time the breaker stays open before transitioning to half-open.
	Timeout time.Duration
	// MaxRequests is the max number of requests allowed while half-open.
	MaxRequests int
	// MinRequests is the minimum number of requests in the window before failure
	// rate evaluation is performed.
	MinRequests int
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		FailureThreshold: 0.5,
		SuccessThreshold: 2,
		Timeout:          10 * time.Second,
		MaxRequests:      1,
		MinRequests:      5,
	}
}

// Breaker is a thread-safe circuit breaker.
type Breaker struct {
	mu       sync.Mutex
	cfg      Config
	state    State
	openedAt time.Time

	// counters (reset on state transition)
	requests  int64
	failures  int64
	successes int64 // consecutive successes in half-open
	halfReqs  int64 // requests sent in half-open state
}

// New creates a new Breaker with the given config.
func New(cfg Config) *Breaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 0.5
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 2
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.MaxRequests <= 0 {
		cfg.MaxRequests = 1
	}
	if cfg.MinRequests <= 0 {
		cfg.MinRequests = 5
	}
	return &Breaker{cfg: cfg}
}

// Allow returns nil if the request should be forwarded, or ErrOpen if the
// circuit is open and the request should be rejected.
func (b *Breaker) Allow() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		atomic.AddInt64(&b.requests, 1)
		return nil
	case StateOpen:
		if time.Since(b.openedAt) >= b.cfg.Timeout {
			b.transition(StateHalfOpen)
		} else {
			return ErrOpen
		}
		fallthrough
	case StateHalfOpen:
		if atomic.LoadInt64(&b.halfReqs) >= int64(b.cfg.MaxRequests) {
			return ErrOpen
		}
		atomic.AddInt64(&b.halfReqs, 1)
		return nil
	}
	return nil
}

// Success records a successful request outcome.
func (b *Breaker) Success() {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		// Nothing to do; failures will be checked separately.
	case StateHalfOpen:
		b.successes++
		if b.successes >= int64(b.cfg.SuccessThreshold) {
			b.transition(StateClosed)
		}
	}
}

// Failure records a failed request outcome.
func (b *Breaker) Failure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		b.failures++
		b.checkThreshold()
	case StateHalfOpen:
		// Any failure in half-open state reopens the circuit.
		b.transition(StateOpen)
	}
}

// State returns the current circuit state.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

func (b *Breaker) checkThreshold() {
	if b.requests < int64(b.cfg.MinRequests) {
		return
	}
	rate := float64(b.failures) / float64(b.requests)
	if rate >= b.cfg.FailureThreshold {
		b.transition(StateOpen)
	}
}

func (b *Breaker) transition(to State) {
	b.state = to
	switch to {
	case StateOpen:
		b.openedAt = time.Now()
		b.requests = 0
		b.failures = 0
		b.successes = 0
		b.halfReqs = 0
	case StateHalfOpen:
		b.halfReqs = 0
		b.successes = 0
	case StateClosed:
		b.requests = 0
		b.failures = 0
		b.successes = 0
		b.halfReqs = 0
	}
}
