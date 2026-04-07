package router

import "github.com/bangzzzz/pb-go-action/pkg/ggrpc/registry"

// RegionRouter filters instances by region, datacenter, and/or tenant.
// Only non-empty fields are applied as constraints.
type RegionRouter struct {
	region     string
	datacenter string
	tenant     string
}

// NewRegionRouter returns a Router that filters by region, datacenter, and/or tenant.
func NewRegionRouter(region, datacenter, tenant string) *RegionRouter {
	return &RegionRouter{region: region, datacenter: datacenter, tenant: tenant}
}

// Route returns instances that satisfy all configured constraints.
func (r *RegionRouter) Route(instances []*registry.Instance) []*registry.Instance {
	if r.region == "" && r.datacenter == "" && r.tenant == "" {
		return instances
	}
	out := make([]*registry.Instance, 0, len(instances))
	for _, inst := range instances {
		if r.region != "" && inst.Region != r.region {
			continue
		}
		if r.datacenter != "" && inst.Datacenter != r.datacenter {
			continue
		}
		if r.tenant != "" && inst.Tenant != r.tenant {
			continue
		}
		out = append(out, inst)
	}
	return out
}
