package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"
)

var (
	targetURL     = flag.String("url", "http://localhost:8080", "Target URL")
	requests      = flag.Int("requests", 10000, "Number of requests")
	concurrency   = flag.Int("concurrency", 10, "Concurrent requests")
	mode          = flag.String("mode", "http", "Mode: http or tcp")
	tcpAddr       = flag.String("tcp", "localhost:8080", "TCP address for tcp mode")
)

type LatencyStats struct {
	Latencies []time.Duration
	Errors    int
}

func main() {
	flag.Parse()

	fmt.Printf("Latency Benchmark\n")
	fmt.Printf("=================\n")
	fmt.Printf("Target: %s\n", getTarget())
	fmt.Printf("Requests: %d\n", *requests)
	fmt.Printf("Concurrency: %d\n\n", *concurrency)

	var stats *LatencyStats
	if *mode == "http" {
		stats = runHTTPBenchmark()
	} else {
		stats = runTCPBenchmark()
	}

	printStats(stats)
}

func getTarget() string {
	if *mode == "http" {
		return *targetURL
	}
	return *tcpAddr
}

func runHTTPBenchmark() *LatencyStats {
	stats := &LatencyStats{
		Latencies: make([]time.Duration, 0, *requests),
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        *concurrency,
			MaxIdleConnsPerHost: *concurrency,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	requestsPerWorker := *requests / *concurrency
	start := time.Now()

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < requestsPerWorker; j++ {
				reqStart := time.Now()

				resp, err := client.Get(*targetURL)
				if err != nil {
					mu.Lock()
					stats.Errors++
					mu.Unlock()
					continue
				}

				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()

				latency := time.Since(reqStart)

				mu.Lock()
				stats.Latencies = append(stats.Latencies, latency)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	fmt.Printf("Completed in: %s\n\n", elapsed)
	return stats
}

func runTCPBenchmark() *LatencyStats {
	stats := &LatencyStats{
		Latencies: make([]time.Duration, 0, *requests),
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	requestsPerWorker := *requests / *concurrency
	start := time.Now()

	message := []byte("PING\n")
	response := make([]byte, 1024)

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			conn, err := net.Dial("tcp", *tcpAddr)
			if err != nil {
				mu.Lock()
				stats.Errors += requestsPerWorker
				mu.Unlock()
				return
			}
			defer conn.Close()

			for j := 0; j < requestsPerWorker; j++ {
				reqStart := time.Now()

				_, err := conn.Write(message)
				if err != nil {
					mu.Lock()
					stats.Errors++
					mu.Unlock()
					continue
				}

				_, err = conn.Read(response)
				if err != nil {
					mu.Lock()
					stats.Errors++
					mu.Unlock()
					continue
				}

				latency := time.Since(reqStart)

				mu.Lock()
				stats.Latencies = append(stats.Latencies, latency)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	fmt.Printf("Completed in: %s\n\n", elapsed)
	return stats
}

func printStats(stats *LatencyStats) {
	if len(stats.Latencies) == 0 {
		fmt.Printf("No successful requests\n")
		fmt.Printf("Errors: %d\n", stats.Errors)
		return
	}

	// Sort latencies
	sort.Slice(stats.Latencies, func(i, j int) bool {
		return stats.Latencies[i] < stats.Latencies[j]
	})

	// Calculate stats
	total := time.Duration(0)
	for _, lat := range stats.Latencies {
		total += lat
	}

	count := len(stats.Latencies)
	avg := total / time.Duration(count)
	min := stats.Latencies[0]
	max := stats.Latencies[count-1]

	p50 := stats.Latencies[count*50/100]
	p90 := stats.Latencies[count*90/100]
	p95 := stats.Latencies[count*95/100]
	p99 := stats.Latencies[count*99/100]
	p999 := stats.Latencies[count*999/1000]

	fmt.Printf("Latency Statistics\n")
	fmt.Printf("==================\n")
	fmt.Printf("Total Requests:  %d\n", count)
	fmt.Printf("Errors:          %d (%.2f%%)\n", stats.Errors,
		float64(stats.Errors)/float64(count+stats.Errors)*100)
	fmt.Printf("\n")
	fmt.Printf("Min:             %s\n", min)
	fmt.Printf("Max:             %s\n", max)
	fmt.Printf("Mean:            %s\n", avg)
	fmt.Printf("\n")
	fmt.Printf("Percentiles:\n")
	fmt.Printf("  p50:           %s\n", p50)
	fmt.Printf("  p90:           %s\n", p90)
	fmt.Printf("  p95:           %s\n", p95)
	fmt.Printf("  p99:           %s\n", p99)
	fmt.Printf("  p99.9:         %s\n", p999)

	// Check against targets
	fmt.Printf("\n")
	fmt.Printf("Performance Targets\n")
	fmt.Printf("===================\n")

	target := 10 * time.Millisecond
	if p99 <= target {
		fmt.Printf("✓ p99 latency: %s (target: <%s)\n", p99, target)
	} else {
		fmt.Printf("✗ p99 latency: %s (target: <%s) - MISSED\n", p99, target)
	}

	// Distribution
	fmt.Printf("\n")
	fmt.Printf("Latency Distribution\n")
	fmt.Printf("====================\n")

	buckets := []struct {
		name  string
		limit time.Duration
	}{
		{"<1ms", 1 * time.Millisecond},
		{"<5ms", 5 * time.Millisecond},
		{"<10ms", 10 * time.Millisecond},
		{"<50ms", 50 * time.Millisecond},
		{"<100ms", 100 * time.Millisecond},
		{"<500ms", 500 * time.Millisecond},
		{"<1s", 1 * time.Second},
		{">1s", time.Hour},
	}

	for _, bucket := range buckets {
		cnt := 0
		for _, lat := range stats.Latencies {
			if lat <= bucket.limit {
				cnt++
			}
		}
		pct := float64(cnt) / float64(count) * 100
		fmt.Printf("  %-8s %6d (%.2f%%)\n", bucket.name, cnt, pct)
	}
}
