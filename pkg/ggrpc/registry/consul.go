package registry

import (
	"context"
	"fmt"
)

// ConsulClient is the minimal interface callers must satisfy to use consul-backed registry.
// This keeps the registry package free of the consul SDK.
type ConsulClient interface {
	// RegisterService registers a service instance with consul.
	RegisterService(id, name, address string, port int, tags []string, meta map[string]string) error
	// DeregisterService removes a service instance from consul.
	DeregisterService(id string) error
	// HealthService returns the passing instances for the given service.
	HealthService(service, tag string, passingOnly bool) ([]ConsulEntry, error)
}

// ConsulEntry is a minimal representation of a consul service entry.
type ConsulEntry struct {
	ID      string
	Service string
	Address string
	Port    int
	Tags    []string
	Meta    map[string]string
}

type consulRegistry struct {
	client ConsulClient
}

// NewConsulRegistry wraps an arbitrary ConsulClient as a Registry.
func NewConsulRegistry(client ConsulClient) Registry {
	return &consulRegistry{client: client}
}

func (r *consulRegistry) Register(_ context.Context, inst *Instance) error {
	if inst == nil {
		return fmt.Errorf("consul: nil instance")
	}
	host, port, err := splitHostPort(inst.Address)
	if err != nil {
		return fmt.Errorf("consul: invalid address %q: %w", inst.Address, err)
	}
	tags := labelsToTags(inst.Labels)
	meta := map[string]string{
		"version":    inst.Version,
		"region":     inst.Region,
		"datacenter": inst.Datacenter,
		"tenant":     inst.Tenant,
	}
	return r.client.RegisterService(inst.ID, inst.Service, host, port, tags, meta)
}

func (r *consulRegistry) Deregister(_ context.Context, id string) error {
	return r.client.DeregisterService(id)
}

func (r *consulRegistry) Discover(_ context.Context, service string) ([]*Instance, error) {
	entries, err := r.client.HealthService(service, "", true)
	if err != nil {
		return nil, err
	}
	out := make([]*Instance, 0, len(entries))
	for _, e := range entries {
		out = append(out, &Instance{
			ID:         e.ID,
			Service:    e.Service,
			Address:    fmt.Sprintf("%s:%d", e.Address, e.Port),
			Labels:     tagsToLabels(e.Tags),
			Version:    e.Meta["version"],
			Region:     e.Meta["region"],
			Datacenter: e.Meta["datacenter"],
			Tenant:     e.Meta["tenant"],
			Weight:     100,
			Healthy:    true,
		})
	}
	return out, nil
}

func (r *consulRegistry) Watch(ctx context.Context, service string) (<-chan []*Instance, error) {
	ch := make(chan []*Instance, 8)
	list, err := r.Discover(ctx, service)
	if err != nil {
		return nil, err
	}
	ch <- list
	// Polling-based watch: real-world callers would use consul's blocking queries.
	go func() {
		defer close(ch)
		// A real implementation would use consul long-polling; this is a placeholder.
		<-ctx.Done()
	}()
	return ch, nil
}
