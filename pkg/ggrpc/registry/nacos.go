package registry

import (
	"context"
	"fmt"
)

// NacosClient is the minimal interface callers must satisfy to use nacos-backed registry.
type NacosClient interface {
	// RegisterInstance registers a service instance.
	RegisterInstance(serviceName, groupName, ip string, port uint64, metadata map[string]string, weight float64) error
	// DeregisterInstance removes a service instance.
	DeregisterInstance(serviceName, groupName, ip string, port uint64) error
	// GetHealthyInstances returns healthy instances for the service.
	GetHealthyInstances(serviceName, groupName string) ([]NacosInstance, error)
	// Subscribe returns a channel that receives the full instance list on change.
	Subscribe(serviceName, groupName string) (<-chan []NacosInstance, error)
}

// NacosInstance is a minimal representation of a nacos service instance.
type NacosInstance struct {
	InstanceID  string
	ServiceName string
	IP          string
	Port        uint64
	Metadata    map[string]string
	Weight      float64
	Healthy     bool
}

type nacosRegistry struct {
	client    NacosClient
	groupName string
}

// NewNacosRegistry wraps a NacosClient as a Registry.
func NewNacosRegistry(client NacosClient, groupName string) Registry {
	if groupName == "" {
		groupName = "DEFAULT_GROUP"
	}
	return &nacosRegistry{client: client, groupName: groupName}
}

func (r *nacosRegistry) Register(_ context.Context, inst *Instance) error {
	if inst == nil {
		return fmt.Errorf("nacos: nil instance")
	}
	host, port, err := splitHostPort(inst.Address)
	if err != nil {
		return fmt.Errorf("nacos: invalid address %q: %w", inst.Address, err)
	}
	meta := map[string]string{
		"id":         inst.ID,
		"cluster":    inst.Cluster,
		"version":    inst.Version,
		"region":     inst.Region,
		"datacenter": inst.Datacenter,
		"tenant":     inst.Tenant,
	}
	for k, v := range inst.Labels {
		meta["label."+k] = v
	}
	return r.client.RegisterInstance(inst.Service, r.groupName, host, uint64(port), meta, float64(inst.Weight)/100.0)
}

func (r *nacosRegistry) Deregister(_ context.Context, id string) error {
	// Nacos requires service+IP+port; we encode the id into metadata at registration.
	// Without knowing the address here we cannot directly deregister by id alone.
	// Callers should use the full instance reference; this is a best-effort no-op stub.
	_ = id
	return fmt.Errorf("nacos: deregister by id requires address; use NacosClient directly")
}

func (r *nacosRegistry) Discover(_ context.Context, service string) ([]*Instance, error) {
	list, err := r.client.GetHealthyInstances(service, r.groupName)
	if err != nil {
		return nil, err
	}
	out := make([]*Instance, 0, len(list))
	for _, n := range list {
		if !n.Healthy {
			continue
		}
		w := int(n.Weight * 100)
		if w <= 0 {
			w = 100
		}
		out = append(out, &Instance{
			ID:         n.InstanceID,
			Service:    n.ServiceName,
			Address:    fmt.Sprintf("%s:%d", n.IP, n.Port),
			Weight:     w,
			Labels:     nacosMetaToLabels(n.Metadata),
			Version:    n.Metadata["version"],
			Region:     n.Metadata["region"],
			Datacenter: n.Metadata["datacenter"],
			Tenant:     n.Metadata["tenant"],
			Cluster:    n.Metadata["cluster"],
			Healthy:    true,
		})
	}
	return out, nil
}

func (r *nacosRegistry) Watch(ctx context.Context, service string) (<-chan []*Instance, error) {
	events, err := r.client.Subscribe(service, r.groupName)
	if err != nil {
		return nil, err
	}
	ch := make(chan []*Instance, 8)
	list, _ := r.Discover(ctx, service)
	ch <- list

	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case nlist, ok := <-events:
				if !ok {
					return
				}
				out := make([]*Instance, 0, len(nlist))
				for _, n := range nlist {
					if !n.Healthy {
						continue
					}
					w := int(n.Weight * 100)
					if w <= 0 {
						w = 100
					}
					out = append(out, &Instance{
						ID:         n.InstanceID,
						Service:    n.ServiceName,
						Address:    fmt.Sprintf("%s:%d", n.IP, n.Port),
						Weight:     w,
						Labels:     nacosMetaToLabels(n.Metadata),
						Version:    n.Metadata["version"],
						Region:     n.Metadata["region"],
						Datacenter: n.Metadata["datacenter"],
						Tenant:     n.Metadata["tenant"],
						Cluster:    n.Metadata["cluster"],
						Healthy:    true,
					})
				}
				select {
				case ch <- out:
				default:
				}
			}
		}
	}()
	return ch, nil
}

func nacosMetaToLabels(meta map[string]string) map[string]string {
	labels := make(map[string]string)
	for k, v := range meta {
		if len(k) > 6 && k[:6] == "label." {
			labels[k[6:]] = v
		}
	}
	return labels
}
