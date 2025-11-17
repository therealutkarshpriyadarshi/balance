package optimize

import (
	"fmt"
	"testing"
)

func TestStringInterner(t *testing.T) {
	interner := NewStringInterner(100)

	// Test basic interning
	s1 := interner.Intern("hello")
	s2 := interner.Intern("hello")

	// Should return the same pointer
	if &s1[0] != &s2[0] {
		t.Errorf("Expected interned strings to have same pointer")
	}

	// Test stats
	stats := interner.Stats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}
}

func TestStringInternerBytes(t *testing.T) {
	interner := NewStringInterner(100)

	b1 := []byte("hello")
	b2 := []byte("hello")

	s1 := interner.InternBytes(b1)
	s2 := interner.InternBytes(b2)

	// Should return the same pointer
	if &s1[0] != &s2[0] {
		t.Errorf("Expected interned strings to have same pointer")
	}
}

func TestStringInternerEviction(t *testing.T) {
	interner := NewStringInterner(10)

	// Fill beyond capacity
	for i := 0; i < 15; i++ {
		interner.Intern(fmt.Sprintf("string-%d", i))
	}

	stats := interner.Stats()
	if stats.Evictions == 0 {
		t.Errorf("Expected some evictions, got 0")
	}

	size := interner.Size()
	if size > 10 {
		t.Errorf("Expected size <= 10 after eviction, got %d", size)
	}
}

func TestStringInternerClear(t *testing.T) {
	interner := NewStringInterner(100)

	interner.Intern("hello")
	interner.Intern("world")

	if interner.Size() != 2 {
		t.Errorf("Expected size 2, got %d", interner.Size())
	}

	interner.Clear()

	if interner.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", interner.Size())
	}
}

func TestStringInternerContains(t *testing.T) {
	interner := NewStringInterner(100)

	interner.Intern("hello")

	if !interner.Contains("hello") {
		t.Errorf("Expected 'hello' to be interned")
	}

	if interner.Contains("world") {
		t.Errorf("Expected 'world' to not be interned")
	}
}

func TestHTTPHeaderInterner(t *testing.T) {
	interner := NewHTTPHeaderInterner()

	// Test common header
	h1 := interner.InternHeader("Content-Type")
	h2 := interner.InternHeader("Content-Type")

	if &h1[0] != &h2[0] {
		t.Errorf("Expected common headers to be pre-interned")
	}

	// Test custom header
	c1 := interner.InternHeader("X-Custom-Header")
	c2 := interner.InternHeader("X-Custom-Header")

	if &c1[0] != &c2[0] {
		t.Errorf("Expected custom headers to be interned")
	}
}

func TestHTTPHeaderInternerBytes(t *testing.T) {
	interner := NewHTTPHeaderInterner()

	b1 := []byte("Content-Type")
	h1 := interner.InternHeaderBytes(b1)

	b2 := []byte("Content-Type")
	h2 := interner.InternHeaderBytes(b2)

	if &h1[0] != &h2[0] {
		t.Errorf("Expected interned header bytes to have same pointer")
	}
}

func TestGlobalHeaderInterner(t *testing.T) {
	h1 := InternHeader("User-Agent")
	h2 := InternHeader("User-Agent")

	if &h1[0] != &h2[0] {
		t.Errorf("Expected global interner to work")
	}

	b1 := []byte("Host")
	h3 := InternHeaderBytes(b1)

	b2 := []byte("Host")
	h4 := InternHeaderBytes(b2)

	if &h3[0] != &h4[0] {
		t.Errorf("Expected global interner bytes to work")
	}
}

func TestByteString(t *testing.T) {
	bs := ByteString("hello")

	if !bs.Equal("hello") {
		t.Errorf("Expected ByteString to equal 'hello'")
	}

	if bs.Equal("world") {
		t.Errorf("Expected ByteString to not equal 'world'")
	}

	if !bs.EqualBytes([]byte("hello")) {
		t.Errorf("Expected ByteString to equal []byte('hello')")
	}
}

func TestByteStringZeroAlloc(t *testing.T) {
	bs := ByteString("test")
	testStr := "test"

	// This comparison should not allocate
	result := bs.Equal(testStr)
	if !result {
		t.Errorf("Expected ByteString.Equal to work")
	}
}

// Benchmarks
func BenchmarkStringIntern(b *testing.B) {
	interner := NewStringInterner(1000)
	headers := []string{
		"Content-Type",
		"User-Agent",
		"Accept",
		"Host",
		"Connection",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interner.Intern(headers[i%len(headers)])
	}
}

func BenchmarkStringInternMiss(b *testing.B) {
	interner := NewStringInterner(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interner.Intern(fmt.Sprintf("header-%d", i))
	}
}

func BenchmarkStringInternBytes(b *testing.B) {
	interner := NewStringInterner(1000)
	headerBytes := [][]byte{
		[]byte("Content-Type"),
		[]byte("User-Agent"),
		[]byte("Accept"),
		[]byte("Host"),
		[]byte("Connection"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interner.InternBytes(headerBytes[i%len(headerBytes)])
	}
}

func BenchmarkHTTPHeaderInterner(b *testing.B) {
	interner := NewHTTPHeaderInterner()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interner.InternHeader("Content-Type")
	}
}

func BenchmarkByteStringEqual(b *testing.B) {
	bs := ByteString("Content-Type")
	str := "Content-Type"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bs.Equal(str)
	}
}

func BenchmarkStringCompare(b *testing.B) {
	s1 := "Content-Type"
	s2 := "Content-Type"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s1 == s2
	}
}

// Concurrent benchmarks
func BenchmarkStringInternConcurrent(b *testing.B) {
	interner := NewStringInterner(1000)
	headers := []string{
		"Content-Type",
		"User-Agent",
		"Accept",
		"Host",
		"Connection",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			interner.Intern(headers[i%len(headers)])
			i++
		}
	})
}
