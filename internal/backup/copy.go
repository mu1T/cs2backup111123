package backup

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

const copyBufferSize = 64 * 1024 // 64KB per ADR 0002

var copyBufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, copyBufferSize)
		return &buf
	},
}

// CopyFile streams data from src to dst using a pooled 64KB buffer.
func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file %s: %w", src, err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create destination dir for %s: %w", dst, err)
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer dstFile.Close()

	bufPtr := copyBufPool.Get().(*[]byte)
	defer copyBufPool.Put(bufPtr)

	if _, err := io.CopyBuffer(dstFile, srcFile, *bufPtr); err != nil {
		return fmt.Errorf("failed to copy buffer from %s to %s: %w", src, dst, err)
	}

	return dstFile.Sync()
}

// CopyDir recursively copies the directory tree from src to dst using pooled streaming buffers.
func CopyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("source directory %s stat error: %w", src, err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source %s is not a directory", src)
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("failed to create destination root dir %s: %w", dst, err)
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		targetPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		return CopyFile(path, targetPath)
	})
}
