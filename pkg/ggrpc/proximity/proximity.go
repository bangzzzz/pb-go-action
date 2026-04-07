// Package proximity provides proximity-based instance selection.
package proximity

import (
	"sort"

	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/registry"
)

// Policy controls how proximity filtering is applied.
type Policy int

const (
	// SameDatacenter prefers instances in the same datacenter.
	SameDatacenter Policy = iota
	// SameRegion prefers instances in the same region.
	SameRegion
	// LatencyFirst sorts instances by a provided latency map (lowest first).
	LatencyFirst
)

// Selector filters and sorts instances based on proximity to the caller.
type Selector struct {
	policy     Policy
	datacenter string
	region     string
	latency    map[string]int64 // instance ID → latency nanoseconds
}

// NewSelector returns a Selector configured for the given policy and locality.
func NewSelector(policy Policy, datacenter, region string, latency map[string]int64) *Selector {
	return &Selector{
		policy:     policy,
		datacenter: datacenter,
		region:     region,
		latency:    latency,
	}
}

// Select returns instances ordered by proximity. If no instances match the
// preferred locality the full list is returned unchanged.
func (s *Selector) Select(instances []*registry.Instance) []*registry.Instance {
	if len(instances) == 0 {
		return instances
	}

	switch s.policy {
	case SameDatacenter:
		return s.prefer(instances, func(inst *registry.Instance) bool {
			return inst.Datacenter == s.datacenter
		})
	case SameRegion:
		return s.prefer(instances, func(inst *registry.Instance) bool {
			return inst.Region == s.region
		})
	case LatencyFirst:
		return s.sortByLatency(instances)
	}
	return instances
}

// prefer puts matching instances first; falls back to all if none match.
func (s *Selector) prefer(instances []*registry.Instance, match func(*registry.Instance) bool) []*registry.Instance {
	var preferred, rest []*registry.Instance
	for _, inst := range instances {
		if match(inst) {
			preferred = append(preferred, inst)
		} else {
			rest = append(rest, inst)
		}
	}
	if len(preferred) == 0 {
		return instances
	}
	return append(preferred, rest...)
}

// sortByLatency returns a copy sorted by latency (ascending).
func (s *Selector) sortByLatency(instances []*registry.Instance) []*registry.Instance {
	cp := make([]*registry.Instance, len(instances))
	copy(cp, instances)
	sort.Slice(cp, func(i, j int) bool {
		li := s.latency[cp[i].ID]
		lj := s.latency[cp[j].ID]
		return li < lj
	})
	return cp
}
