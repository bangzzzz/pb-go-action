package registry

import (
	"context"
	"fmt"
)

// EtcdClient is the minimal interface callers must satisfy to use etcd-backed registry.
type EtcdClient interface {
	// Put stores value under key with an optional lease TTL in seconds (0 = no TTL).
	Put(ctx context.Context, key, value string, ttlSec int64) error
	// Delete removes the key from etcd.
	Delete(ctx context.Context, key string) error
	// GetWithPrefix returns all key-value pairs whose key starts with prefix.
	GetWithPrefix(ctx context.Context, prefix string) (map[string]string, error)
	// WatchPrefix returns a channel of (key, value) pairs; deletions are signalled
	// with an empty value.
	WatchPrefix(ctx context.Context, prefix string) (<-chan EtcdEvent, error)
}

// EtcdEvent represents a single etcd watch event.
type EtcdEvent struct {
	Key     string
	Value   string
	Deleted bool
}

type etcdRegistry struct {
	client EtcdClient
	ttl    int64
}

// NewEtcdRegistry wraps an arbitrary EtcdClient as a Registry.
func NewEtcdRegistry(client EtcdClient, ttlSec int64) Registry {
	return &etcdRegistry{client: client, ttl: ttlSec}
}

func etcdKey(inst *Instance) string {
	return fmt.Sprintf("/ggrpc/%s/%s", inst.Service, inst.ID)
}

func etcdServicePrefix(service string) string {
	return fmt.Sprintf("/ggrpc/%s/", service)
}

func (r *etcdRegistry) Register(ctx context.Context, inst *Instance) error {
	if inst == nil {
		return fmt.Errorf("etcd: nil instance")
	}
	val, err := marshalInstance(inst)
	if err != nil {
		return err
	}
	return r.client.Put(ctx, etcdKey(inst), val, r.ttl)
}

func (r *etcdRegistry) Deregister(ctx context.Context, id string) error {
	// We do not have the service name here, so we use a prefix scan.
	prefix := "/ggrpc/"
	kvs, err := r.client.GetWithPrefix(ctx, prefix)
	if err != nil {
		return err
	}
	for k := range kvs {
		if len(k) >= len(id) && k[len(k)-len(id):] == id {
			return r.client.Delete(ctx, k)
		}
	}
	return nil
}

func (r *etcdRegistry) Discover(ctx context.Context, service string) ([]*Instance, error) {
	kvs, err := r.client.GetWithPrefix(ctx, etcdServicePrefix(service))
	if err != nil {
		return nil, err
	}
	out := make([]*Instance, 0, len(kvs))
	for _, v := range kvs {
		inst, err := unmarshalInstance(v)
		if err != nil {
			continue
		}
		if inst.Healthy {
			out = append(out, inst)
		}
	}
	return out, nil
}

func (r *etcdRegistry) Watch(ctx context.Context, service string) (<-chan []*Instance, error) {
	events, err := r.client.WatchPrefix(ctx, etcdServicePrefix(service))
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
			case ev, ok := <-events:
				if !ok {
					return
				}
				_ = ev
				// Re-discover on any change event.
				list, _ := r.Discover(ctx, service)
				select {
				case ch <- list:
				default:
				}
			}
		}
	}()
	return ch, nil
}
