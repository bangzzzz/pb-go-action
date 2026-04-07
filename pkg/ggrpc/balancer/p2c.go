package balancer

import (
	"math/rand"
	"sync"
	"sync/atomic"

	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/registry"
)

// p2cEntry tracks in-flight count per instance for P2C.
type p2cEntry struct {
	inst     *registry.Instance
	inflight int64
}

// P2C implements the Power-of-Two-Choices balancer.
// It picks two random candidates and returns the one with fewer in-flight requests.
type P2C struct {
	mu      sync.Mutex
	entries map[string]*p2cEntry
}

// NewP2C returns a new P2C balancer.
func NewP2C() *P2C {
	return &P2C{entries: make(map[string]*p2cEntry)}
}

// Pick selects an instance using the P2C algorithm.
func (p *P2C) Pick(instances []*registry.Instance) (*registry.Instance, error) {
	if len(instances) == 0 {
		return nil, ErrNoInstances
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.sync(instances)

	if len(instances) == 1 {
		e := p.entries[instances[0].ID]
		atomic.AddInt64(&e.inflight, 1)
		return e.inst, nil
	}

	// Pick two distinct random indices.
	i := rand.Intn(len(instances))
	j := rand.Intn(len(instances) - 1)
	if j >= i {
		j++
	}

	eA := p.entries[instances[i].ID]
	eB := p.entries[instances[j].ID]

	var best *p2cEntry
	if atomic.LoadInt64(&eA.inflight) <= atomic.LoadInt64(&eB.inflight) {
		best = eA
	} else {
		best = eB
	}
	atomic.AddInt64(&best.inflight, 1)
	return best.inst, nil
}

// PickWithDone picks and returns a DoneFunc that decrements inflight.
func (p *P2C) PickWithDone(instances []*registry.Instance) (*registry.Instance, DoneFunc, error) {
	inst, err := p.Pick(instances)
	if err != nil {
		return nil, nil, err
	}
	p.mu.Lock()
	e := p.entries[inst.ID]
	p.mu.Unlock()

	done := func(_ DoneInfo) {
		atomic.AddInt64(&e.inflight, -1)
	}
	return inst, done, nil
}

func (p *P2C) sync(instances []*registry.Instance) {
	for _, inst := range instances {
		if _, ok := p.entries[inst.ID]; !ok {
			p.entries[inst.ID] = &p2cEntry{inst: inst}
		} else {
			p.entries[inst.ID].inst = inst
		}
	}
}
