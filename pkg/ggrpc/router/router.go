// Package router provides composable routing strategies for service instances.
package router

import "github.com/bangzzzz/pb-go-action/pkg/ggrpc/registry"

// Router filters a list of instances and returns the eligible subset.
type Router interface {
	Route(instances []*registry.Instance) []*registry.Instance
}

// Chain applies a sequence of routers in order; each router narrows the list.
// If a router produces an empty list the chain falls back to the input of that
// router to avoid total blackout.
type Chain struct {
	routers []Router
}

// NewChain builds a filter chain from the given routers.
func NewChain(routers ...Router) *Chain {
	return &Chain{routers: routers}
}

// Route runs all routers in sequence.
func (c *Chain) Route(instances []*registry.Instance) []*registry.Instance {
	cur := instances
	for _, r := range c.routers {
		filtered := r.Route(cur)
		if len(filtered) > 0 {
			cur = filtered
		}
		// if filtered is empty, keep cur unchanged (fallback)
	}
	return cur
}
