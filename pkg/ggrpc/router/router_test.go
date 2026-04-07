package router

import (
	"testing"

	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/registry"
)

var testInstances = []*registry.Instance{
	{ID: "a", Service: "svc", Labels: map[string]string{"env": "prod", "zone": "us"}, Version: "v1", Region: "us-east", Datacenter: "dc1", Tenant: "tenant1"},
	{ID: "b", Service: "svc", Labels: map[string]string{"env": "prod", "zone": "eu"}, Version: "v2", Region: "eu-west", Datacenter: "dc2", Tenant: "tenant1"},
	{ID: "c", Service: "svc", Labels: map[string]string{"env": "staging"}, Version: "v1", Region: "us-east", Datacenter: "dc1", Tenant: "tenant2"},
}

func TestLabelRouter_Match(t *testing.T) {
	r := NewLabelRouter(map[string]string{"env": "prod"})
	out := r.Route(testInstances)
	if len(out) != 2 {
		t.Errorf("expected 2, got %d", len(out))
	}
}

func TestLabelRouter_NoMatch_Fallback(t *testing.T) {
	// Chain falls back to original list when no instances match.
	lr := NewLabelRouter(map[string]string{"env": "nonexistent"})
	chain := NewChain(lr)
	out := chain.Route(testInstances)
	// Chain keeps original when filtered is empty.
	if len(out) != 3 {
		t.Errorf("expected 3 (fallback), got %d", len(out))
	}
}

func TestLabelRouter_Empty(t *testing.T) {
	r := NewLabelRouter(nil)
	out := r.Route(testInstances)
	if len(out) != 3 {
		t.Errorf("empty labels should return all, got %d", len(out))
	}
}

func TestVersionRouter(t *testing.T) {
	r := NewVersionRouter("v1")
	out := r.Route(testInstances)
	if len(out) != 2 {
		t.Errorf("expected 2 v1 instances, got %d", len(out))
	}
}

func TestVersionRouter_Empty(t *testing.T) {
	r := NewVersionRouter("")
	out := r.Route(testInstances)
	if len(out) != 3 {
		t.Errorf("empty version should return all, got %d", len(out))
	}
}

func TestRegionRouter(t *testing.T) {
	r := NewRegionRouter("us-east", "", "")
	out := r.Route(testInstances)
	if len(out) != 2 {
		t.Errorf("expected 2 us-east instances, got %d", len(out))
	}
}

func TestRegionRouter_Tenant(t *testing.T) {
	r := NewRegionRouter("", "", "tenant1")
	out := r.Route(testInstances)
	if len(out) != 2 {
		t.Errorf("expected 2 tenant1 instances, got %d", len(out))
	}
}

func TestRegionRouter_DatacenterAndTenant(t *testing.T) {
	r := NewRegionRouter("", "dc1", "tenant1")
	out := r.Route(testInstances)
	if len(out) != 1 || out[0].ID != "a" {
		t.Errorf("expected instance a, got %v", out)
	}
}

func TestChain_MultipleFilters(t *testing.T) {
	chain := NewChain(
		NewLabelRouter(map[string]string{"env": "prod"}),
		NewVersionRouter("v1"),
	)
	out := chain.Route(testInstances)
	if len(out) != 1 || out[0].ID != "a" {
		t.Errorf("expected only instance a, got %v", out)
	}
}
