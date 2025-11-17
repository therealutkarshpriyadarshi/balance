//go:build linux
// +build linux

package proxy

import (
	"errors"
	"syscall"
)

const (
	// SPLICE_F_MOVE hints to move pages instead of copying
	SPLICE_F_MOVE = 0x1
	// SPLICE_F_NONBLOCK makes splice non-blocking
	SPLICE_F_NONBLOCK = 0x2
	// SPLICE_F_MORE hints that more data will be sent
	SPLICE_F_MORE = 0x4
)

// spliceCopy performs zero-copy data transfer using splice system call
func spliceCopy(dstFd, srcFd int, bufferSize int) (written int64, err error) {
	// Create a pipe for splice
	var pipeFds [2]int
	if err := syscall.Pipe(pipeFds[:]); err != nil {
		return 0, err
	}
	defer syscall.Close(pipeFds[0])
	defer syscall.Close(pipeFds[1])

	var totalWritten int64

	for {
		// Splice from source to pipe
		n, err := splice(srcFd, nil, pipeFds[1], nil, bufferSize, SPLICE_F_MOVE|SPLICE_F_MORE)
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EINTR {
				continue
			}
			if totalWritten > 0 {
				return totalWritten, nil
			}
			return 0, err
		}

		if n == 0 {
			// EOF
			break
		}

		// Splice from pipe to destination
		written := int64(0)
		for written < int64(n) {
			w, err := splice(pipeFds[0], nil, dstFd, nil, int(int64(n)-written), SPLICE_F_MOVE|SPLICE_F_MORE)
			if err != nil {
				if err == syscall.EAGAIN || err == syscall.EINTR {
					continue
				}
				return totalWritten, err
			}
			written += int64(w)
		}

		totalWritten += written
	}

	return totalWritten, nil
}

// splice is a wrapper around the splice system call
func splice(fdIn int, offIn *int64, fdOut int, offOut *int64, len int, flags int) (n int, err error) {
	var roffIn *int64
	var roffOut *int64

	if offIn != nil {
		roffIn = offIn
	}
	if offOut != nil {
		roffOut = offOut
	}

	r0, _, e1 := syscall.Syscall6(
		syscall.SYS_SPLICE,
		uintptr(fdIn),
		uintptr(ptrOrZero(roffIn)),
		uintptr(fdOut),
		uintptr(ptrOrZero(roffOut)),
		uintptr(len),
		uintptr(flags),
	)

	n = int(r0)
	if e1 != 0 {
		err = e1
	}
	return
}

// ptrOrZero returns the pointer value or 0 if nil
func ptrOrZero(p *int64) uintptr {
	if p == nil {
		return 0
	}
	return uintptr(*p)
}

// SendFile performs zero-copy file sending using sendfile system call
func SendFile(dstFd, srcFd int, count int) (written int64, err error) {
	var offset int64 = 0
	n, err := syscall.Sendfile(dstFd, srcFd, &offset, count)
	return int64(n), err
}

// SpliceSupported returns true if splice is supported on this platform
func SpliceSupported() bool {
	return true
}

// SetupSpliceFlags sets appropriate flags for splice operations
func SetupSpliceFlags(nonBlocking bool) int {
	flags := SPLICE_F_MOVE | SPLICE_F_MORE

	if nonBlocking {
		flags |= SPLICE_F_NONBLOCK
	}

	return flags
}

// TrySplice attempts to use splice for zero-copy transfer
func TrySplice(dstFd, srcFd int, count int, nonBlocking bool) (int, error) {
	// Create a pipe for splice
	var pipeFds [2]int
	if err := syscall.Pipe(pipeFds[:]); err != nil {
		return 0, err
	}
	defer syscall.Close(pipeFds[0])
	defer syscall.Close(pipeFds[1])

	flags := SetupSpliceFlags(nonBlocking)

	// Splice from source to pipe
	n, err := splice(srcFd, nil, pipeFds[1], nil, count, flags)
	if err != nil {
		return 0, err
	}

	if n == 0 {
		return 0, errors.New("splice: unexpected EOF")
	}

	// Splice from pipe to destination
	written := 0
	for written < n {
		w, err := splice(pipeFds[0], nil, dstFd, nil, n-written, flags)
		if err != nil {
			return written, err
		}
		written += w
	}

	return written, nil
}
