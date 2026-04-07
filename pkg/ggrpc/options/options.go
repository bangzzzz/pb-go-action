package options

import "time"

// Config_ holds all configurable settings for a ggrpc server or client.
// The trailing underscore avoids a collision with the Config constructor.
type Config_ struct { //nolint:revive
	// Registry
	RegistryAddr    string
	RegistryType    string // "consul", "etcd", "nacos", "dns", "memory"
	RegistryTTL     time.Duration
	RegistryTimeout time.Duration

	// Routing
	Labels     map[string]string
	Version    string
	Region     string
	Datacenter string
	Tenant     string

	// Load balancer
	BalancerType string // "rr", "wrr", "leastconn", "p2c", "ewma"

	// Proximity
	EnableProximity bool

	// Connection pool
	MaxConns        int
	MaxIdleConns    int
	ConnIdleTimeout time.Duration

	// Circuit breaker
	CBFailureThreshold  float64
	CBSuccessThreshold  int
	CBTimeout           time.Duration
	CBMaxRequests        int

	// Interceptors
	Timeout time.Duration
}

// Option is a function that mutates Config_.
type Option func(*Config_)

// Config returns a default Config_ and applies the given options.
func Config(opts []Option) *Config_ { //nolint:revive
	c := &Config_{
		RegistryType:        "memory",
		RegistryTTL:         30 * time.Second,
		RegistryTimeout:     5 * time.Second,
		BalancerType:        "rr",
		MaxConns:            100,
		MaxIdleConns:        10,
		ConnIdleTimeout:     60 * time.Second,
		CBFailureThreshold:  0.5,
		CBSuccessThreshold:  2,
		CBTimeout:           10 * time.Second,
		CBMaxRequests:       1,
		Timeout:             5 * time.Second,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Exported config field accessors.

func (c *Config_) GetRegistryAddr() string           { return c.RegistryAddr }
func (c *Config_) GetRegistryType() string           { return c.RegistryType }
func (c *Config_) GetRegistryTTL() time.Duration     { return c.RegistryTTL }
func (c *Config_) GetRegistryTimeout() time.Duration { return c.RegistryTimeout }
func (c *Config_) GetLabels() map[string]string      { return c.Labels }
func (c *Config_) GetVersion() string                { return c.Version }
func (c *Config_) GetRegion() string                 { return c.Region }
func (c *Config_) GetDatacenter() string             { return c.Datacenter }
func (c *Config_) GetTenant() string                 { return c.Tenant }
func (c *Config_) GetBalancerType() string           { return c.BalancerType }
func (c *Config_) GetEnableProximity() bool          { return c.EnableProximity }
func (c *Config_) GetMaxConns() int                  { return c.MaxConns }
func (c *Config_) GetMaxIdleConns() int              { return c.MaxIdleConns }
func (c *Config_) GetConnIdleTimeout() time.Duration { return c.ConnIdleTimeout }
func (c *Config_) GetCBFailureThreshold() float64   { return c.CBFailureThreshold }
func (c *Config_) GetCBSuccessThreshold() int       { return c.CBSuccessThreshold }
func (c *Config_) GetCBTimeout() time.Duration      { return c.CBTimeout }
func (c *Config_) GetCBMaxRequests() int            { return c.CBMaxRequests }
func (c *Config_) GetTimeout() time.Duration        { return c.Timeout }

// WithRegistryAddr sets the address of the registry backend.
func WithRegistryAddr(addr string) Option {
	return func(c *Config_) { c.RegistryAddr = addr }
}

// WithRegistryType sets the registry type ("consul", "etcd", "nacos", "dns", "memory").
func WithRegistryType(t string) Option {
	return func(c *Config_) { c.RegistryType = t }
}

// WithRegistryTTL sets the TTL for registry entries.
func WithRegistryTTL(d time.Duration) Option {
	return func(c *Config_) { c.RegistryTTL = d }
}

// WithRegistryTimeout sets the timeout for registry operations.
func WithRegistryTimeout(d time.Duration) Option {
	return func(c *Config_) { c.RegistryTimeout = d }
}

// WithLabels sets the label selector used during routing.
func WithLabels(labels map[string]string) Option {
	return func(c *Config_) { c.Labels = labels }
}

// WithVersion sets the version used for version-based routing.
func WithVersion(v string) Option {
	return func(c *Config_) { c.Version = v }
}

// WithRegion sets the region used for region-based routing.
func WithRegion(r string) Option {
	return func(c *Config_) { c.Region = r }
}

// WithDatacenter sets the datacenter used for routing and proximity.
func WithDatacenter(dc string) Option {
	return func(c *Config_) { c.Datacenter = dc }
}

// WithTenant sets the tenant identifier used for routing.
func WithTenant(t string) Option {
	return func(c *Config_) { c.Tenant = t }
}

// WithBalancer sets the load-balancer type.
func WithBalancer(t string) Option {
	return func(c *Config_) { c.BalancerType = t }
}

// WithProximity enables or disables proximity-based selection.
func WithProximity(enable bool) Option {
	return func(c *Config_) { c.EnableProximity = enable }
}

// WithMaxConns sets the maximum number of connections in the pool.
func WithMaxConns(n int) Option {
	return func(c *Config_) { c.MaxConns = n }
}

// WithMaxIdleConns sets the maximum number of idle connections.
func WithMaxIdleConns(n int) Option {
	return func(c *Config_) { c.MaxIdleConns = n }
}

// WithConnIdleTimeout sets the idle timeout for pooled connections.
func WithConnIdleTimeout(d time.Duration) Option {
	return func(c *Config_) { c.ConnIdleTimeout = d }
}

// WithCBFailureThreshold sets the failure-rate threshold to open the circuit.
func WithCBFailureThreshold(t float64) Option {
	return func(c *Config_) { c.CBFailureThreshold = t }
}

// WithCBSuccessThreshold sets the number of consecutive successes to close the circuit.
func WithCBSuccessThreshold(n int) Option {
	return func(c *Config_) { c.CBSuccessThreshold = n }
}

// WithCBTimeout sets the half-open timeout of the circuit breaker.
func WithCBTimeout(d time.Duration) Option {
	return func(c *Config_) { c.CBTimeout = d }
}

// WithCBMaxRequests sets the max requests allowed in the half-open state.
func WithCBMaxRequests(n int) Option {
	return func(c *Config_) { c.CBMaxRequests = n }
}

// WithTimeout sets the per-RPC call timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Config_) { c.Timeout = d }
}
