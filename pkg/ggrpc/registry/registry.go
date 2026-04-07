// Package registry provides service registration and discovery abstractions.
package registry

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Instance represents a single service instance.
type Instance struct {
	ID         string            // unique instance identifier
	Service    string            // service name
	Cluster    string            // cluster name
	Address    string            // host:port
	Weight     int               // load balancing weight (default 100)
	Labels     map[string]string // arbitrary labels
	Version    string            // service version
	Region     string            // geographic region
	Datacenter string            // datacenter
	Tenant     string            // tenant identifier
	Healthy    bool              // health status
}

// Registry is the interface every registry implementation must satisfy.
type Registry interface {
	// Register publishes an instance to the registry.
	Register(ctx context.Context, inst *Instance) error
	// Deregister removes an instance from the registry.
	Deregister(ctx context.Context, id string) error
	// Discover returns all healthy instances for the given service name.
	Discover(ctx context.Context, service string) ([]*Instance, error)
	// Watch returns a channel that emits the full instance list whenever it changes.
	Watch(ctx context.Context, service string) (<-chan []*Instance, error)
}

// memoryRegistry is an in-memory registry suitable for testing.
type memoryRegistry struct {
	mu        sync.RWMutex
	instances map[string]*Instance                // id → instance
	watchers  map[string][]chan []*Instance        // service → list of watcher channels
	heartbeat map[string]time.Time                // id → last heartbeat
	ttl       time.Duration
	stopCh    chan struct{}
}

// NewMemoryRegistry returns a fully functional in-memory Registry.
func NewMemoryRegistry(ttl time.Duration) Registry {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	r := &memoryRegistry{
		instances: make(map[string]*Instance),
		watchers:  make(map[string][]chan []*Instance),
		heartbeat: make(map[string]time.Time),
		ttl:       ttl,
		stopCh:    make(chan struct{}),
	}
	go r.reaper()
	return r
}

func (r *memoryRegistry) Register(ctx context.Context, inst *Instance) error {
	if inst == nil || inst.ID == "" || inst.Service == "" {
		return fmt.Errorf("registry: instance must have non-empty ID and Service")
	}
	if inst.Weight <= 0 {
		inst.Weight = 100
	}
	inst.Healthy = true

	r.mu.Lock()
	r.instances[inst.ID] = inst
	r.heartbeat[inst.ID] = time.Now()
	r.mu.Unlock()

	r.notify(inst.Service)
	return nil
}

func (r *memoryRegistry) Deregister(_ context.Context, id string) error {
	r.mu.Lock()
	inst, ok := r.instances[id]
	if !ok {
		r.mu.Unlock()
		return nil
	}
	service := inst.Service
	delete(r.instances, id)
	delete(r.heartbeat, id)
	r.mu.Unlock()

	r.notify(service)
	return nil
}

func (r *memoryRegistry) Discover(_ context.Context, service string) ([]*Instance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Instance
	for _, inst := range r.instances {
		if inst.Service == service && inst.Healthy {
			cp := *inst
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *memoryRegistry) Watch(ctx context.Context, service string) (<-chan []*Instance, error) {
	ch := make(chan []*Instance, 8)

	r.mu.Lock()
	r.watchers[service] = append(r.watchers[service], ch)
	r.mu.Unlock()

	// Send current snapshot immediately.
	list, _ := r.Discover(ctx, service)
	ch <- list

	// Close the channel when the context is cancelled.
	go func() {
		<-ctx.Done()
		r.mu.Lock()
		watchers := r.watchers[service]
		for i, w := range watchers {
			if w == ch {
				r.watchers[service] = append(watchers[:i], watchers[i+1:]...)
				break
			}
		}
		r.mu.Unlock()
		close(ch)
	}()

	return ch, nil
}

// Heartbeat refreshes the TTL for the given instance. Call it periodically.
func (r *memoryRegistry) Heartbeat(id string) {
	r.mu.Lock()
	if _, ok := r.instances[id]; ok {
		r.heartbeat[id] = time.Now()
	}
	r.mu.Unlock()
}

// reaper removes instances whose heartbeat has expired.
func (r *memoryRegistry) reaper() {
	ticker := time.NewTicker(r.ttl / 2)
	defer ticker.Stop()
	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.mu.Lock()
			stale := make([]string, 0)
			for id, ts := range r.heartbeat {
				if time.Since(ts) > r.ttl {
					stale = append(stale, id)
				}
			}
			affected := make(map[string]struct{})
			for _, id := range stale {
				if inst, ok := r.instances[id]; ok {
					inst.Healthy = false
					affected[inst.Service] = struct{}{}
				}
				delete(r.instances, id)
				delete(r.heartbeat, id)
			}
			r.mu.Unlock()
			for svc := range affected {
				r.notify(svc)
			}
		}
	}
}

// Stop shuts down the background reaper goroutine.
func (r *memoryRegistry) Stop() {
	close(r.stopCh)
}

func (r *memoryRegistry) notify(service string) {
	r.mu.RLock()
	watchers := make([]chan []*Instance, len(r.watchers[service]))
	copy(watchers, r.watchers[service])
	r.mu.RUnlock()

	if len(watchers) == 0 {
		return
	}

	list, _ := r.Discover(context.Background(), service)
	for _, ch := range watchers {
		select {
		case ch <- list:
		default:
		}
	}
}
