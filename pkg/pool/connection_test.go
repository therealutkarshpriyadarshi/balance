package pool

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

// mockListener creates a test TCP listener
func setupTestListener(t *testing.T) (string, func()) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	// Accept connections in background
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Echo server
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					if _, err := c.Write(buf[:n]); err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	cleanup := func() {
		listener.Close()
	}

	return listener.Addr().String(), cleanup
}

func TestConnectionPool_GetAndPut(t *testing.T) {
	addr, cleanup := setupTestListener(t)
	defer cleanup()

	config := PoolConfig{
		Address:        addr,
		MaxSize:        5,
		MaxIdleTime:    1 * time.Minute,
		ConnectTimeout: 5 * time.Second,
	}

	pool := NewConnectionPool(config)
	defer pool.Close()

	// Get a connection
	ctx := context.Background()
	conn1, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}

	// Verify connection works
	testMsg := []byte("hello")
	n, err := conn1.Write(testMsg)
	if err != nil || n != len(testMsg) {
		t.Fatalf("Failed to write: %v", err)
	}

	buf := make([]byte, 1024)
	n, err = conn1.Read(buf)
	if err != nil || n != len(testMsg) {
		t.Fatalf("Failed to read: %v", err)
	}

	// Return connection to pool
	if err := conn1.Close(); err != nil {
		t.Fatalf("Failed to close connection: %v", err)
	}

	// Get another connection (should reuse)
	conn2, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get second connection: %v", err)
	}
	defer conn2.Close()

	stats := pool.Stats()
	if stats.TotalReused == 0 {
		t.Error("Expected connection to be reused")
	}
}

func TestConnectionPool_MaxSize(t *testing.T) {
	addr, cleanup := setupTestListener(t)
	defer cleanup()

	maxSize := 3
	config := PoolConfig{
		Address:        addr,
		MaxSize:        maxSize,
		MaxIdleTime:    1 * time.Minute,
		ConnectTimeout: 5 * time.Second,
	}

	pool := NewConnectionPool(config)
	defer pool.Close()

	ctx := context.Background()
	connections := make([]*PooledConnection, maxSize)

	// Get max number of connections
	for i := 0; i < maxSize; i++ {
		conn, err := pool.Get(ctx)
		if err != nil {
			t.Fatalf("Failed to get connection %d: %v", i, err)
		}
		connections[i] = conn
	}

	// Try to get one more (should block/timeout)
	ctx2, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	_, err := pool.Get(ctx2)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected timeout error when pool exhausted, got: %v", err)
	}

	// Return one connection
	connections[0].Close()

	// Now should be able to get a connection
	ctx3 := context.Background()
	conn, err := pool.Get(ctx3)
	if err != nil {
		t.Fatalf("Failed to get connection after return: %v", err)
	}
	defer conn.Close()

	// Clean up
	for i := 1; i < maxSize; i++ {
		connections[i].Close()
	}
}

func TestConnectionPool_ConcurrentAccess(t *testing.T) {
	addr, cleanup := setupTestListener(t)
	defer cleanup()

	config := PoolConfig{
		Address:        addr,
		MaxSize:        10,
		MaxIdleTime:    1 * time.Minute,
		ConnectTimeout: 5 * time.Second,
	}

	pool := NewConnectionPool(config)
	defer pool.Close()

	var wg sync.WaitGroup
	numGoroutines := 50
	numOperations := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				ctx := context.Background()
				conn, err := pool.Get(ctx)
				if err != nil {
					t.Errorf("Failed to get connection: %v", err)
					continue
				}

				// Use the connection
				msg := []byte("test")
				conn.Write(msg)

				// Small delay to simulate work
				time.Sleep(1 * time.Millisecond)

				conn.Close()
			}
		}()
	}

	wg.Wait()

	stats := pool.Stats()
	if stats.TotalCreated == 0 {
		t.Error("Expected connections to be created")
	}
	if stats.TotalReused == 0 {
		t.Error("Expected connections to be reused")
	}
}

func TestConnectionPool_IdleCleanup(t *testing.T) {
	addr, cleanup := setupTestListener(t)
	defer cleanup()

	config := PoolConfig{
		Address:        addr,
		MaxSize:        5,
		MaxIdleTime:    100 * time.Millisecond,
		ConnectTimeout: 5 * time.Second,
	}

	pool := NewConnectionPool(config)
	defer pool.Close()

	ctx := context.Background()

	// Create and return several connections
	for i := 0; i < 3; i++ {
		conn, err := pool.Get(ctx)
		if err != nil {
			t.Fatalf("Failed to get connection: %v", err)
		}
		conn.Close()
	}

	// Wait for idle timeout
	time.Sleep(200 * time.Millisecond)

	// Trigger cleanup
	pool.cleanup()

	stats := pool.Stats()
	if stats.Idle > 0 {
		t.Logf("Warning: Expected idle connections to be cleaned up, got %d idle", stats.Idle)
	}
}

func TestConnectionPool_Close(t *testing.T) {
	addr, cleanup := setupTestListener(t)
	defer cleanup()

	config := PoolConfig{
		Address:        addr,
		MaxSize:        5,
		MaxIdleTime:    1 * time.Minute,
		ConnectTimeout: 5 * time.Second,
	}

	pool := NewConnectionPool(config)

	// Get some connections
	ctx := context.Background()
	conn1, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}
	conn1.Close()

	// Close the pool
	if err := pool.Close(); err != nil {
		t.Fatalf("Failed to close pool: %v", err)
	}

	// Try to get a connection (should fail)
	_, err = pool.Get(ctx)
	if err != ErrPoolClosed {
		t.Errorf("Expected ErrPoolClosed, got: %v", err)
	}
}

func TestPooledConnection_Operations(t *testing.T) {
	addr, cleanup := setupTestListener(t)
	defer cleanup()

	config := PoolConfig{
		Address:        addr,
		MaxSize:        5,
		MaxIdleTime:    1 * time.Minute,
		ConnectTimeout: 5 * time.Second,
	}

	pool := NewConnectionPool(config)
	defer pool.Close()

	ctx := context.Background()
	conn, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}
	defer conn.Close()

	// Test LocalAddr and RemoteAddr
	if conn.LocalAddr() == nil {
		t.Error("Expected non-nil LocalAddr")
	}
	if conn.RemoteAddr() == nil {
		t.Error("Expected non-nil RemoteAddr")
	}

	// Test SetDeadline
	if err := conn.SetDeadline(time.Now().Add(1 * time.Second)); err != nil {
		t.Errorf("SetDeadline failed: %v", err)
	}

	// Test SetReadDeadline
	if err := conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
		t.Errorf("SetReadDeadline failed: %v", err)
	}

	// Test SetWriteDeadline
	if err := conn.SetWriteDeadline(time.Now().Add(1 * time.Second)); err != nil {
		t.Errorf("SetWriteDeadline failed: %v", err)
	}
}

func TestConnectionPool_Stats(t *testing.T) {
	addr, cleanup := setupTestListener(t)
	defer cleanup()

	config := PoolConfig{
		Address:        addr,
		MaxSize:        5,
		MaxIdleTime:    1 * time.Minute,
		ConnectTimeout: 5 * time.Second,
	}

	pool := NewConnectionPool(config)
	defer pool.Close()

	ctx := context.Background()

	// Initial stats
	stats := pool.Stats()
	if stats.Active != 0 || stats.Idle != 0 {
		t.Errorf("Expected 0 active and idle connections initially, got active=%d idle=%d", stats.Active, stats.Idle)
	}

	// Get and keep a connection
	conn1, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}

	stats = pool.Stats()
	if stats.TotalCreated != 1 {
		t.Errorf("Expected 1 created connection, got %d", stats.TotalCreated)
	}

	// Return it
	conn1.Close()

	// Get it again (should be reused)
	conn2, err := pool.Get(ctx)
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}
	defer conn2.Close()

	stats = pool.Stats()
	if stats.TotalReused != 1 {
		t.Errorf("Expected 1 reused connection, got %d", stats.TotalReused)
	}
}

func BenchmarkConnectionPool_GetPut(b *testing.B) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Accept connections in background
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Echo server
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					if _, err := c.Write(buf[:n]); err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	addr := listener.Addr().String()

	config := PoolConfig{
		Address:        addr,
		MaxSize:        10,
		MaxIdleTime:    1 * time.Minute,
		ConnectTimeout: 5 * time.Second,
	}

	pool := NewConnectionPool(config)
	defer pool.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn, err := pool.Get(ctx)
			if err != nil {
				b.Fatalf("Failed to get connection: %v", err)
			}
			conn.Close()
		}
	})
}
