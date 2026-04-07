package ggrpc

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/balancer"
	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/circuitbreaker"
	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/interceptor"
	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/options"
	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/proximity"
	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/registry"
	"github.com/bangzzzz/pb-go-action/pkg/ggrpc/router"
)

// Client is a governed gRPC client that integrates service discovery, routing,
// load balancing, proximity selection, and circuit breaking.
type Client struct {
	quintet  *Quintet
	cfg      *options.Config_
	registry registry.Registry
	balancer balancer.Balancer
	selector *proximity.Selector
	router   *router.Chain
	breaker  *circuitbreaker.Breaker
}

// NewClient creates a governed gRPC client.
func NewClient(quintet *Quintet, opts ...options.Option) (*Client, error) {
	if quintet == nil {
		return nil, fmt.Errorf("ggrpc: quintet must not be nil")
	}

	cfg := options.Config(opts)

	// Default in-memory registry if none is provided via options.
	reg := registry.NewMemoryRegistry(cfg.GetRegistryTTL())

	lb := buildBalancer(cfg.GetBalancerType())

	var sel *proximity.Selector
	if cfg.GetEnableProximity() {
		sel = proximity.NewSelector(proximity.SameDatacenter, cfg.GetDatacenter(), cfg.GetRegion(), nil)
	}

	routerChain := router.NewChain(
		router.NewLabelRouter(cfg.GetLabels()),
		router.NewVersionRouter(cfg.GetVersion()),
		router.NewRegionRouter(cfg.GetRegion(), cfg.GetDatacenter(), cfg.GetTenant()),
	)

	cb := circuitbreaker.New(circuitbreaker.Config{
		FailureThreshold: cfg.GetCBFailureThreshold(),
		SuccessThreshold: cfg.GetCBSuccessThreshold(),
		Timeout:          cfg.GetCBTimeout(),
		MaxRequests:      cfg.GetCBMaxRequests(),
		MinRequests:      5,
	})

	return &Client{
		quintet:  quintet,
		cfg:      cfg,
		registry: reg,
		balancer: lb,
		selector: sel,
		router:   routerChain,
		breaker:  cb,
	}, nil
}

// WithRegistry replaces the default in-memory registry with the given one.
func (c *Client) WithRegistry(r registry.Registry) *Client {
	c.registry = r
	return c
}

// Dial connects to an instance of the callee service. It performs discovery,
// routing, proximity selection, load balancing and circuit breaking.
func (c *Client) Dial(ctx context.Context, extraOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	instances, err := c.registry.Discover(ctx, c.quintet.CalleeService)
	if err != nil {
		return nil, fmt.Errorf("ggrpc: discover %q: %w", c.quintet.CalleeService, err)
	}

	// Apply routing filters.
	instances = c.router.Route(instances)

	// Apply proximity selection.
	if c.selector != nil {
		instances = c.selector.Select(instances)
	}

	// Pick an instance.
	inst, err := c.balancer.Pick(instances)
	if err != nil {
		return nil, fmt.Errorf("ggrpc: pick: %w", err)
	}

	// Circuit breaker check.
	if err := c.breaker.Allow(); err != nil {
		return nil, fmt.Errorf("ggrpc: circuit breaker: %w", err)
	}

	// Build the interceptor chain.
	chain := interceptor.ChainUnaryClient(
		interceptor.ClientTimeout(c.cfg.GetTimeout()),
		interceptor.ClientCircuitBreaker(c.breaker),
		interceptor.ClientQuintetMetadata(
			c.quintet.CallerService,
			c.quintet.CallerCluster,
			c.quintet.CalleeService,
			c.quintet.CalleeCluster,
		),
	)

	defaultOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(chain),
	}
	defaultOpts = append(defaultOpts, extraOpts...)

	conn, err := grpc.NewClient(inst.Address, defaultOpts...)
	if err != nil {
		c.breaker.Failure()
		return nil, fmt.Errorf("ggrpc: dial %s: %w", inst.Address, err)
	}
	c.breaker.Success()
	return conn, nil
}

// buildBalancer constructs a Balancer from a type string.
func buildBalancer(typ string) balancer.Balancer {
	switch typ {
	case "wrr":
		return balancer.NewWeightedRoundRobin()
	case "leastconn":
		return balancer.NewLeastConn()
	case "p2c":
		return balancer.NewP2C()
	case "ewma":
		return balancer.NewEWMA()
	default: // "rr" or unknown
		return balancer.NewRoundRobin()
	}
}
