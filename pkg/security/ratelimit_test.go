package security

import (
	"testing"
	"time"
)

func TestTokenBucket(t *testing.T) {
	// Create a token bucket that allows 10 requests per second with burst of 20
	tb := NewTokenBucket(10.0, 20)

	// Should allow initial burst
	for i := 0; i < 20; i++ {
		if !tb.Allow("test-key") {
			t.Errorf("Expected request %d to be allowed", i)
		}
	}

	// Next request should be blocked (bucket exhausted)
	if tb.Allow("test-key") {
		t.Error("Expected request to be blocked after burst")
	}

	// Wait for tokens to refill
	time.Sleep(200 * time.Millisecond)

	// Should allow a couple more requests (2 tokens refilled in 200ms at 10/sec)
	if !tb.Allow("test-key") {
		t.Error("Expected request to be allowed after refill")
	}
}

func TestTokenBucketMultipleKeys(t *testing.T) {
	tb := NewTokenBucket(5.0, 10)

	// Different keys should have independent buckets
	for i := 0; i < 10; i++ {
		if !tb.Allow("key1") {
			t.Errorf("Expected request %d for key1 to be allowed", i)
		}
	}

	for i := 0; i < 10; i++ {
		if !tb.Allow("key2") {
			t.Errorf("Expected request %d for key2 to be allowed", i)
		}
	}

	// Both should be exhausted now
	if tb.Allow("key1") {
		t.Error("Expected key1 to be exhausted")
	}
	if tb.Allow("key2") {
		t.Error("Expected key2 to be exhausted")
	}
}

func TestTokenBucketReset(t *testing.T) {
	tb := NewTokenBucket(5.0, 10)

	// Exhaust bucket
	for i := 0; i < 10; i++ {
		tb.Allow("test-key")
	}

	// Should be blocked
	if tb.Allow("test-key") {
		t.Error("Expected request to be blocked")
	}

	// Reset
	tb.Reset("test-key")

	// Should be allowed again
	if !tb.Allow("test-key") {
		t.Error("Expected request to be allowed after reset")
	}
}

func TestTokenBucketStats(t *testing.T) {
	tb := NewTokenBucket(10.0, 20)

	tb.Allow("key1")
	tb.Allow("key1")
	tb.Allow("key2")

	stats := tb.Stats()

	if stats["total_requests"] != int64(3) {
		t.Errorf("Expected 3 total requests, got %v", stats["total_requests"])
	}

	if stats["allowed"] != int64(3) {
		t.Errorf("Expected 3 allowed requests, got %v", stats["allowed"])
	}
}

func TestSlidingWindow(t *testing.T) {
	// Allow 5 requests per 100ms window
	sw := NewSlidingWindow(5, 100*time.Millisecond)

	// Should allow 5 requests
	for i := 0; i < 5; i++ {
		if !sw.Allow("test-key") {
			t.Errorf("Expected request %d to be allowed", i)
		}
	}

	// 6th request should be blocked
	if sw.Allow("test-key") {
		t.Error("Expected 6th request to be blocked")
	}

	// Wait for window to slide
	time.Sleep(150 * time.Millisecond)

	// Should allow new requests
	if !sw.Allow("test-key") {
		t.Error("Expected request to be allowed after window slide")
	}
}

func TestSlidingWindowReset(t *testing.T) {
	sw := NewSlidingWindow(5, 100*time.Millisecond)

	// Fill window
	for i := 0; i < 5; i++ {
		sw.Allow("test-key")
	}

	// Should be blocked
	if sw.Allow("test-key") {
		t.Error("Expected request to be blocked")
	}

	// Reset
	sw.Reset("test-key")

	// Should be allowed again
	if !sw.Allow("test-key") {
		t.Error("Expected request to be allowed after reset")
	}
}

func TestPerIPRateLimiter(t *testing.T) {
	tb := NewTokenBucket(10.0, 20)
	limiter := NewPerIPRateLimiter(tb)

	// Should allow requests
	if !limiter.AllowIP("192.168.1.1") {
		t.Error("Expected request to be allowed")
	}

	// Different IP should have independent limit
	if !limiter.AllowIP("192.168.1.2") {
		t.Error("Expected request to be allowed for different IP")
	}
}

func TestCombinedRateLimiter(t *testing.T) {
	tb1 := NewTokenBucket(10.0, 20)
	tb2 := NewTokenBucket(5.0, 10)

	combined, err := NewCombinedRateLimiter(tb1, tb2)
	if err != nil {
		t.Fatalf("Failed to create combined rate limiter: %v", err)
	}

	// Allow requests until stricter limiter (tb2) blocks
	for i := 0; i < 10; i++ {
		combined.Allow("test-key")
	}

	// Next request should be blocked by tb2
	if combined.Allow("test-key") {
		t.Error("Expected request to be blocked by stricter limiter")
	}
}

func TestCombinedRateLimiterNoLimiters(t *testing.T) {
	_, err := NewCombinedRateLimiter()
	if err == nil {
		t.Error("Expected error when creating combined limiter with no limiters")
	}
}

func BenchmarkTokenBucket(b *testing.B) {
	tb := NewTokenBucket(1000000.0, 1000000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tb.Allow("test-key")
	}
}

func BenchmarkSlidingWindow(b *testing.B) {
	sw := NewSlidingWindow(1000000, 1*time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sw.Allow("test-key")
	}
}
