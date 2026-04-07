package connpool

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func noopDialer(_ context.Context, target string) (*grpc.ClientConn, error) {
	return grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

func TestPool_GetSameTarget(t *testing.T) {
	p := New(10, time.Minute, noopDialer)
	defer p.Close()

	ctx := context.Background()
	conn1, err := p.Get(ctx, "127.0.0.1:19999")
	if err != nil {
		t.Fatalf("first get: %v", err)
	}
	conn2, err := p.Get(ctx, "127.0.0.1:19999")
	if err != nil {
		t.Fatalf("second get: %v", err)
	}
	if conn1 != conn2 {
		t.Error("expected same connection for same target")
	}
}

func TestPool_MaxConns(t *testing.T) {
	p := New(2, time.Minute, noopDialer)
	defer p.Close()

	ctx := context.Background()
	if _, err := p.Get(ctx, "127.0.0.1:19991"); err != nil {
		t.Fatalf("get 1: %v", err)
	}
	if _, err := p.Get(ctx, "127.0.0.1:19992"); err != nil {
		t.Fatalf("get 2: %v", err)
	}
	if _, err := p.Get(ctx, "127.0.0.1:19993"); err == nil {
		t.Error("expected error when max connections reached")
	}
}

func TestPool_Close(t *testing.T) {
	p := New(5, time.Minute, noopDialer)
	ctx := context.Background()
	_, _ = p.Get(ctx, "127.0.0.1:19994")
	if err := p.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}
