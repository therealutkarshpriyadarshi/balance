package pool

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestGoroutinePool(t *testing.T) {
	config := GoroutinePoolConfig{
		MaxWorkers:  5,
		MaxIdleTime: 100 * time.Millisecond,
		QueueSize:   10,
	}

	pool := NewGoroutinePool(config)
	defer pool.Close()

	var counter int32

	// Submit some tasks
	for i := 0; i < 10; i++ {
		err := pool.Submit(func() {
			atomic.AddInt32(&counter, 1)
		})
		if err != nil {
			t.Errorf("Failed to submit task: %v", err)
		}
	}

	// Wait for tasks to complete
	time.Sleep(200 * time.Millisecond)

	if atomic.LoadInt32(&counter) != 10 {
		t.Errorf("Expected counter to be 10, got %d", counter)
	}

	stats := pool.Stats()
	if stats.Completed != 10 {
		t.Errorf("Expected 10 completed tasks, got %d", stats.Completed)
	}
}

func TestGoroutinePoolClosed(t *testing.T) {
	pool := NewGoroutinePool(DefaultGoroutinePoolConfig())
	pool.Close()

	err := pool.Submit(func() {})
	if err != ErrGoroutinePoolClosed {
		t.Errorf("Expected ErrGoroutinePoolClosed, got %v", err)
	}
}

func TestGoroutinePoolTimeout(t *testing.T) {
	config := GoroutinePoolConfig{
		MaxWorkers:  1,
		MaxIdleTime: 100 * time.Millisecond,
		QueueSize:   1,
	}

	pool := NewGoroutinePool(config)
	defer pool.Close()

	// Fill the queue and block the worker
	pool.Submit(func() {
		time.Sleep(200 * time.Millisecond)
	})
	pool.Submit(func() {})

	// This should timeout
	err := pool.SubmitWithTimeout(func() {}, 10*time.Millisecond)
	if err != ErrGoroutinePoolTimeout {
		t.Errorf("Expected ErrGoroutinePoolTimeout, got %v", err)
	}
}

func TestGoroutinePoolContext(t *testing.T) {
	config := GoroutinePoolConfig{
		MaxWorkers:  1,
		MaxIdleTime: 100 * time.Millisecond,
		QueueSize:   1,
	}

	pool := NewGoroutinePool(config)
	defer pool.Close()

	// Fill the queue and block the worker
	pool.Submit(func() {
		time.Sleep(200 * time.Millisecond)
	})
	pool.Submit(func() {})

	// This should be cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := pool.SubmitWithContext(ctx, func() {})
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

func TestGoroutinePoolPanicRecovery(t *testing.T) {
	pool := NewGoroutinePool(DefaultGoroutinePoolConfig())
	defer pool.Close()

	var counter int32

	// Submit a task that panics
	pool.Submit(func() {
		panic("test panic")
	})

	// Submit another task - pool should still work
	pool.Submit(func() {
		atomic.AddInt32(&counter, 1)
	})

	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&counter) != 1 {
		t.Errorf("Pool should recover from panic and continue working")
	}
}

func TestGoroutinePoolStats(t *testing.T) {
	config := GoroutinePoolConfig{
		MaxWorkers:  2,
		MaxIdleTime: 100 * time.Millisecond,
		QueueSize:   5,
	}

	pool := NewGoroutinePool(config)
	defer pool.Close()

	// Submit tasks
	for i := 0; i < 5; i++ {
		pool.Submit(func() {
			time.Sleep(50 * time.Millisecond)
		})
	}

	time.Sleep(10 * time.Millisecond)

	stats := pool.Stats()
	if stats.Submitted != 5 {
		t.Errorf("Expected 5 submitted tasks, got %d", stats.Submitted)
	}

	// Wait for completion
	time.Sleep(200 * time.Millisecond)

	stats = pool.Stats()
	if stats.Completed != 5 {
		t.Errorf("Expected 5 completed tasks, got %d", stats.Completed)
	}
}

func TestGoroutinePoolNonBlocking(t *testing.T) {
	config := GoroutinePoolConfig{
		MaxWorkers:  1,
		MaxIdleTime: 100 * time.Millisecond,
		QueueSize:   1,
		NonBlocking: true,
	}

	pool := NewGoroutinePool(config)
	defer pool.Close()

	// Fill the queue and block the worker
	pool.Submit(func() {
		time.Sleep(200 * time.Millisecond)
	})
	pool.Submit(func() {})

	// This should fail immediately in non-blocking mode
	err := pool.Submit(func() {})
	if err != ErrGoroutinePoolTimeout {
		t.Errorf("Expected ErrGoroutinePoolTimeout in non-blocking mode, got %v", err)
	}
}

func TestDefaultPool(t *testing.T) {
	_ = GetDefaultPool() // Ensure pool is initialized

	var counter int32
	err := Submit(func() {
		atomic.AddInt32(&counter, 1)
	})

	if err != nil {
		t.Errorf("Failed to submit to default pool: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&counter) != 1 {
		t.Errorf("Expected counter to be 1, got %d", counter)
	}
}

func TestGoroutinePoolConcurrent(t *testing.T) {
	pool := NewGoroutinePool(DefaultGoroutinePoolConfig())
	defer pool.Close()

	var counter int32
	tasks := 100

	// Submit many tasks concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < tasks/10; j++ {
				pool.Submit(func() {
					atomic.AddInt32(&counter, 1)
					time.Sleep(1 * time.Millisecond)
				})
			}
			done <- true
		}()
	}

	// Wait for all submissions
	for i := 0; i < 10; i++ {
		<-done
	}

	// Wait for all tasks to complete
	time.Sleep(500 * time.Millisecond)

	if atomic.LoadInt32(&counter) != int32(tasks) {
		t.Errorf("Expected counter to be %d, got %d", tasks, counter)
	}
}

// Benchmarks
func BenchmarkGoroutinePoolSubmit(b *testing.B) {
	pool := NewGoroutinePool(DefaultGoroutinePoolConfig())
	defer pool.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pool.Submit(func() {})
		}
	})
}

func BenchmarkGoroutinePoolExecute(b *testing.B) {
	pool := NewGoroutinePool(DefaultGoroutinePoolConfig())
	defer pool.Close()

	var counter int32
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pool.Submit(func() {
			atomic.AddInt32(&counter, 1)
		})
	}

	pool.Close()
}

func BenchmarkRawGoroutine(b *testing.B) {
	var counter int32
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		go func() {
			atomic.AddInt32(&counter, 1)
		}()
	}
}
