//go:build windows

package lan

import (
	"syscall"
)

// controlReusePort configures SO_REUSEADDR on the socket on Windows.
func controlReusePort(network, address string, c syscall.RawConn) error {
	var controlErr error
	err := c.Control(func(fd uintptr) {
		controlErr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	})
	if err != nil {
		return err
	}
	return controlErr
}

// controlBroadcast configures SO_BROADCAST on the socket on Windows.
func controlBroadcast(network, address string, c syscall.RawConn) error {
	var controlErr error
	err := c.Control(func(fd uintptr) {
		controlErr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
	})
	if err != nil {
		return err
	}
	return controlErr
}
