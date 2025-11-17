package xds

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
)

// XDSServer implements a simplified xDS control plane
// This is a basic implementation for dynamic configuration
type XDSServer struct {
	addr          string
	grpcServer    *grpc.Server
	snapshotCache *SnapshotCache
	mu            sync.RWMutex
	started       bool
}

// ServerConfig contains xDS server configuration
type ServerConfig struct {
	ListenAddr string
	TLSConfig  *TLSConfig
}

// TLSConfig contains TLS configuration for xDS server
type TLSConfig struct {
	CertFile string
	KeyFile  string
}

// NewXDSServer creates a new xDS server
func NewXDSServer(config ServerConfig) *XDSServer {
	return &XDSServer{
		addr:          config.ListenAddr,
		snapshotCache: NewSnapshotCache(),
	}
}

// Start starts the xDS server
func (s *XDSServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("xDS server already started")
	}

	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}

	s.grpcServer = grpc.NewServer()

	// Register xDS services would go here
	// In a full implementation, we'd register CDS, EDS, LDS, RDS services

	go func() {
		if err := s.grpcServer.Serve(listener); err != nil {
			fmt.Printf("xDS server error: %v\n", err)
		}
	}()

	s.started = true
	fmt.Printf("xDS server started on %s\n", s.addr)
	return nil
}

// Stop stops the xDS server
func (s *XDSServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return fmt.Errorf("xDS server not started")
	}

	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	s.started = false
	fmt.Printf("xDS server stopped\n")
	return nil
}

// UpdateSnapshot updates the configuration snapshot
func (s *XDSServer) UpdateSnapshot(nodeID string, snapshot *Snapshot) error {
	return s.snapshotCache.SetSnapshot(nodeID, snapshot)
}

// GetSnapshot gets the configuration snapshot for a node
func (s *XDSServer) GetSnapshot(nodeID string) (*Snapshot, error) {
	return s.snapshotCache.GetSnapshot(nodeID)
}

// Snapshot represents a configuration snapshot
type Snapshot struct {
	Version   string
	Clusters  []Cluster
	Endpoints []Endpoint
	Listeners []Listener
	Routes    []Route
	Timestamp time.Time
}

// Cluster represents a backend cluster
type Cluster struct {
	Name     string
	Type     string // STATIC, STRICT_DNS, LOGICAL_DNS, EDS
	Backends []string
	LBPolicy string
}

// Endpoint represents a backend endpoint
type Endpoint struct {
	ClusterName string
	Address     string
	Port        int
	Weight      int
	Healthy     bool
}

// Listener represents a listener configuration
type Listener struct {
	Name    string
	Address string
	Port    int
	Routes  []string
}

// Route represents a routing rule
type Route struct {
	Name    string
	Match   RouteMatch
	Cluster string
}

// RouteMatch represents route matching criteria
type RouteMatch struct {
	Prefix      string
	Path        string
	Headers     map[string]string
	QueryParams map[string]string
}

// SnapshotCache caches configuration snapshots
type SnapshotCache struct {
	mu        sync.RWMutex
	snapshots map[string]*Snapshot
	versions  map[string]string
}

// NewSnapshotCache creates a new snapshot cache
func NewSnapshotCache() *SnapshotCache {
	return &SnapshotCache{
		snapshots: make(map[string]*Snapshot),
		versions:  make(map[string]string),
	}
}

// SetSnapshot sets a snapshot for a node
func (sc *SnapshotCache) SetSnapshot(nodeID string, snapshot *Snapshot) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if snapshot.Version == "" {
		snapshot.Version = fmt.Sprintf("%d", time.Now().Unix())
	}

	snapshot.Timestamp = time.Now()
	sc.snapshots[nodeID] = snapshot
	sc.versions[nodeID] = snapshot.Version

	return nil
}

// GetSnapshot gets a snapshot for a node
func (sc *SnapshotCache) GetSnapshot(nodeID string) (*Snapshot, error) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	snapshot, ok := sc.snapshots[nodeID]
	if !ok {
		return nil, fmt.Errorf("snapshot not found for node %s", nodeID)
	}

	return snapshot, nil
}

// GetVersion gets the version for a node
func (sc *SnapshotCache) GetVersion(nodeID string) (string, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	version, ok := sc.versions[nodeID]
	return version, ok
}

// ClearSnapshot clears the snapshot for a node
func (sc *SnapshotCache) ClearSnapshot(nodeID string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	delete(sc.snapshots, nodeID)
	delete(sc.versions, nodeID)
}

// DynamicConfigManager manages dynamic configuration updates
type DynamicConfigManager struct {
	xdsServer *XDSServer
	mu        sync.RWMutex
	callbacks []ConfigUpdateCallback
}

// ConfigUpdateCallback is called when configuration is updated
type ConfigUpdateCallback func(snapshot *Snapshot) error

// NewDynamicConfigManager creates a new dynamic config manager
func NewDynamicConfigManager(xdsServer *XDSServer) *DynamicConfigManager {
	return &DynamicConfigManager{
		xdsServer: xdsServer,
		callbacks: make([]ConfigUpdateCallback, 0),
	}
}

// RegisterCallback registers a callback for config updates
func (dcm *DynamicConfigManager) RegisterCallback(callback ConfigUpdateCallback) {
	dcm.mu.Lock()
	defer dcm.mu.Unlock()

	dcm.callbacks = append(dcm.callbacks, callback)
}

// UpdateConfig updates the configuration
func (dcm *DynamicConfigManager) UpdateConfig(nodeID string, snapshot *Snapshot) error {
	if err := dcm.xdsServer.UpdateSnapshot(nodeID, snapshot); err != nil {
		return err
	}

	// Notify callbacks
	dcm.mu.RLock()
	callbacks := dcm.callbacks
	dcm.mu.RUnlock()

	for _, callback := range callbacks {
		if err := callback(snapshot); err != nil {
			fmt.Printf("Config update callback error: %v\n", err)
		}
	}

	return nil
}

// Watch watches for configuration changes
func (dcm *DynamicConfigManager) Watch(ctx context.Context, nodeID string, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	lastVersion := ""

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			version, ok := dcm.xdsServer.snapshotCache.GetVersion(nodeID)
			if !ok {
				continue
			}

			if version != lastVersion {
				snapshot, err := dcm.xdsServer.GetSnapshot(nodeID)
				if err != nil {
					fmt.Printf("Failed to get snapshot: %v\n", err)
					continue
				}

				dcm.mu.RLock()
				callbacks := dcm.callbacks
				dcm.mu.RUnlock()

				for _, callback := range callbacks {
					if err := callback(snapshot); err != nil {
						fmt.Printf("Config update callback error: %v\n", err)
					}
				}

				lastVersion = version
			}
		}
	}
}

// Helper functions for creating snapshots

// NewSnapshot creates a new snapshot
func NewSnapshot(version string) *Snapshot {
	return &Snapshot{
		Version:   version,
		Clusters:  make([]Cluster, 0),
		Endpoints: make([]Endpoint, 0),
		Listeners: make([]Listener, 0),
		Routes:    make([]Route, 0),
		Timestamp: time.Now(),
	}
}

// ToJSON converts snapshot to JSON
func (s *Snapshot) ToJSON() (string, error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON creates snapshot from JSON
func FromJSON(data string) (*Snapshot, error) {
	var snapshot Snapshot
	if err := json.Unmarshal([]byte(data), &snapshot); err != nil {
		return nil, err
	}
	return &snapshot, nil
}
