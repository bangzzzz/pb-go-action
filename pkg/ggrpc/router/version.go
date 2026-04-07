package router

import "github.com/bangzzzz/pb-go-action/pkg/ggrpc/registry"

// VersionRouter filters instances that match the requested version.
// An empty version string passes all instances through.
type VersionRouter struct {
	version string
}

// NewVersionRouter returns a Router that keeps only instances with the given version.
func NewVersionRouter(version string) *VersionRouter {
	return &VersionRouter{version: version}
}

// Route returns instances whose Version field matches the configured version.
func (r *VersionRouter) Route(instances []*registry.Instance) []*registry.Instance {
	if r.version == "" {
		return instances
	}
	out := make([]*registry.Instance, 0, len(instances))
	for _, inst := range instances {
		if inst.Version == r.version {
			out = append(out, inst)
		}
	}
	return out
}
