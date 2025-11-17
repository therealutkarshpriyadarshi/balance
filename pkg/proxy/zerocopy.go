package proxy

import (
	"io"
	"net"
	"syscall"
)

// ZeroCopier provides zero-copy data transfer capabilities
type ZeroCopier interface {
	// Copy copies data from src to dst using zero-copy techniques when possible
	Copy(dst, src net.Conn) (written int64, err error)
}

// DefaultZeroCopier is the default zero-copy implementation
type DefaultZeroCopier struct {
	bufferSize int
}

// NewDefaultZeroCopier creates a new default zero-copier
func NewDefaultZeroCopier(bufferSize int) *DefaultZeroCopier {
	if bufferSize <= 0 {
		bufferSize = 32 * 1024
	}
	return &DefaultZeroCopier{
		bufferSize: bufferSize,
	}
}

// Copy implements zero-copy transfer when possible
func (z *DefaultZeroCopier) Copy(dst, src net.Conn) (written int64, err error) {
	// Try to use splice/sendfile for TCP connections on Linux
	if tcpSrc, ok := src.(*net.TCPConn); ok {
		if tcpDst, ok := dst.(*net.TCPConn); ok {
			return z.tcpSplice(tcpDst, tcpSrc)
		}
	}

	// Fallback to regular io.Copy
	return io.Copy(dst, src)
}

// tcpSplice attempts to use splice for zero-copy TCP transfer
func (z *DefaultZeroCopier) tcpSplice(dst, src *net.TCPConn) (written int64, err error) {
	// Get raw file descriptors
	srcFile, err := src.File()
	if err != nil {
		// Fallback to regular copy
		return io.Copy(dst, src)
	}
	defer srcFile.Close()

	dstFile, err := dst.File()
	if err != nil {
		// Fallback to regular copy
		return io.Copy(dst, src)
	}
	defer dstFile.Close()

	srcFd := int(srcFile.Fd())
	dstFd := int(dstFile.Fd())

	// Try splice on Linux
	written, err = spliceCopy(dstFd, srcFd, z.bufferSize)
	if err != nil {
		// If splice fails, fallback to regular copy
		return io.Copy(dst, src)
	}

	return written, nil
}

// BidirectionalCopy copies data bidirectionally between two connections
func BidirectionalCopy(conn1, conn2 net.Conn, copier ZeroCopier) error {
	errChan := make(chan error, 2)

	copy := func(dst, src net.Conn) {
		_, err := copier.Copy(dst, src)
		errChan <- err
	}

	go copy(conn1, conn2)
	go copy(conn2, conn1)

	// Wait for one direction to complete
	err := <-errChan

	// Close both connections to stop the other direction
	conn1.Close()
	conn2.Close()

	// Wait for the other direction
	<-errChan

	return err
}

// OptimizedBidirectionalCopy is an optimized version using connection pools
func OptimizedBidirectionalCopy(conn1, conn2 net.Conn, copier ZeroCopier, bufferPool interface {
	Get(size int) []byte
	Put(buf []byte)
}) error {
	errChan := make(chan error, 2)

	copy := func(dst, src net.Conn) {
		var err error
		var written int64

		// Try zero-copy first
		if copier != nil {
			written, err = copier.Copy(dst, src)
		} else {
			// Use buffered copy with pool
			buf := bufferPool.Get(32 * 1024)
			defer bufferPool.Put(buf)
			written, err = io.CopyBuffer(dst, src, buf)
		}

		_ = written // Avoid unused variable
		errChan <- err
	}

	go copy(conn1, conn2)
	go copy(conn2, conn1)

	// Wait for one direction to complete
	err := <-errChan

	// Close both connections to stop the other direction
	conn1.Close()
	conn2.Close()

	// Wait for the other direction
	<-errChan

	return err
}

// ReadWriteOptimizer provides optimized read/write operations
type ReadWriteOptimizer struct {
	readBuffer  []byte
	writeBuffer []byte
}

// NewReadWriteOptimizer creates a new read/write optimizer
func NewReadWriteOptimizer(bufferSize int) *ReadWriteOptimizer {
	if bufferSize <= 0 {
		bufferSize = 32 * 1024
	}
	return &ReadWriteOptimizer{
		readBuffer:  make([]byte, bufferSize),
		writeBuffer: make([]byte, bufferSize),
	}
}

// CopyWithBuffers copies data using pre-allocated buffers
func (rw *ReadWriteOptimizer) CopyWithBuffers(dst io.Writer, src io.Reader) (written int64, err error) {
	return io.CopyBuffer(dst, src, rw.readBuffer)
}

// SetTCPOptimizations sets optimal TCP options for proxying
func SetTCPOptimizations(conn *net.TCPConn) error {
	// Enable TCP_NODELAY to disable Nagle's algorithm
	if err := conn.SetNoDelay(true); err != nil {
		return err
	}

	// Set keep-alive
	if err := conn.SetKeepAlive(true); err != nil {
		return err
	}

	// Get the raw connection
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return err
	}

	var setErr error
	err = rawConn.Control(func(fd uintptr) {
		// Set TCP_QUICKACK on Linux to send ACKs immediately
		setErr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1)
		if setErr != nil {
			return
		}

		// Try to set TCP_CORK on Linux for better batching on writes
		// This is optional and may fail on non-Linux systems
		_ = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_CORK, 0)
	})

	if err != nil {
		return err
	}

	return setErr
}

// GetOptimalBufferSize returns the optimal buffer size for the connection
func GetOptimalBufferSize(conn net.Conn) int {
	// Try to get SO_RCVBUF size
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		rawConn, err := tcpConn.SyscallConn()
		if err == nil {
			var size int
			rawConn.Control(func(fd uintptr) {
				size, _ = syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF)
			})
			if size > 0 {
				return size
			}
		}
	}

	// Default to 32KB
	return 32 * 1024
}
