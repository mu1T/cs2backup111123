//go:build windows

package video

import (
	"os"
	"syscall"
)

// IsReadOnly reports whether the file at path has the read-only attribute set.
func IsReadOnly(path string) (bool, error) {
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}
	attrs, err := syscall.GetFileAttributes(ptr)
	if err != nil {
		return false, err
	}
	return (attrs & syscall.FILE_ATTRIBUTE_READONLY) != 0, nil
}

// SetReadOnly adds or removes the read-only attribute on Windows.
func SetReadOnly(path string, readOnly bool) error {
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attrs, err := syscall.GetFileAttributes(ptr)
	if err != nil {
		return err
	}
	if readOnly {
		attrs |= syscall.FILE_ATTRIBUTE_READONLY
		_ = os.Chmod(path, 0444)
	} else {
		attrs &^= syscall.FILE_ATTRIBUTE_READONLY
		_ = os.Chmod(path, 0666)
	}
	return syscall.SetFileAttributes(ptr, attrs)
}
