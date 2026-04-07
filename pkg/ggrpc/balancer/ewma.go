package balancer

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/registry"
)

const (
	defaultDecay    = 0.95  // EMA decay factor applied per window
	defaultInitCost = int64(time.Millisecond) // initial cost estimate
)

// ewmaEntry tracks per-instance latency statistics.
type ewmaEntry struct {
	inst     *registry.Instance
	inflight int64
	cost     int64 // EMA of latency in nanoseconds (int64 for atomic)
	stamp    int64 // unix nano of last update
}

func (e *ewmaEntry) load() float64 {
	inflight := atomic.LoadInt64(&e.inflight)
	cost := atomic.LoadInt64(&e.cost)
	if cost <= 0 {
		cost = defaultInitCost
	}
	// Use sqrt to reduce the dominance of inflight on the score.
	return float64(cost) * (math.Sqrt(float64(inflight+1)))
}

// EWMA implements Exponentially Weighted Moving Average load balancing.
// It scores each instance by cost × sqrt(inflight+1) and picks the minimum.
type EWMA struct {
	mu      sync.Mutex
	entries map[string]*ewmaEntry
}

// NewEWMA returns a new EWMA balancer.
func NewEWMA() *EWMA {
	return &EWMA{entries: make(map[string]*ewmaEntry)}
}

// Pick selects the instance with the lowest EWMA score.
func (e *EWMA) Pick(instances []*registry.Instance) (*registry.Instance, error) {
	inst, _, err := e.PickWithDone(instances)
	return inst, err
}

// PickWithDone picks an instance and returns a DoneFunc for feedback.
func (e *EWMA) PickWithDone(instances []*registry.Instance) (*registry.Instance, DoneFunc, error) {
	if len(instances) == 0 {
		return nil, nil, ErrNoInstances
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.sync(instances)

	var best *ewmaEntry
	for _, inst := range instances {
		en := e.entries[inst.ID]
		if best == nil || en.load() < best.load() {
			best = en
		}
	}
	atomic.AddInt64(&best.inflight, 1)
	start := time.Now()

	done := func(info DoneInfo) {
		latency := info.Latency
		if latency <= 0 {
			latency = time.Since(start).Nanoseconds()
		}
		atomic.AddInt64(&best.inflight, -1)

		now := time.Now().UnixNano()
		last := atomic.LoadInt64(&best.stamp)
		atomic.StoreInt64(&best.stamp, now)

		// Compute time-weighted decay.
		dt := float64(now-last) / float64(time.Second)
		if dt < 0 {
			dt = 0
		}
		decay := math.Pow(defaultDecay, dt)
		old := float64(atomic.LoadInt64(&best.cost))
		newVal := old*decay + float64(latency)*(1-decay)
		atomic.StoreInt64(&best.cost, int64(newVal))
	}
	return best.inst, done, nil
}

func (e *EWMA) sync(instances []*registry.Instance) {
	for _, inst := range instances {
		if _, ok := e.entries[inst.ID]; !ok {
			e.entries[inst.ID] = &ewmaEntry{
				inst: inst,
				cost: defaultInitCost,
			}
		} else {
			e.entries[inst.ID].inst = inst
		}
	}
}
