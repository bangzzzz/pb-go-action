package balancer

import (
	"sync"

	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/registry"
)

// wrrEntry tracks the current weight of an instance.
type wrrEntry struct {
	inst    *registry.Instance
	current int
}

// WeightedRoundRobin implements the smooth weighted round-robin algorithm
// (Nginx-style: advances current weights, picks the highest, then reduces it
// by the total weight sum).
type WeightedRoundRobin struct {
	mu      sync.Mutex
	entries []*wrrEntry
	ids     []string // ordered IDs used to detect list changes
}

// NewWeightedRoundRobin returns a new WeightedRoundRobin balancer.
func NewWeightedRoundRobin() *WeightedRoundRobin {
	return &WeightedRoundRobin{}
}

// Pick selects an instance using smooth weighted round-robin.
func (w *WeightedRoundRobin) Pick(instances []*registry.Instance) (*registry.Instance, error) {
	if len(instances) == 0 {
		return nil, ErrNoInstances
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.sync(instances)

	total := 0
	for _, e := range w.entries {
		weight := e.inst.Weight
		if weight <= 0 {
			weight = 1
		}
		total += weight
		e.current += weight
	}

	best := w.entries[0]
	for _, e := range w.entries[1:] {
		if e.current > best.current {
			best = e
		}
	}
	best.current -= total
	return best.inst, nil
}

// sync rebuilds the entries slice when the instance list changes.
func (w *WeightedRoundRobin) sync(instances []*registry.Instance) {
	ids := make([]string, len(instances))
	for i, inst := range instances {
		ids[i] = inst.ID
	}
	if equalIDs(w.ids, ids) {
		// Update instance pointers in-place (weights may have changed).
		for i, e := range w.entries {
			e.inst = instances[i]
		}
		return
	}
	// Rebuild entries, preserving current weight for known instances.
	prev := make(map[string]int, len(w.entries))
	for _, e := range w.entries {
		prev[e.inst.ID] = e.current
	}
	w.entries = make([]*wrrEntry, len(instances))
	for i, inst := range instances {
		w.entries[i] = &wrrEntry{inst: inst, current: prev[inst.ID]}
	}
	w.ids = ids
}

func equalIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
