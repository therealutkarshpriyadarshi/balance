package optimize

import (
	"sync"
)

// StringInterner provides string interning to reduce memory allocations
// for frequently used strings like HTTP header names
type StringInterner struct {
	mu      sync.RWMutex
	strings map[string]string
	maxSize int
	stats   InternStats
}

// InternStats tracks string interning statistics
type InternStats struct {
	Hits       uint64
	Misses     uint64
	Evictions  uint64
	TotalSize  int
	UniqueKeys int
}

// NewStringInterner creates a new string interner
func NewStringInterner(maxSize int) *StringInterner {
	if maxSize <= 0 {
		maxSize = 10000 // Default max size
	}

	return &StringInterner{
		strings: make(map[string]string, 100),
		maxSize: maxSize,
	}
}

// Intern interns a string, returning a canonical version
func (si *StringInterner) Intern(s string) string {
	// Fast path: check if already interned (read lock)
	si.mu.RLock()
	if interned, exists := si.strings[s]; exists {
		si.mu.RUnlock()
		si.stats.Hits++
		return interned
	}
	si.mu.RUnlock()

	// Slow path: add to intern map (write lock)
	si.mu.Lock()
	defer si.mu.Unlock()

	// Double-check after acquiring write lock
	if interned, exists := si.strings[s]; exists {
		si.stats.Hits++
		return interned
	}

	// Check if we need to evict
	if len(si.strings) >= si.maxSize {
		si.evictLRU()
	}

	// Intern the string
	si.strings[s] = s
	si.stats.Misses++
	si.stats.TotalSize += len(s)
	si.stats.UniqueKeys = len(si.strings)

	return s
}

// InternBytes interns a byte slice as a string
func (si *StringInterner) InternBytes(b []byte) string {
	s := string(b)
	return si.Intern(s)
}

// evictLRU evicts oldest entries (simplified LRU)
func (si *StringInterner) evictLRU() {
	// Simple strategy: evict 10% of entries
	toEvict := si.maxSize / 10
	if toEvict == 0 {
		toEvict = 1
	}

	evicted := 0
	for key := range si.strings {
		delete(si.strings, key)
		si.stats.TotalSize -= len(key)
		evicted++
		if evicted >= toEvict {
			break
		}
	}

	si.stats.Evictions += uint64(evicted)
	si.stats.UniqueKeys = len(si.strings)
}

// Clear clears all interned strings
func (si *StringInterner) Clear() {
	si.mu.Lock()
	defer si.mu.Unlock()

	si.strings = make(map[string]string, 100)
	si.stats = InternStats{}
}

// Stats returns current statistics
func (si *StringInterner) Stats() InternStats {
	si.mu.RLock()
	defer si.mu.RUnlock()

	stats := si.stats
	stats.UniqueKeys = len(si.strings)
	return stats
}

// Size returns the number of interned strings
func (si *StringInterner) Size() int {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return len(si.strings)
}

// Contains checks if a string is already interned
func (si *StringInterner) Contains(s string) bool {
	si.mu.RLock()
	defer si.mu.RUnlock()
	_, exists := si.strings[s]
	return exists
}

// HTTPHeaderInterner is a specialized interner for common HTTP headers
type HTTPHeaderInterner struct {
	interner *StringInterner
}

// Common HTTP header names (pre-interned)
var commonHeaders = []string{
	"Accept",
	"Accept-Encoding",
	"Accept-Language",
	"Authorization",
	"Cache-Control",
	"Connection",
	"Content-Encoding",
	"Content-Length",
	"Content-Type",
	"Cookie",
	"Date",
	"Host",
	"If-Modified-Since",
	"If-None-Match",
	"Last-Modified",
	"Location",
	"Referer",
	"Server",
	"Set-Cookie",
	"Transfer-Encoding",
	"User-Agent",
	"Vary",
	"X-Forwarded-For",
	"X-Forwarded-Proto",
	"X-Real-IP",
	"X-Request-ID",
}

// NewHTTPHeaderInterner creates a new HTTP header interner
func NewHTTPHeaderInterner() *HTTPHeaderInterner {
	interner := NewStringInterner(1000)

	// Pre-intern common headers
	for _, header := range commonHeaders {
		interner.Intern(header)
	}

	return &HTTPHeaderInterner{
		interner: interner,
	}
}

// InternHeader interns an HTTP header name
func (hhi *HTTPHeaderInterner) InternHeader(name string) string {
	return hhi.interner.Intern(name)
}

// InternHeaderBytes interns an HTTP header name from bytes
func (hhi *HTTPHeaderInterner) InternHeaderBytes(name []byte) string {
	return hhi.interner.InternBytes(name)
}

// Stats returns interning statistics
func (hhi *HTTPHeaderInterner) Stats() InternStats {
	return hhi.interner.Stats()
}

// Global HTTP header interner
var globalHeaderInterner *HTTPHeaderInterner
var headerInternerOnce sync.Once

// GetGlobalHeaderInterner returns the global HTTP header interner
func GetGlobalHeaderInterner() *HTTPHeaderInterner {
	headerInternerOnce.Do(func() {
		globalHeaderInterner = NewHTTPHeaderInterner()
	})
	return globalHeaderInterner
}

// InternHeader interns an HTTP header name using the global interner
func InternHeader(name string) string {
	return GetGlobalHeaderInterner().InternHeader(name)
}

// InternHeaderBytes interns an HTTP header name from bytes using the global interner
func InternHeaderBytes(name []byte) string {
	return GetGlobalHeaderInterner().InternHeaderBytes(name)
}

// ByteString is a zero-allocation string type for temporary string views
type ByteString []byte

// String converts ByteString to string (causes allocation)
func (bs ByteString) String() string {
	return string(bs)
}

// Equal compares ByteString with a string without allocation
func (bs ByteString) Equal(s string) bool {
	if len(bs) != len(s) {
		return false
	}
	for i := 0; i < len(bs); i++ {
		if bs[i] != s[i] {
			return false
		}
	}
	return true
}

// EqualBytes compares two ByteStrings
func (bs ByteString) EqualBytes(other []byte) bool {
	if len(bs) != len(other) {
		return false
	}
	for i := 0; i < len(bs); i++ {
		if bs[i] != other[i] {
			return false
		}
	}
	return true
}
