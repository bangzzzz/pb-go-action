package interceptor

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/circuitbreaker"
	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/quintet_key"
)

// ClientTimeout returns a UnaryClientInterceptor that enforces a per-call timeout.
func ClientTimeout(timeout time.Duration) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		if timeout <= 0 {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// ClientCircuitBreaker returns a UnaryClientInterceptor that gates outbound
// requests through the given Breaker.
func ClientCircuitBreaker(b *circuitbreaker.Breaker) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		if err := b.Allow(); err != nil {
			return status.Errorf(codes.Unavailable, "circuit breaker: %v", err)
		}
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			b.Failure()
		} else {
			b.Success()
		}
		return err
	}
}

// ClientQuintetMetadata injects the quintet fields into outgoing metadata so
// the server can use them for tracing and routing.
func ClientQuintetMetadata(callerService, callerCluster, calleeService, calleeCluster string) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		md := metadata.Pairs(
			quintet_key.CallerService, callerService,
			quintet_key.CallerCluster, callerCluster,
			quintet_key.CalleeService, calleeService,
			quintet_key.CalleeCluster, calleeCluster,
		)
		ctx = metadata.NewOutgoingContext(ctx, md)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// ChainUnaryClient chains multiple UnaryClientInterceptors into one.
func ChainUnaryClient(interceptors ...grpc.UnaryClientInterceptor) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		chain := invoker
		for i := len(interceptors) - 1; i >= 0; i-- {
			cur := interceptors[i]
			next := chain
			chain = func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
				return cur(ctx, method, req, reply, cc, next, opts...)
			}
		}
		return chain(ctx, method, req, reply, cc, opts...)
	}
}
