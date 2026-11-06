package steam

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// ScanUserdataAccountIDs scans the given Steam directory's userdata/ folder,
// finds all purely numeric directory entries (excluding 0) and returns them sorted numerically ascending.
func ScanUserdataAccountIDs(steamPath string) ([]uint32, error) {
	if steamPath == "" {
		return nil, fmt.Errorf("empty steam path")
	}

	userdataDir := filepath.Join(steamPath, "userdata")
	entries, err := os.ReadDir(userdataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read userdata directory: %w", err)
	}

	var accountIDs []uint32
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Parse purely numeric IDs
		aid, err := strconv.ParseUint(name, 10, 32)
		if err != nil || aid == 0 {
			continue
		}
		accountIDs = append(accountIDs, uint32(aid))
	}

	sort.Slice(accountIDs, func(i, j int) bool {
		return accountIDs[i] < accountIDs[j]
	})

	return accountIDs, nil
}
