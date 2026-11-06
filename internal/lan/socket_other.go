//go:build !windows

package lan

import (
	"syscall"
)

// controlReusePort configures SO_REUSEADDR (and SO_REUSEPORT if available) on the socket.
func controlReusePort(network, address string, c syscall.RawConn) error {
	var controlErr error
	err := c.Control(func(fd uintptr) {
		if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
			controlErr = err
			return
		}
		// Try SO_REUSEPORT (0x0f on Linux/BSD)
		const soReusePort = 0x0f
		_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, soReusePort, 1)
	})
	if err != nil {
		return err
	}
	return controlErr
}

// controlBroadcast configures SO_BROADCAST on the socket.
func controlBroadcast(network, address string, c syscall.RawConn) error {
	var controlErr error
	err := c.Control(func(fd uintptr) {
		controlErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
	})
	if err != nil {
		return err
	}
	return controlErr
}
