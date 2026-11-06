//go:build !windows

package video

import "os"

// IsReadOnly reports whether the file at path is read-only.
func IsReadOnly(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return (fi.Mode() & 0200) == 0, nil
}

// SetReadOnly changes the file mode to read-only or writable.
func SetReadOnly(path string, readOnly bool) error {
	if readOnly {
		return os.Chmod(path, 0444)
	}
	return os.Chmod(path, 0666)
}
