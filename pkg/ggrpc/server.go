package ggrpc

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"

	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/circuitbreaker"
	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/interceptor"
	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/options"
	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/registry"
)

// NewServer creates a governed gRPC server.
// The quintet identifies this service instance; opts tune the governance features.
func NewServer(quintet *Quintet, opts ...options.Option) (*grpc.Server, error) {
	if quintet == nil {
		return nil, fmt.Errorf("ggrpc: quintet must not be nil")
	}

	cfg := options.Config(opts)

	cb := circuitbreaker.New(circuitbreaker.Config{
		FailureThreshold: cfg.GetCBFailureThreshold(),
		SuccessThreshold: cfg.GetCBSuccessThreshold(),
		Timeout:          cfg.GetCBTimeout(),
		MaxRequests:      cfg.GetCBMaxRequests(),
		MinRequests:      5,
	})

	chain := interceptor.ChainUnaryServer(
		interceptor.ServerRecovery(),
		interceptor.ServerTimeout(cfg.GetTimeout()),
		interceptor.ServerCircuitBreaker(cb),
	)

	srv := grpc.NewServer(grpc.UnaryInterceptor(chain))
	return srv, nil
}

// RegisterAndServe is a convenience function that starts the gRPC server, registers
// the service instance in the registry and begins serving on addr (e.g. ":50051").
func RegisterAndServe(
	ctx context.Context,
	srv *grpc.Server,
	reg registry.Registry,
	inst *registry.Instance,
	addr string,
) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("ggrpc: listen %s: %w", addr, err)
	}

	if reg != nil && inst != nil {
		if err := reg.Register(ctx, inst); err != nil {
			return fmt.Errorf("ggrpc: register: %w", err)
		}
		go func() {
			<-ctx.Done()
			_ = reg.Deregister(context.Background(), inst.ID)
			srv.GracefulStop()
		}()
	}

	return srv.Serve(lis)
}
