package proxy

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

func TestDefaultZeroCopier(t *testing.T) {
	copier := NewDefaultZeroCopier(32 * 1024)

	// Create test connections
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	testData := []byte("Hello, World!")

	// Start copying in background
	go func() {
		copier.Copy(server, client)
	}()

	// Write data
	client.Write(testData)
	client.Close()

	// Read data
	buf := make([]byte, len(testData))
	n, err := io.ReadFull(server, buf)
	if err != nil && err != io.EOF {
		t.Errorf("Failed to read: %v", err)
	}

	if n != len(testData) {
		t.Errorf("Expected to read %d bytes, got %d", len(testData), n)
	}

	if !bytes.Equal(buf, testData) {
		t.Errorf("Data mismatch: expected %s, got %s", testData, buf)
	}
}

func TestBidirectionalCopy(t *testing.T) {
	copier := NewDefaultZeroCopier(32 * 1024)

	// Create two pairs of connections
	client1, server1 := net.Pipe()
	defer client1.Close()
	defer server1.Close()

	client2, server2 := net.Pipe()
	defer client2.Close()
	defer server2.Close()

	// Start bidirectional copy
	go BidirectionalCopy(server1, server2, copier)

	testData1 := []byte("Hello from client1")
	testData2 := []byte("Hello from client2")

	// Send from client1
	go func() {
		client1.Write(testData1)
		time.Sleep(10 * time.Millisecond)
		client1.Close()
	}()

	// Send from client2
	go func() {
		client2.Write(testData2)
		time.Sleep(10 * time.Millisecond)
		client2.Close()
	}()

	// Read at client2 (should receive from client1)
	buf1 := make([]byte, len(testData1))
	n1, _ := io.ReadFull(client2, buf1)
	if n1 == len(testData1) && bytes.Equal(buf1, testData1) {
		t.Logf("Successfully received data from client1 at client2")
	}

	// Read at client1 (should receive from client2)
	buf2 := make([]byte, len(testData2))
	n2, _ := io.ReadFull(client1, buf2)
	if n2 == len(testData2) && bytes.Equal(buf2, testData2) {
		t.Logf("Successfully received data from client2 at client1")
	}
}

func TestReadWriteOptimizer(t *testing.T) {
	optimizer := NewReadWriteOptimizer(32 * 1024)

	testData := make([]byte, 64*1024)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	src := bytes.NewReader(testData)
	dst := &bytes.Buffer{}

	written, err := optimizer.CopyWithBuffers(dst, src)
	if err != nil {
		t.Errorf("CopyWithBuffers failed: %v", err)
	}

	if written != int64(len(testData)) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(testData), written)
	}

	if !bytes.Equal(dst.Bytes(), testData) {
		t.Errorf("Data mismatch after copy")
	}
}

func TestSetTCPOptimizations(t *testing.T) {
	// Create a TCP listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Accept connection in background
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		time.Sleep(100 * time.Millisecond)
	}()

	// Connect
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		t.Fatalf("Not a TCP connection")
	}

	// Apply optimizations
	err = SetTCPOptimizations(tcpConn)
	if err != nil {
		t.Errorf("SetTCPOptimizations failed: %v", err)
	}
}

func TestGetOptimalBufferSize(t *testing.T) {
	// Create a TCP listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Accept connection in background
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		time.Sleep(100 * time.Millisecond)
	}()

	// Connect
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	size := GetOptimalBufferSize(conn)
	if size <= 0 {
		t.Errorf("Invalid buffer size: %d", size)
	}

	t.Logf("Optimal buffer size: %d", size)
}

func BenchmarkZeroCopy(b *testing.B) {
	copier := NewDefaultZeroCopier(32 * 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client, server := net.Pipe()

		go func() {
			data := make([]byte, 1024)
			client.Write(data)
			client.Close()
		}()

		copier.Copy(io.Discard, server)
		server.Close()
		client.Close()
	}
}

func BenchmarkRegularCopy(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client, server := net.Pipe()

		go func() {
			data := make([]byte, 1024)
			client.Write(data)
			client.Close()
		}()

		io.Copy(io.Discard, server)
		server.Close()
		client.Close()
	}
}

func BenchmarkReadWriteOptimizer(b *testing.B) {
	optimizer := NewReadWriteOptimizer(32 * 1024)
	data := make([]byte, 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src := bytes.NewReader(data)
		optimizer.CopyWithBuffers(io.Discard, src)
	}
}
