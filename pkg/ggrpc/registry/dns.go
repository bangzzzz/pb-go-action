package registry

import (
	"context"
	"fmt"
	"net"
)

// dnsRegistry resolves service instances from DNS SRV records.
type dnsRegistry struct {
	resolver *net.Resolver
}

// NewDNSRegistry returns a Registry backed by DNS SRV lookups.
// Pass nil to use the default system resolver.
func NewDNSRegistry(resolver *net.Resolver) Registry {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return &dnsRegistry{resolver: resolver}
}

func (r *dnsRegistry) Register(_ context.Context, _ *Instance) error {
	return fmt.Errorf("dns: registration is managed externally")
}

func (r *dnsRegistry) Deregister(_ context.Context, _ string) error {
	return fmt.Errorf("dns: deregistration is managed externally")
}

func (r *dnsRegistry) Discover(ctx context.Context, service string) ([]*Instance, error) {
	_, addrs, err := r.resolver.LookupSRV(ctx, "", "", service)
	if err != nil {
		// Fall back to A/AAAA lookup on port 0.
		ips, err2 := r.resolver.LookupHost(ctx, service)
		if err2 != nil {
			return nil, fmt.Errorf("dns: lookup %q: %w", service, err)
		}
		out := make([]*Instance, 0, len(ips))
		for _, ip := range ips {
			out = append(out, &Instance{
				ID:      fmt.Sprintf("%s:%s", service, ip),
				Service: service,
				Address: ip,
				Weight:  100,
				Healthy: true,
			})
		}
		return out, nil
	}
	out := make([]*Instance, 0, len(addrs))
	for _, srv := range addrs {
		addr := fmt.Sprintf("%s:%d", srv.Target, srv.Port)
		out = append(out, &Instance{
			ID:      addr,
			Service: service,
			Address: addr,
			Weight:  int(srv.Weight),
			Healthy: true,
		})
	}
	return out, nil
}

func (r *dnsRegistry) Watch(ctx context.Context, service string) (<-chan []*Instance, error) {
	ch := make(chan []*Instance, 4)
	list, err := r.Discover(ctx, service)
	if err != nil {
		return nil, err
	}
	ch <- list
	// DNS does not push changes; the channel will be closed when the context is done.
	go func() {
		defer close(ch)
		<-ctx.Done()
	}()
	return ch, nil
}
