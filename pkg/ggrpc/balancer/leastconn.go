package balancer

import (
	"sync"
	"sync/atomic"

	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/registry"
)

// leastConnEntry tracks in-flight connections for one instance.
type leastConnEntry struct {
	inst    *registry.Instance
	inflight int64
}

// LeastConn picks the instance with the fewest in-flight requests.
type LeastConn struct {
	mu      sync.Mutex
	entries map[string]*leastConnEntry
}

// NewLeastConn returns a new LeastConn balancer.
func NewLeastConn() *LeastConn {
	return &LeastConn{entries: make(map[string]*leastConnEntry)}
}

// Pick selects the instance with the lowest in-flight request count.
func (l *LeastConn) Pick(instances []*registry.Instance) (*registry.Instance, error) {
	if len(instances) == 0 {
		return nil, ErrNoInstances
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	l.syncEntries(instances)

	var best *leastConnEntry
	for _, inst := range instances {
		e := l.entries[inst.ID]
		if best == nil || atomic.LoadInt64(&e.inflight) < atomic.LoadInt64(&best.inflight) {
			best = e
		}
	}
	atomic.AddInt64(&best.inflight, 1)
	return best.inst, nil
}

// PickWithDone picks an instance and returns a DoneFunc that decrements the counter.
func (l *LeastConn) PickWithDone(instances []*registry.Instance) (*registry.Instance, DoneFunc, error) {
	inst, err := l.Pick(instances)
	if err != nil {
		return nil, nil, err
	}
	l.mu.Lock()
	e := l.entries[inst.ID]
	l.mu.Unlock()

	done := func(_ DoneInfo) {
		atomic.AddInt64(&e.inflight, -1)
	}
	return inst, done, nil
}

func (l *LeastConn) syncEntries(instances []*registry.Instance) {
	for _, inst := range instances {
		if _, ok := l.entries[inst.ID]; !ok {
			l.entries[inst.ID] = &leastConnEntry{inst: inst}
		} else {
			l.entries[inst.ID].inst = inst
		}
	}
}
