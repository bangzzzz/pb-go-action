// Package interceptor provides gRPC server-side interceptors for service governance.
package interceptor

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/circuitbreaker"
)

// ServerTimeout returns a UnaryServerInterceptor that enforces a per-call timeout.
func ServerTimeout(timeout time.Duration) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if timeout <= 0 {
			return handler(ctx, req)
		}
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return handler(ctx, req)
	}
}

// ServerRecovery returns a UnaryServerInterceptor that recovers from panics.
func ServerRecovery() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = status.Errorf(codes.Internal, "panic: %v", r)
			}
		}()
		return handler(ctx, req)
	}
}

// ServerCircuitBreaker returns a UnaryServerInterceptor that gates requests
// through the given Breaker.
func ServerCircuitBreaker(b *circuitbreaker.Breaker) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if err := b.Allow(); err != nil {
			return nil, status.Errorf(codes.Unavailable, "circuit breaker: %v", err)
		}
		resp, err := handler(ctx, req)
		if err != nil {
			b.Failure()
		} else {
			b.Success()
		}
		return resp, err
	}
}

// ChainUnaryServer chains multiple UnaryServerInterceptors into one.
func ChainUnaryServer(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		chain := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			cur := interceptors[i]
			next := chain
			chain = func(ctx context.Context, req interface{}) (interface{}, error) {
				return cur(ctx, req, info, next)
			}
		}
		return chain(ctx, req)
	}
}
