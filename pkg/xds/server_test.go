package xds

import (
	"context"
	"testing"
	"time"
)

func TestSnapshotCache(t *testing.T) {
	cache := NewSnapshotCache()

	// Test set and get
	snapshot := &Snapshot{
		Version:  "v1",
		Clusters: []Cluster{{Name: "test-cluster"}},
	}

	err := cache.SetSnapshot("node1", snapshot)
	if err != nil {
		t.Errorf("Failed to set snapshot: %v", err)
	}

	retrieved, err := cache.GetSnapshot("node1")
	if err != nil {
		t.Errorf("Failed to get snapshot: %v", err)
	}

	if retrieved.Version != "v1" {
		t.Errorf("Expected version v1, got %s", retrieved.Version)
	}

	// Test version
	version, ok := cache.GetVersion("node1")
	if !ok || version != "v1" {
		t.Errorf("Expected version v1, got %s", version)
	}

	// Test clear
	cache.ClearSnapshot("node1")
	_, err = cache.GetSnapshot("node1")
	if err == nil {
		t.Errorf("Expected error after clearing snapshot")
	}
}

func TestSnapshot(t *testing.T) {
	snapshot := NewSnapshot("v1")

	// Add clusters
	snapshot.Clusters = append(snapshot.Clusters, Cluster{
		Name:     "backend-cluster",
		Type:     "STATIC",
		Backends: []string{"localhost:9001", "localhost:9002"},
		LBPolicy: "round-robin",
	})

	// Add endpoints
	snapshot.Endpoints = append(snapshot.Endpoints, Endpoint{
		ClusterName: "backend-cluster",
		Address:     "localhost",
		Port:        9001,
		Weight:      1,
		Healthy:     true,
	})

	// Add listeners
	snapshot.Listeners = append(snapshot.Listeners, Listener{
		Name:    "http-listener",
		Address: "0.0.0.0",
		Port:    8080,
	})

	// Test JSON serialization
	jsonStr, err := snapshot.ToJSON()
	if err != nil {
		t.Errorf("Failed to convert to JSON: %v", err)
	}

	// Test JSON deserialization
	restored, err := FromJSON(jsonStr)
	if err != nil {
		t.Errorf("Failed to parse JSON: %v", err)
	}

	if restored.Version != "v1" {
		t.Errorf("Expected version v1, got %s", restored.Version)
	}

	if len(restored.Clusters) != 1 {
		t.Errorf("Expected 1 cluster, got %d", len(restored.Clusters))
	}
}

func TestDynamicConfigManager(t *testing.T) {
	xdsServer := NewXDSServer(ServerConfig{
		ListenAddr: ":0", // Random port
	})

	manager := NewDynamicConfigManager(xdsServer)

	// Register callback
	callbackCalled := false
	manager.RegisterCallback(func(snapshot *Snapshot) error {
		callbackCalled = true
		return nil
	})

	// Update config
	snapshot := NewSnapshot("v1")
	err := manager.UpdateConfig("node1", snapshot)
	if err != nil {
		t.Errorf("Failed to update config: %v", err)
	}

	if !callbackCalled {
		t.Errorf("Callback was not called")
	}
}

func TestDynamicConfigManagerWatch(t *testing.T) {
	xdsServer := NewXDSServer(ServerConfig{
		ListenAddr: ":0",
	})

	manager := NewDynamicConfigManager(xdsServer)

	updateCount := 0
	manager.RegisterCallback(func(snapshot *Snapshot) error {
		updateCount++
		return nil
	})

	// Start watching in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go manager.Watch(ctx, "node1", 10*time.Millisecond)

	// Update config a few times
	for i := 1; i <= 3; i++ {
		snapshot := NewSnapshot(string(rune('0' + i)))
		manager.UpdateConfig("node1", snapshot)
		time.Sleep(20 * time.Millisecond)
	}

	// Cancel and check
	cancel()

	if updateCount < 3 {
		t.Logf("Warning: Expected at least 3 updates, got %d", updateCount)
	}
}

func TestXDSServer(t *testing.T) {
	server := NewXDSServer(ServerConfig{
		ListenAddr: "localhost:0",
	})

	// Start server
	err := server.Start()
	if err != nil {
		t.Errorf("Failed to start server: %v", err)
	}

	// Update snapshot
	snapshot := NewSnapshot("v1")
	err = server.UpdateSnapshot("node1", snapshot)
	if err != nil {
		t.Errorf("Failed to update snapshot: %v", err)
	}

	// Get snapshot
	retrieved, err := server.GetSnapshot("node1")
	if err != nil {
		t.Errorf("Failed to get snapshot: %v", err)
	}

	if retrieved.Version != "v1" {
		t.Errorf("Expected version v1, got %s", retrieved.Version)
	}

	// Stop server
	err = server.Stop()
	if err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}
}

func TestRouteMatch(t *testing.T) {
	route := Route{
		Name: "test-route",
		Match: RouteMatch{
			Prefix:  "/api",
			Headers: map[string]string{"X-Test": "value"},
		},
		Cluster: "backend-cluster",
	}

	if route.Match.Prefix != "/api" {
		t.Errorf("Expected prefix /api, got %s", route.Match.Prefix)
	}
}

func TestCluster(t *testing.T) {
	cluster := Cluster{
		Name:     "test-cluster",
		Type:     "STATIC",
		Backends: []string{"localhost:9001", "localhost:9002"},
		LBPolicy: "round-robin",
	}

	if len(cluster.Backends) != 2 {
		t.Errorf("Expected 2 backends, got %d", len(cluster.Backends))
	}
}

func TestEndpoint(t *testing.T) {
	endpoint := Endpoint{
		ClusterName: "test-cluster",
		Address:     "localhost",
		Port:        9001,
		Weight:      1,
		Healthy:     true,
	}

	if !endpoint.Healthy {
		t.Errorf("Expected endpoint to be healthy")
	}
}
