//go:build !linux
// +build !linux

package proxy

import (
	"errors"
)

var (
	// ErrSpliceNotSupported is returned when splice is not supported on the platform
	ErrSpliceNotSupported = errors.New("splice not supported on this platform")
)

// spliceCopy is not supported on non-Linux platforms
func spliceCopy(dstFd, srcFd int, bufferSize int) (written int64, err error) {
	return 0, ErrSpliceNotSupported
}

// SendFile is not supported on non-Linux platforms (or uses fallback)
func SendFile(dstFd, srcFd int, count int) (written int64, err error) {
	return 0, ErrSpliceNotSupported
}

// SpliceSupported returns false on non-Linux platforms
func SpliceSupported() bool {
	return false
}

// SetupSpliceFlags returns 0 on non-Linux platforms
func SetupSpliceFlags(nonBlocking bool) int {
	return 0
}

// TrySplice is not supported on non-Linux platforms
func TrySplice(dstFd, srcFd int, count int, nonBlocking bool) (int, error) {
	return 0, ErrSpliceNotSupported
}
