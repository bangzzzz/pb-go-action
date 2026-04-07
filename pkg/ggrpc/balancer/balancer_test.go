package balancer

import (
	"testing"

	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/registry"
)

func mkInstances(ids ...string) []*registry.Instance {
	out := make([]*registry.Instance, len(ids))
	for i, id := range ids {
		out[i] = &registry.Instance{ID: id, Service: "svc", Address: "127.0.0.1:800" + id, Weight: 100}
	}
	return out
}

// ── Round Robin ─────────────────────────────────────────────────────────────

func TestRoundRobin_Order(t *testing.T) {
	rr := NewRoundRobin()
	instances := mkInstances("0", "1", "2")

	for cycle := 0; cycle < 2; cycle++ {
		for i, inst := range instances {
			got, err := rr.Pick(instances)
			if err != nil {
				t.Fatalf("cycle %d pick %d: %v", cycle, i, err)
			}
			if got.ID != inst.ID {
				t.Errorf("cycle %d pick %d: want %s, got %s", cycle, i, inst.ID, got.ID)
			}
		}
	}
}

func TestRoundRobin_Empty(t *testing.T) {
	rr := NewRoundRobin()
	if _, err := rr.Pick(nil); err == nil {
		t.Error("expected error on empty list")
	}
}

// ── Weighted Round Robin ─────────────────────────────────────────────────────

func TestWeightedRoundRobin_RespectedWeights(t *testing.T) {
	wrr := NewWeightedRoundRobin()
	instances := []*registry.Instance{
		{ID: "heavy", Service: "svc", Address: "127.0.0.1:8001", Weight: 3},
		{ID: "light", Service: "svc", Address: "127.0.0.1:8002", Weight: 1},
	}

	counts := map[string]int{}
	for i := 0; i < 100; i++ {
		inst, err := wrr.Pick(instances)
		if err != nil {
			t.Fatalf("pick: %v", err)
		}
		counts[inst.ID]++
	}
	// heavy should appear ~75 times, light ~25.
	if counts["heavy"] < 60 || counts["light"] < 10 {
		t.Errorf("weights not respected: %v", counts)
	}
}

func TestWeightedRoundRobin_Empty(t *testing.T) {
	wrr := NewWeightedRoundRobin()
	if _, err := wrr.Pick(nil); err == nil {
		t.Error("expected error on empty list")
	}
}

// ── LeastConn ────────────────────────────────────────────────────────────────

func TestLeastConn_PickLeast(t *testing.T) {
	lc := NewLeastConn()
	instances := mkInstances("a", "b")

	// Pick "a" manually to inflate its counter.
	_, done, _ := lc.PickWithDone(instances)
	_ = done // keep inflight=1 on "a"

	// Next pick should choose "b" (inflight=0).
	got, _ := lc.Pick(instances)
	if got.ID != "b" {
		t.Errorf("expected b (least conn), got %s", got.ID)
	}
}

func TestLeastConn_DoneDecrement(t *testing.T) {
	lc := NewLeastConn()
	instances := mkInstances("x")

	_, done, _ := lc.PickWithDone(instances)
	done(DoneInfo{})

	// After done the counter is back to 0; pick should succeed.
	got, err := lc.Pick(instances)
	if err != nil || got.ID != "x" {
		t.Errorf("unexpected: %v %v", got, err)
	}
}

// ── P2C ──────────────────────────────────────────────────────────────────────

func TestP2C_PickReturnsInstance(t *testing.T) {
	p := NewP2C()
	instances := mkInstances("p", "q", "r")

	for i := 0; i < 20; i++ {
		inst, err := p.Pick(instances)
		if err != nil {
			t.Fatalf("pick: %v", err)
		}
		if inst == nil {
			t.Fatal("nil instance")
		}
	}
}

func TestP2C_SingleInstance(t *testing.T) {
	p := NewP2C()
	instances := mkInstances("only")
	inst, err := p.Pick(instances)
	if err != nil || inst.ID != "only" {
		t.Errorf("single instance: %v %v", inst, err)
	}
}

// ── EWMA ─────────────────────────────────────────────────────────────────────

func TestEWMA_PrefersLowerLoad(t *testing.T) {
	e := NewEWMA()
	instances := mkInstances("slow", "fast")

	// Report a high latency for "slow".
	_, done, _ := e.PickWithDone(instances)
	done(DoneInfo{Latency: int64(500e6)}) // 500 ms

	// Force a few picks; "fast" should win most of the time.
	fastCount := 0
	for i := 0; i < 20; i++ {
		inst, _ := e.Pick(instances)
		if inst.ID == "fast" {
			fastCount++
		}
	}
	if fastCount == 0 {
		t.Error("EWMA should prefer the fast instance")
	}
}
