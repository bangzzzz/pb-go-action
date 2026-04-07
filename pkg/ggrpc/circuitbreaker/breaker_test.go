package circuitbreaker

import (
	"testing"
	"time"
)

func testConfig() Config {
	return Config{
		FailureThreshold: 0.5,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
		MaxRequests:      2,
		MinRequests:      4,
	}
}

func TestBreaker_InitiallyClosed(t *testing.T) {
	b := New(testConfig())
	if b.State() != StateClosed {
		t.Errorf("expected closed, got %s", b.State())
	}
}

func TestBreaker_OpenOnFailureThreshold(t *testing.T) {
	b := New(testConfig())

	// Send 4 requests: 2 success, 2 failure → 50% failure rate = threshold.
	for i := 0; i < 2; i++ {
		_ = b.Allow()
		b.Success()
	}
	for i := 0; i < 2; i++ {
		_ = b.Allow()
		b.Failure()
	}

	if b.State() != StateOpen {
		t.Errorf("expected open after threshold, got %s", b.State())
	}
}

func TestBreaker_RejectsWhenOpen(t *testing.T) {
	b := New(testConfig())
	// Force open state directly.
	for i := 0; i < 4; i++ {
		_ = b.Allow()
		b.Failure()
	}

	if err := b.Allow(); err != ErrOpen {
		t.Errorf("expected ErrOpen, got %v", err)
	}
}

func TestBreaker_HalfOpen_AfterTimeout(t *testing.T) {
	b := New(testConfig())
	for i := 0; i < 4; i++ {
		_ = b.Allow()
		b.Failure()
	}
	if b.State() != StateOpen {
		t.Fatalf("expected open, got %s", b.State())
	}

	time.Sleep(60 * time.Millisecond) // wait for timeout

	// The next Allow() should transition to half-open.
	if err := b.Allow(); err != nil {
		t.Errorf("expected allow after timeout, got %v", err)
	}
	if b.State() != StateHalfOpen {
		t.Errorf("expected half-open, got %s", b.State())
	}
}

func TestBreaker_HalfOpen_ClosesOnSuccess(t *testing.T) {
	b := New(testConfig())
	for i := 0; i < 4; i++ {
		_ = b.Allow()
		b.Failure()
	}
	time.Sleep(60 * time.Millisecond)

	// Two consecutive successes should close the breaker.
	_ = b.Allow()
	b.Success()
	_ = b.Allow()
	b.Success()

	if b.State() != StateClosed {
		t.Errorf("expected closed after successes, got %s", b.State())
	}
}

func TestBreaker_HalfOpen_ReopensOnFailure(t *testing.T) {
	b := New(testConfig())
	for i := 0; i < 4; i++ {
		_ = b.Allow()
		b.Failure()
	}
	time.Sleep(60 * time.Millisecond)

	_ = b.Allow()
	b.Failure() // single failure in half-open → reopen

	if b.State() != StateOpen {
		t.Errorf("expected open after half-open failure, got %s", b.State())
	}
}

func TestBreaker_MaxRequestsHalfOpen(t *testing.T) {
	cfg := testConfig()
	cfg.MaxRequests = 1
	b := New(cfg)

	for i := 0; i < 4; i++ {
		_ = b.Allow()
		b.Failure()
	}
	time.Sleep(60 * time.Millisecond)

	_ = b.Allow() // allowed (1st probe)
	if err := b.Allow(); err != ErrOpen {
		t.Errorf("expected ErrOpen for 2nd probe, got %v", err)
	}
}
