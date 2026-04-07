// Package balancer provides load-balancing algorithms for service instances.
package balancer

import (
	"errors"

	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/registry"
)

// ErrNoInstances is returned when there are no instances to pick from.
var ErrNoInstances = errors.New("balancer: no available instances")

// Balancer selects a single instance from the candidate list.
type Balancer interface {
	Pick(instances []*registry.Instance) (*registry.Instance, error)
}

// DoneFunc is called by the caller after the RPC completes to report metrics
// (latency, success/failure) back to the balancer. It is optional; if not
// called the balancer simply does not receive feedback.
type DoneFunc func(info DoneInfo)

// DoneInfo carries per-RPC completion information.
type DoneInfo struct {
	Err     error
	Latency int64 // nanoseconds
}

// FeedbackBalancer extends Balancer with a feedback mechanism for adaptive algorithms.
type FeedbackBalancer interface {
	Balancer
	// PickWithDone returns the chosen instance AND a DoneFunc to call when done.
	PickWithDone(instances []*registry.Instance) (*registry.Instance, DoneFunc, error)
}
