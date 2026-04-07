// Package connpool provides a gRPC connection pool.
package connpool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Dialer creates a new grpc.ClientConn to the given target.
type Dialer func(ctx context.Context, target string) (*grpc.ClientConn, error)

// entry is a single pooled connection.
type entry struct {
	conn     *grpc.ClientConn
	target   string
	lastUsed time.Time
}

// Pool manages a set of reusable gRPC connections.
type Pool struct {
	mu          sync.Mutex
	conns       []*entry
	maxConns    int
	idleTimeout time.Duration
	dialer      Dialer
	stopCh      chan struct{}
}

// New creates a connection pool.
// maxConns is the maximum number of connections kept open.
// idleTimeout is how long an idle connection is kept before closing.
func New(maxConns int, idleTimeout time.Duration, dialer Dialer) *Pool {
	if dialer == nil {
		dialer = defaultDialer
	}
	p := &Pool{
		maxConns:    maxConns,
		idleTimeout: idleTimeout,
		dialer:      dialer,
		stopCh:      make(chan struct{}),
	}
	go p.reaper()
	return p
}

// Get returns an existing connection to target or dials a new one.
func (p *Pool) Get(ctx context.Context, target string) (*grpc.ClientConn, error) {
	p.mu.Lock()
	for _, e := range p.conns {
		if e.target == target {
			e.lastUsed = time.Now()
			p.mu.Unlock()
			return e.conn, nil
		}
	}

	// Check capacity.
	if len(p.conns) >= p.maxConns {
		p.mu.Unlock()
		return nil, fmt.Errorf("connpool: max connections (%d) reached", p.maxConns)
	}
	p.mu.Unlock()

	conn, err := p.dialer(ctx, target)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.conns = append(p.conns, &entry{conn: conn, target: target, lastUsed: time.Now()})
	p.mu.Unlock()
	return conn, nil
}

// Release is a no-op in this pool: connections are shared and reused.
// Call Close on the pool when you want to shut everything down.
func (p *Pool) Release(_ *grpc.ClientConn) {}

// Close closes all pooled connections and stops the background reaper.
func (p *Pool) Close() error {
	close(p.stopCh)
	p.mu.Lock()
	defer p.mu.Unlock()
	var lastErr error
	for _, e := range p.conns {
		if err := e.conn.Close(); err != nil {
			lastErr = err
		}
	}
	p.conns = nil
	return lastErr
}

// reaper periodically closes idle connections.
func (p *Pool) reaper() {
	ticker := time.NewTicker(p.idleTimeout / 2)
	if p.idleTimeout <= 0 {
		ticker = time.NewTicker(30 * time.Second)
	}
	defer ticker.Stop()
	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.mu.Lock()
			active := p.conns[:0]
			for _, e := range p.conns {
				if time.Since(e.lastUsed) < p.idleTimeout {
					active = append(active, e)
				} else {
					_ = e.conn.Close()
				}
			}
			p.conns = active
			p.mu.Unlock()
		}
	}
}

func defaultDialer(_ context.Context, target string) (*grpc.ClientConn, error) {
	return grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
}
