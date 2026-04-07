package balancer

import (
	"sync/atomic"

	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/registry"
)

// RoundRobin picks instances in turn, cycling through the list indefinitely.
type RoundRobin struct {
	counter uint64
}

// NewRoundRobin returns a new RoundRobin balancer.
func NewRoundRobin() *RoundRobin { return &RoundRobin{} }

// Pick returns the next instance in a round-robin fashion.
func (r *RoundRobin) Pick(instances []*registry.Instance) (*registry.Instance, error) {
	if len(instances) == 0 {
		return nil, ErrNoInstances
	}
	n := atomic.AddUint64(&r.counter, 1)
	return instances[(n-1)%uint64(len(instances))], nil
}
