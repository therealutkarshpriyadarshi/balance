package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	proxyAddr   = flag.String("proxy", "localhost:8080", "Proxy address")
	backendAddr = flag.String("backend", "localhost:9001", "Backend address")
	connections = flag.Int("connections", 100, "Number of concurrent connections")
	duration    = flag.Duration("duration", 30*time.Second, "Test duration")
	messageSize = flag.Int("size", 1024, "Message size in bytes")
)

func main() {
	flag.Parse()

	fmt.Printf("TCP Benchmark\n")
	fmt.Printf("=============\n")
	fmt.Printf("Proxy: %s\n", *proxyAddr)
	fmt.Printf("Connections: %d\n", *connections)
	fmt.Printf("Duration: %s\n", *duration)
	fmt.Printf("Message Size: %d bytes\n\n", *messageSize)

	// Start backend server
	go startBackend()
	time.Sleep(100 * time.Millisecond)

	// Run benchmark
	results := runBenchmark()

	// Print results
	printResults(results)
}

func startBackend() {
	listener, err := net.Listen("tcp", *backendAddr)
	if err != nil {
		fmt.Printf("Failed to start backend: %v\n", err)
		return
	}
	defer listener.Close()

	fmt.Printf("Backend listening on %s\n", *backendAddr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleBackend(conn)
	}
}

func handleBackend(conn net.Conn) {
	defer conn.Close()

	// Echo server
	buf := make([]byte, 32*1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				fmt.Printf("Backend read error: %v\n", err)
			}
			return
		}

		_, err = conn.Write(buf[:n])
		if err != nil {
			fmt.Printf("Backend write error: %v\n", err)
			return
		}
	}
}

type BenchmarkResults struct {
	TotalRequests    uint64
	TotalBytes       uint64
	TotalErrors      uint64
	Duration         time.Duration
	RequestsPerSec   float64
	BytesPerSec      float64
	MegabitsPerSec   float64
	AvgLatencyMs     float64
	ErrorRate        float64
}

func runBenchmark() *BenchmarkResults {
	var (
		totalRequests uint64
		totalBytes    uint64
		totalErrors   uint64
		totalLatency  uint64
	)

	var wg sync.WaitGroup
	start := time.Now()
	stop := make(chan struct{})

	// Start test clients
	for i := 0; i < *connections; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			conn, err := net.Dial("tcp", *proxyAddr)
			if err != nil {
				atomic.AddUint64(&totalErrors, 1)
				return
			}
			defer conn.Close()

			message := make([]byte, *messageSize)
			response := make([]byte, *messageSize)

			for {
				select {
				case <-stop:
					return
				default:
				}

				reqStart := time.Now()

				// Send message
				_, err := conn.Write(message)
				if err != nil {
					atomic.AddUint64(&totalErrors, 1)
					continue
				}

				// Read response
				_, err = io.ReadFull(conn, response)
				if err != nil {
					atomic.AddUint64(&totalErrors, 1)
					continue
				}

				latency := time.Since(reqStart)
				atomic.AddUint64(&totalRequests, 1)
				atomic.AddUint64(&totalBytes, uint64(*messageSize*2))
				atomic.AddUint64(&totalLatency, uint64(latency.Microseconds()))
			}
		}()
	}

	// Wait for duration
	time.Sleep(*duration)
	close(stop)
	wg.Wait()

	elapsed := time.Since(start)

	// Calculate results
	requests := atomic.LoadUint64(&totalRequests)
	bytes := atomic.LoadUint64(&totalBytes)
	errors := atomic.LoadUint64(&totalErrors)
	latency := atomic.LoadUint64(&totalLatency)

	results := &BenchmarkResults{
		TotalRequests:  requests,
		TotalBytes:     bytes,
		TotalErrors:    errors,
		Duration:       elapsed,
		RequestsPerSec: float64(requests) / elapsed.Seconds(),
		BytesPerSec:    float64(bytes) / elapsed.Seconds(),
		MegabitsPerSec: (float64(bytes) * 8) / (elapsed.Seconds() * 1024 * 1024),
		ErrorRate:      float64(errors) / float64(requests+errors) * 100,
	}

	if requests > 0 {
		results.AvgLatencyMs = float64(latency) / float64(requests) / 1000.0
	}

	return results
}

func printResults(r *BenchmarkResults) {
	fmt.Printf("\nResults\n")
	fmt.Printf("=======\n")
	fmt.Printf("Total Requests:    %d\n", r.TotalRequests)
	fmt.Printf("Total Bytes:       %d (%.2f MB)\n", r.TotalBytes, float64(r.TotalBytes)/(1024*1024))
	fmt.Printf("Total Errors:      %d\n", r.TotalErrors)
	fmt.Printf("Duration:          %s\n", r.Duration)
	fmt.Printf("\nPerformance\n")
	fmt.Printf("-----------\n")
	fmt.Printf("Requests/sec:      %.2f\n", r.RequestsPerSec)
	fmt.Printf("Throughput:        %.2f MB/s (%.2f Mbps)\n",
		r.BytesPerSec/(1024*1024), r.MegabitsPerSec)
	fmt.Printf("Avg Latency:       %.2f ms\n", r.AvgLatencyMs)
	fmt.Printf("Error Rate:        %.2f%%\n", r.ErrorRate)
}
