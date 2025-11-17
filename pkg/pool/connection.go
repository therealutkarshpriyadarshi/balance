package pool

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

var (
	// ErrPoolClosed is returned when attempting to get a connection from a closed pool
	ErrPoolClosed = errors.New("connection pool is closed")

	// ErrPoolExhausted is returned when the pool has reached its maximum size
	ErrPoolExhausted = errors.New("connection pool exhausted")

	// ErrConnectionClosed is returned when trying to use a closed connection
	ErrConnectionClosed = errors.New("connection is closed")
)

// PooledConnection wraps a net.Conn with pooling metadata
type PooledConnection struct {
	conn         net.Conn
	pool         *ConnectionPool
	lastUsed     time.Time
	inUse        bool
	mu           sync.Mutex
	closeOnce    sync.Once
}

// Read implements net.Conn
func (pc *PooledConnection) Read(b []byte) (n int, err error) {
	if pc.conn == nil {
		return 0, ErrConnectionClosed
	}
	n, err = pc.conn.Read(b)
	pc.updateLastUsed()
	return
}

// Write implements net.Conn
func (pc *PooledConnection) Write(b []byte) (n int, err error) {
	if pc.conn == nil {
		return 0, ErrConnectionClosed
	}
	n, err = pc.conn.Write(b)
	pc.updateLastUsed()
	return
}

// Close returns the connection to the pool or closes it if the pool is full
func (pc *PooledConnection) Close() error {
	var err error
	pc.closeOnce.Do(func() {
		if pc.pool != nil && !pc.pool.closed {
			err = pc.pool.put(pc)
		} else if pc.conn != nil {
			err = pc.conn.Close()
			pc.conn = nil
		}
	})
	return err
}

// LocalAddr implements net.Conn
func (pc *PooledConnection) LocalAddr() net.Addr {
	if pc.conn == nil {
		return nil
	}
	return pc.conn.LocalAddr()
}

// RemoteAddr implements net.Conn
func (pc *PooledConnection) RemoteAddr() net.Addr {
	if pc.conn == nil {
		return nil
	}
	return pc.conn.RemoteAddr()
}

// SetDeadline implements net.Conn
func (pc *PooledConnection) SetDeadline(t time.Time) error {
	if pc.conn == nil {
		return ErrConnectionClosed
	}
	return pc.conn.SetDeadline(t)
}

// SetReadDeadline implements net.Conn
func (pc *PooledConnection) SetReadDeadline(t time.Time) error {
	if pc.conn == nil {
		return ErrConnectionClosed
	}
	return pc.conn.SetReadDeadline(t)
}

// SetWriteDeadline implements net.Conn
func (pc *PooledConnection) SetWriteDeadline(t time.Time) error {
	if pc.conn == nil {
		return ErrConnectionClosed
	}
	return pc.conn.SetWriteDeadline(t)
}

func (pc *PooledConnection) updateLastUsed() {
	pc.mu.Lock()
	pc.lastUsed = time.Now()
	pc.mu.Unlock()
}

// MarkInUse marks the connection as in use
func (pc *PooledConnection) MarkInUse() {
	pc.mu.Lock()
	pc.inUse = true
	pc.lastUsed = time.Now()
	pc.mu.Unlock()
}

// IsHealthy checks if the connection is still healthy
func (pc *PooledConnection) IsHealthy() bool {
	if pc.conn == nil {
		return false
	}

	// Try to set a read deadline and check for errors
	// This is a lightweight check
	one := []byte{}
	pc.conn.SetReadDeadline(time.Now())
	_, err := pc.conn.Read(one)
	pc.conn.SetReadDeadline(time.Time{})

	// If we got a timeout, the connection is healthy
	// Any other error means the connection is broken
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	return false
}

// ConnectionPool manages a pool of connections to a backend
type ConnectionPool struct {
	address         string
	maxSize         int
	maxIdleTime     time.Duration
	connectTimeout  time.Duration
	connections     chan *PooledConnection
	mu              sync.RWMutex
	closed          bool
	activeCount     int
	totalCreated    int
	totalReused     int
	factory         func(context.Context) (net.Conn, error)
	cleanupTicker   *time.Ticker
	cleanupDone     chan struct{}
}

// PoolConfig configures a connection pool
type PoolConfig struct {
	Address        string
	MaxSize        int
	MaxIdleTime    time.Duration
	ConnectTimeout time.Duration
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(config PoolConfig) *ConnectionPool {
	if config.MaxSize <= 0 {
		config.MaxSize = 10
	}
	if config.MaxIdleTime <= 0 {
		config.MaxIdleTime = 5 * time.Minute
	}
	if config.ConnectTimeout <= 0 {
		config.ConnectTimeout = 5 * time.Second
	}

	pool := &ConnectionPool{
		address:        config.Address,
		maxSize:        config.MaxSize,
		maxIdleTime:    config.MaxIdleTime,
		connectTimeout: config.ConnectTimeout,
		connections:    make(chan *PooledConnection, config.MaxSize),
		cleanupDone:    make(chan struct{}),
		factory: func(ctx context.Context) (net.Conn, error) {
			dialer := &net.Dialer{
				Timeout: config.ConnectTimeout,
			}
			return dialer.DialContext(ctx, "tcp", config.Address)
		},
	}

	// Start cleanup goroutine
	pool.cleanupTicker = time.NewTicker(30 * time.Second)
	go pool.cleanupIdleConnections()

	return pool
}

// Get retrieves a connection from the pool or creates a new one
func (p *ConnectionPool) Get(ctx context.Context) (*PooledConnection, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, ErrPoolClosed
	}
	p.mu.RUnlock()

	// Try to get an existing connection from the pool
	select {
	case pc := <-p.connections:
		// Check if the connection is still healthy
		if pc.IsHealthy() {
			pc.MarkInUse()
			p.mu.Lock()
			p.totalReused++
			p.mu.Unlock()
			return pc, nil
		}
		// Connection is unhealthy, close it and create a new one
		if pc.conn != nil {
			pc.conn.Close()
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// No connections available
	}

	// Create a new connection
	p.mu.Lock()
	if p.activeCount >= p.maxSize {
		p.mu.Unlock()

		// Wait for a connection to become available or context to cancel
		select {
		case pc := <-p.connections:
			if pc.IsHealthy() {
				pc.MarkInUse()
				p.mu.Lock()
				p.totalReused++
				p.mu.Unlock()
				return pc, nil
			}
			// Unhealthy connection, close it
			if pc.conn != nil {
				pc.conn.Close()
			}
			p.mu.Lock()
			p.activeCount--
			p.mu.Unlock()
			// Try to create a new one
			return p.Get(ctx)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	p.activeCount++
	p.totalCreated++
	p.mu.Unlock()

	// Create new connection
	conn, err := p.factory(ctx)
	if err != nil {
		p.mu.Lock()
		p.activeCount--
		p.mu.Unlock()
		return nil, err
	}

	pc := &PooledConnection{
		conn:     conn,
		pool:     p,
		lastUsed: time.Now(),
		inUse:    true,
	}

	return pc, nil
}

// put returns a connection to the pool
func (p *ConnectionPool) put(pc *PooledConnection) error {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		if pc.conn != nil {
			return pc.conn.Close()
		}
		return nil
	}
	p.mu.RUnlock()

	// Mark as not in use
	pc.mu.Lock()
	pc.inUse = false
	pc.lastUsed = time.Now()
	pc.mu.Unlock()

	// Try to return to pool
	select {
	case p.connections <- pc:
		return nil
	default:
		// Pool is full, close the connection
		p.mu.Lock()
		p.activeCount--
		p.mu.Unlock()

		if pc.conn != nil {
			return pc.conn.Close()
		}
		return nil
	}
}

// cleanupIdleConnections periodically removes idle connections
func (p *ConnectionPool) cleanupIdleConnections() {
	for {
		select {
		case <-p.cleanupTicker.C:
			p.cleanup()
		case <-p.cleanupDone:
			return
		}
	}
}

func (p *ConnectionPool) cleanup() {
	now := time.Now()

	// Check connections in the pool
	for {
		select {
		case pc := <-p.connections:
			pc.mu.Lock()
			idle := now.Sub(pc.lastUsed)
			inUse := pc.inUse
			pc.mu.Unlock()

			if inUse || idle < p.maxIdleTime {
				// Put it back if still valid
				select {
				case p.connections <- pc:
				default:
					// Pool is full somehow, close it
					if pc.conn != nil {
						pc.conn.Close()
					}
					p.mu.Lock()
					p.activeCount--
					p.mu.Unlock()
				}
			} else {
				// Connection is too old, close it
				if pc.conn != nil {
					pc.conn.Close()
				}
				p.mu.Lock()
				p.activeCount--
				p.mu.Unlock()
			}
		default:
			return
		}
	}
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	// Stop cleanup goroutine
	p.cleanupTicker.Stop()
	close(p.cleanupDone)

	// Close all connections
	close(p.connections)
	for pc := range p.connections {
		if pc.conn != nil {
			pc.conn.Close()
		}
	}

	return nil
}

// Stats returns pool statistics
func (p *ConnectionPool) Stats() PoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return PoolStats{
		Active:       p.activeCount,
		Idle:         len(p.connections),
		TotalCreated: p.totalCreated,
		TotalReused:  p.totalReused,
		MaxSize:      p.maxSize,
	}
}

// PoolStats contains pool statistics
type PoolStats struct {
	Active       int
	Idle         int
	TotalCreated int
	TotalReused  int
	MaxSize      int
}
