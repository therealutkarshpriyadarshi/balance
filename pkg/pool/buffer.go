package pool

import (
	"sync"
)

// BufferPool manages a pool of byte slices to reduce allocations
type BufferPool struct {
	pool     *sync.Pool
	size     int
	maxPools int // Maximum number of different pool sizes
}

// Global buffer pools for common sizes
var (
	// Small buffer pool (4KB) - for small messages, headers
	SmallBufferPool = NewBufferPool(4 * 1024)

	// Medium buffer pool (32KB) - for typical HTTP requests/responses
	MediumBufferPool = NewBufferPool(32 * 1024)

	// Large buffer pool (64KB) - for large payloads
	LargeBufferPool = NewBufferPool(64 * 1024)

	// Huge buffer pool (1MB) - for very large transfers
	HugeBufferPool = NewBufferPool(1024 * 1024)
)

// NewBufferPool creates a new buffer pool with the specified buffer size
func NewBufferPool(size int) *BufferPool {
	bp := &BufferPool{
		size: size,
	}

	bp.pool = &sync.Pool{
		New: func() interface{} {
			buf := make([]byte, bp.size)
			return &buf
		},
	}

	return bp
}

// Get retrieves a buffer from the pool
func (bp *BufferPool) Get() []byte {
	bufPtr := bp.pool.Get().(*[]byte)
	return (*bufPtr)[:bp.size]
}

// Put returns a buffer to the pool
func (bp *BufferPool) Put(buf []byte) {
	if cap(buf) < bp.size {
		// Buffer is too small, don't return it to the pool
		return
	}

	// Reset the slice to its full capacity
	buf = buf[:cap(buf)]

	// Zero out the buffer to prevent information leakage
	// Only zero the first part to balance security and performance
	for i := 0; i < len(buf) && i < 1024; i++ {
		buf[i] = 0
	}

	bp.pool.Put(&buf)
}

// Size returns the buffer size for this pool
func (bp *BufferPool) Size() int {
	return bp.size
}

// GetOptimalBuffer returns a buffer from the most appropriate pool based on size needed
func GetOptimalBuffer(sizeNeeded int) ([]byte, *BufferPool) {
	switch {
	case sizeNeeded <= SmallBufferPool.size:
		return SmallBufferPool.Get(), SmallBufferPool
	case sizeNeeded <= MediumBufferPool.size:
		return MediumBufferPool.Get(), MediumBufferPool
	case sizeNeeded <= LargeBufferPool.size:
		return LargeBufferPool.Get(), LargeBufferPool
	default:
		return HugeBufferPool.Get(), HugeBufferPool
	}
}

// ByteSlicePool is a specialized pool for variable-sized byte slices
type ByteSlicePool struct {
	pools map[int]*sync.Pool
	mu    sync.RWMutex
}

// NewByteSlicePool creates a new byte slice pool
func NewByteSlicePool() *ByteSlicePool {
	return &ByteSlicePool{
		pools: make(map[int]*sync.Pool),
	}
}

// Get retrieves or creates a byte slice of the specified size
func (bsp *ByteSlicePool) Get(size int) []byte {
	// Round up to nearest power of 2 for better pooling
	poolSize := roundUpPowerOf2(size)

	bsp.mu.RLock()
	pool, exists := bsp.pools[poolSize]
	bsp.mu.RUnlock()

	if !exists {
		bsp.mu.Lock()
		// Double-check after acquiring write lock
		pool, exists = bsp.pools[poolSize]
		if !exists {
			pool = &sync.Pool{
				New: func() interface{} {
					buf := make([]byte, poolSize)
					return &buf
				},
			}
			bsp.pools[poolSize] = pool
		}
		bsp.mu.Unlock()
	}

	bufPtr := pool.Get().(*[]byte)
	return (*bufPtr)[:size]
}

// Put returns a byte slice to the pool
func (bsp *ByteSlicePool) Put(buf []byte) {
	if len(buf) == 0 {
		return
	}

	poolSize := roundUpPowerOf2(cap(buf))

	bsp.mu.RLock()
	pool, exists := bsp.pools[poolSize]
	bsp.mu.RUnlock()

	if exists {
		// Reset the slice to its full capacity
		buf = buf[:cap(buf)]
		pool.Put(&buf)
	}
}

// roundUpPowerOf2 rounds up to the nearest power of 2
func roundUpPowerOf2(n int) int {
	if n <= 0 {
		return 1
	}

	// If already power of 2, return as is
	if n&(n-1) == 0 {
		return n
	}

	// Find the next power of 2
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n++

	return n
}

// SharedBufferPool is a global byte slice pool instance
var SharedBufferPool = NewByteSlicePool()
