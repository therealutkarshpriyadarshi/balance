package pool

import (
	"testing"
)

func TestBufferPool(t *testing.T) {
	pool := NewBufferPool(1024)

	// Test Get
	buf := pool.Get()
	if len(buf) != 1024 {
		t.Errorf("Expected buffer size 1024, got %d", len(buf))
	}

	// Test Put and reuse
	pool.Put(buf)
	buf2 := pool.Get()
	if len(buf2) != 1024 {
		t.Errorf("Expected buffer size 1024, got %d", len(buf2))
	}
}

func TestGetOptimalBuffer(t *testing.T) {
	tests := []struct {
		size         int
		expectedPool *BufferPool
	}{
		{100, SmallBufferPool},
		{4 * 1024, SmallBufferPool},
		{5 * 1024, MediumBufferPool},
		{32 * 1024, MediumBufferPool},
		{33 * 1024, LargeBufferPool},
		{64 * 1024, LargeBufferPool},
		{100 * 1024, HugeBufferPool},
		{1024 * 1024, HugeBufferPool},
	}

	for _, tt := range tests {
		buf, pool := GetOptimalBuffer(tt.size)
		if pool != tt.expectedPool {
			t.Errorf("For size %d, expected pool size %d, got %d",
				tt.size, tt.expectedPool.size, pool.size)
		}
		if len(buf) != pool.size {
			t.Errorf("Buffer length mismatch: expected %d, got %d", pool.size, len(buf))
		}
		pool.Put(buf)
	}
}

func TestByteSlicePool(t *testing.T) {
	pool := NewByteSlicePool()

	// Test various sizes
	sizes := []int{100, 1024, 4096, 8192}

	for _, size := range sizes {
		buf := pool.Get(size)
		if len(buf) != size {
			t.Errorf("Expected buffer size %d, got %d", size, len(buf))
		}
		pool.Put(buf)
	}
}

func TestRoundUpPowerOf2(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{7, 8},
		{8, 8},
		{9, 16},
		{1023, 1024},
		{1024, 1024},
		{1025, 2048},
	}

	for _, tt := range tests {
		result := roundUpPowerOf2(tt.input)
		if result != tt.expected {
			t.Errorf("roundUpPowerOf2(%d) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

// Benchmark buffer pool vs regular allocation
func BenchmarkBufferPoolGet(b *testing.B) {
	pool := NewBufferPool(32 * 1024)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		pool.Put(buf)
	}
}

func BenchmarkRegularAllocation(b *testing.B) {
	size := 32 * 1024
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf := make([]byte, size)
		_ = buf
	}
}

func BenchmarkGetOptimalBuffer(b *testing.B) {
	sizes := []int{100, 4096, 32768, 65536, 1048576}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		size := sizes[i%len(sizes)]
		buf, pool := GetOptimalBuffer(size)
		pool.Put(buf)
	}
}

func BenchmarkByteSlicePool(b *testing.B) {
	pool := NewByteSlicePool()
	sizes := []int{1024, 4096, 8192, 32768}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		size := sizes[i%len(sizes)]
		buf := pool.Get(size)
		pool.Put(buf)
	}
}

// Test concurrent access
func TestBufferPoolConcurrent(t *testing.T) {
	pool := NewBufferPool(1024)
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				buf := pool.Get()
				if len(buf) != 1024 {
					t.Errorf("Expected buffer size 1024, got %d", len(buf))
				}
				pool.Put(buf)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
