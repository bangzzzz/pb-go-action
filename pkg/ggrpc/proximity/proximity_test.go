package proximity

import (
	"testing"

	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/registry"
)

var proxInstances = []*registry.Instance{
	{ID: "dc1-a", Datacenter: "dc1", Region: "us-east"},
	{ID: "dc1-b", Datacenter: "dc1", Region: "us-east"},
	{ID: "dc2-a", Datacenter: "dc2", Region: "eu-west"},
}

func TestSameDatacenter(t *testing.T) {
	sel := NewSelector(SameDatacenter, "dc1", "", nil)
	out := sel.Select(proxInstances)
	if out[0].Datacenter != "dc1" || out[1].Datacenter != "dc1" {
		t.Errorf("first two should be dc1, got %v %v", out[0].Datacenter, out[1].Datacenter)
	}
	if out[2].Datacenter != "dc2" {
		t.Errorf("last should be dc2, got %v", out[2].Datacenter)
	}
}

func TestSameDatacenter_Fallback(t *testing.T) {
	sel := NewSelector(SameDatacenter, "dc99", "", nil)
	out := sel.Select(proxInstances)
	if len(out) != 3 {
		t.Errorf("fallback should return all, got %d", len(out))
	}
}

func TestSameRegion(t *testing.T) {
	sel := NewSelector(SameRegion, "", "eu-west", nil)
	out := sel.Select(proxInstances)
	if out[0].ID != "dc2-a" {
		t.Errorf("expected dc2-a first, got %s", out[0].ID)
	}
}

func TestLatencyFirst(t *testing.T) {
	latency := map[string]int64{
		"dc1-a": 10,
		"dc1-b": 5,
		"dc2-a": 20,
	}
	sel := NewSelector(LatencyFirst, "", "", latency)
	out := sel.Select(proxInstances)
	if out[0].ID != "dc1-b" || out[1].ID != "dc1-a" || out[2].ID != "dc2-a" {
		t.Errorf("unexpected order: %v %v %v", out[0].ID, out[1].ID, out[2].ID)
	}
}
