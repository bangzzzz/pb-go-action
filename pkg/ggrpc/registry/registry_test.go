package registry

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestMemoryRegistry_RegisterDiscover(t *testing.T) {
	r := NewMemoryRegistry(30 * time.Second)
	ctx := context.Background()

	inst := &Instance{
		ID:      "svc-1",
		Service: "my-service",
		Address: "127.0.0.1:8080",
	}
	if err := r.Register(ctx, inst); err != nil {
		t.Fatalf("register: %v", err)
	}

	list, err := r.Discover(ctx, "my-service")
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(list))
	}
	if list[0].ID != "svc-1" {
		t.Errorf("expected ID svc-1, got %s", list[0].ID)
	}
}

func TestMemoryRegistry_Deregister(t *testing.T) {
	r := NewMemoryRegistry(30 * time.Second)
	ctx := context.Background()

	inst := &Instance{ID: "svc-2", Service: "my-service", Address: "127.0.0.1:8081"}
	_ = r.Register(ctx, inst)
	_ = r.Deregister(ctx, "svc-2")

	list, _ := r.Discover(ctx, "my-service")
	if len(list) != 0 {
		t.Errorf("expected 0 instances after deregister, got %d", len(list))
	}
}

func TestMemoryRegistry_Watch(t *testing.T) {
	r := NewMemoryRegistry(30 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := r.Watch(ctx, "watched-service")
	if err != nil {
		t.Fatalf("watch: %v", err)
	}

	// The first message is the current (empty) snapshot.
	initial := <-ch
	if len(initial) != 0 {
		t.Errorf("expected empty initial list, got %d", len(initial))
	}

	inst := &Instance{ID: "svc-w", Service: "watched-service", Address: "127.0.0.1:9000"}
	_ = r.Register(ctx, inst)

	select {
	case list := <-ch:
		if len(list) != 1 {
			t.Errorf("expected 1 instance from watcher, got %d", len(list))
		}
	case <-time.After(2 * time.Second):
		t.Error("timed out waiting for watch notification")
	}
}

func TestMemoryRegistry_MultipleInstances(t *testing.T) {
	r := NewMemoryRegistry(30 * time.Second)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		inst := &Instance{
			ID:      fmt.Sprintf("svc-%d", i),
			Service: "multi",
			Address: fmt.Sprintf("127.0.0.1:9%03d", i),
		}
		if err := r.Register(ctx, inst); err != nil {
			t.Fatalf("register %d: %v", i, err)
		}
	}

	list, _ := r.Discover(ctx, "multi")
	if len(list) != 5 {
		t.Errorf("expected 5 instances, got %d", len(list))
	}
}

func TestMemoryRegistry_InvalidInstance(t *testing.T) {
	r := NewMemoryRegistry(30 * time.Second)
	ctx := context.Background()

	if err := r.Register(ctx, &Instance{}); err == nil {
		t.Error("expected error for empty ID/Service")
	}
}
