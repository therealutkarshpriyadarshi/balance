package profiling

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

// ProfileConfig contains profiling configuration
type ProfileConfig struct {
	CPUProfilePath    string
	MemProfilePath    string
	EnableHTTPProfile bool
	HTTPProfileAddr   string
	ProfileDuration   time.Duration
}

// DefaultProfileConfig returns default profiling configuration
func DefaultProfileConfig() ProfileConfig {
	return ProfileConfig{
		CPUProfilePath:    "",
		MemProfilePath:    "",
		EnableHTTPProfile: true,
		HTTPProfileAddr:   ":6060",
		ProfileDuration:   30 * time.Second,
	}
}

// Profiler manages profiling operations
type Profiler struct {
	config     ProfileConfig
	cpuFile    *os.File
	httpServer *http.Server
}

// NewProfiler creates a new profiler
func NewProfiler(config ProfileConfig) *Profiler {
	return &Profiler{
		config: config,
	}
}

// Start starts profiling
func (p *Profiler) Start() error {
	// Start CPU profiling if configured
	if p.config.CPUProfilePath != "" {
		if err := p.StartCPUProfile(); err != nil {
			return fmt.Errorf("failed to start CPU profile: %w", err)
		}
	}

	// Start HTTP profiling server if configured
	if p.config.EnableHTTPProfile {
		if err := p.StartHTTPProfile(); err != nil {
			return fmt.Errorf("failed to start HTTP profile: %w", err)
		}
	}

	return nil
}

// Stop stops profiling
func (p *Profiler) Stop() error {
	// Stop CPU profiling
	if p.cpuFile != nil {
		p.StopCPUProfile()
	}

	// Write memory profile if configured
	if p.config.MemProfilePath != "" {
		if err := p.WriteMemProfile(); err != nil {
			return fmt.Errorf("failed to write memory profile: %w", err)
		}
	}

	// Stop HTTP server
	if p.httpServer != nil {
		p.httpServer.Close()
	}

	return nil
}

// StartCPUProfile starts CPU profiling
func (p *Profiler) StartCPUProfile() error {
	f, err := os.Create(p.config.CPUProfilePath)
	if err != nil {
		return err
	}

	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		return err
	}

	p.cpuFile = f
	fmt.Printf("CPU profiling started: %s\n", p.config.CPUProfilePath)
	return nil
}

// StopCPUProfile stops CPU profiling
func (p *Profiler) StopCPUProfile() {
	pprof.StopCPUProfile()
	if p.cpuFile != nil {
		p.cpuFile.Close()
		p.cpuFile = nil
		fmt.Printf("CPU profiling stopped: %s\n", p.config.CPUProfilePath)
	}
}

// WriteMemProfile writes memory profile
func (p *Profiler) WriteMemProfile() error {
	f, err := os.Create(p.config.MemProfilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	runtime.GC() // Get up-to-date statistics
	if err := pprof.WriteHeapProfile(f); err != nil {
		return err
	}

	fmt.Printf("Memory profile written: %s\n", p.config.MemProfilePath)
	return nil
}

// StartHTTPProfile starts HTTP profiling server
func (p *Profiler) StartHTTPProfile() error {
	mux := http.NewServeMux()

	// pprof handlers are automatically registered in the default mux
	// We import net/http/pprof which registers handlers
	mux.Handle("/debug/pprof/", http.DefaultServeMux)

	p.httpServer = &http.Server{
		Addr:    p.config.HTTPProfileAddr,
		Handler: mux,
	}

	go func() {
		fmt.Printf("HTTP profiling server started: http://%s/debug/pprof/\n", p.config.HTTPProfileAddr)
		if err := p.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Printf("HTTP profiling server error: %v\n", err)
		}
	}()

	return nil
}

// DumpGoroutineProfile dumps goroutine profile
func (p *Profiler) DumpGoroutineProfile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return pprof.Lookup("goroutine").WriteTo(f, 0)
}

// DumpBlockProfile dumps block profile
func (p *Profiler) DumpBlockProfile(path string) error {
	runtime.SetBlockProfileRate(1)
	defer runtime.SetBlockProfileRate(0)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return pprof.Lookup("block").WriteTo(f, 0)
}

// DumpMutexProfile dumps mutex profile
func (p *Profiler) DumpMutexProfile(path string) error {
	runtime.SetMutexProfileFraction(1)
	defer runtime.SetMutexProfileFraction(0)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return pprof.Lookup("mutex").WriteTo(f, 0)
}

// PrintMemStats prints current memory statistics
func PrintMemStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Printf("\nMemory Statistics\n")
	fmt.Printf("=================\n")
	fmt.Printf("Alloc:        %v MB\n", m.Alloc/1024/1024)
	fmt.Printf("TotalAlloc:   %v MB\n", m.TotalAlloc/1024/1024)
	fmt.Printf("Sys:          %v MB\n", m.Sys/1024/1024)
	fmt.Printf("NumGC:        %v\n", m.NumGC)
	fmt.Printf("Goroutines:   %v\n", runtime.NumGoroutine())
}

// MonitorMemory monitors memory usage and prints stats periodically
func MonitorMemory(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			PrintMemStats()
		case <-stop:
			return
		}
	}
}

// RunProfiler runs profiling for a specified duration
func RunProfiler(config ProfileConfig) error {
	profiler := NewProfiler(config)

	if err := profiler.Start(); err != nil {
		return err
	}

	fmt.Printf("Profiling for %s...\n", config.ProfileDuration)
	time.Sleep(config.ProfileDuration)

	return profiler.Stop()
}

// Global profiler instance
var globalProfiler *Profiler

// StartGlobalProfiler starts the global profiler
func StartGlobalProfiler(config ProfileConfig) error {
	if globalProfiler != nil {
		return fmt.Errorf("profiler already started")
	}

	globalProfiler = NewProfiler(config)
	return globalProfiler.Start()
}

// StopGlobalProfiler stops the global profiler
func StopGlobalProfiler() error {
	if globalProfiler == nil {
		return fmt.Errorf("profiler not started")
	}

	err := globalProfiler.Stop()
	globalProfiler = nil
	return err
}
