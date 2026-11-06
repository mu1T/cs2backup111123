package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DiffType represents the status of a line in a diff.
type DiffType string

const (
	DiffSame DiffType = "same"
	DiffAdd  DiffType = "add"
	DiffDel  DiffType = "del"
	DiffMod  DiffType = "mod"
)

// DiffLine represents a single line in a diff comparison.
type DiffLine struct {
	Type       DiffType `json:"type"`
	OldLineNum int      `json:"old_line_num"`
	NewLineNum int      `json:"new_line_num"`
	Content    string   `json:"content"`
	OldContent string   `json:"old_content,omitempty"`
}

// DiffResult holds all diff lines along with change counters.
type DiffResult struct {
	Lines    []DiffLine `json:"lines"`
	AddCount int        `json:"add_count"`
	DelCount int        `json:"del_count"`
	ModCount int        `json:"mod_count"`
}

// FileDiffItem represents diff comparison for a specific file between snapshot and live config.
type FileDiffItem struct {
	RelPath      string      `json:"rel_path"`
	Status       string      `json:"status"` // "modified", "added", "deleted", "identical"
	SnapshotHash string      `json:"snapshot_hash,omitempty"`
	LiveHash     string      `json:"live_hash,omitempty"`
	Diff         *DiffResult `json:"diff,omitempty"`
}

// splitLines breaks a text into lines with CRLF normalization.
func splitLines(text string) []string {
	if text == "" {
		return []string{}
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	// If the file ends with a trailing newline, drop the trailing empty line
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

type rawDiffOp struct {
	opType     DiffType
	oldLineNum int
	newLineNum int
	content    string
}

// ComputeTextDiff computes line-by-line differences using Myers' SES algorithm with modification pairing.
func ComputeTextDiff(a, b []string) *DiffResult {
	n := len(a)
	m := len(b)

	if n == 0 && m == 0 {
		return &DiffResult{
			Lines: []DiffLine{},
		}
	}

	if n == 0 {
		lines := make([]DiffLine, 0, m)
		for j, line := range b {
			lines = append(lines, DiffLine{
				Type:       DiffAdd,
				OldLineNum: 0,
				NewLineNum: j + 1,
				Content:    line,
			})
		}
		return &DiffResult{
			Lines:    lines,
			AddCount: m,
		}
	}

	if m == 0 {
		lines := make([]DiffLine, 0, n)
		for i, line := range a {
			lines = append(lines, DiffLine{
				Type:       DiffDel,
				OldLineNum: i + 1,
				NewLineNum: 0,
				Content:    line,
			})
		}
		return &DiffResult{
			Lines:    lines,
			DelCount: n,
		}
	}

	max := n + m
	offset := max
	v := make([]int, 2*max+1)
	v[offset+1] = 0

	history := make([][]int, 0, max+1)
	var finalD int
	found := false

	for d := 0; d <= max; d++ {
		vCopy := make([]int, len(v))
		copy(vCopy, v)
		history = append(history, vCopy)

		for k := -d; k <= d; k += 2 {
			kIdx := offset + k
			var x int
			if k == -d || (k != d && v[kIdx-1] < v[kIdx+1]) {
				x = v[kIdx+1] // vertical move (add)
			} else {
				x = v[kIdx-1] + 1 // horizontal move (delete)
			}
			y := x - k

			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[kIdx] = x

			if x >= n && y >= m {
				finalD = d
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	// Backtrack Myers graph
	x := n
	y := m
	rawOps := make([]rawDiffOp, 0, n+m)

	for d := finalD; d > 0; d-- {
		k := x - y
		prevV := history[d]
		kIdx := offset + k

		var prevK int
		if k == -d || (k != d && prevV[kIdx-1] < prevV[kIdx+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := prevV[offset+prevK]
		prevY := prevX - prevK

		for x > prevX && y > prevY {
			rawOps = append(rawOps, rawDiffOp{
				opType:     DiffSame,
				oldLineNum: x,
				newLineNum: y,
				content:    a[x-1],
			})
			x--
			y--
		}

		if prevX == x {
			// Add
			rawOps = append(rawOps, rawDiffOp{
				opType:     DiffAdd,
				oldLineNum: 0,
				newLineNum: y,
				content:    b[prevY],
			})
		} else {
			// Delete
			rawOps = append(rawOps, rawDiffOp{
				opType:     DiffDel,
				oldLineNum: x,
				newLineNum: 0,
				content:    a[prevX],
			})
		}
		x = prevX
		y = prevY
	}

	for x > 0 && y > 0 {
		rawOps = append(rawOps, rawDiffOp{
			opType:     DiffSame,
			oldLineNum: x,
			newLineNum: y,
			content:    a[x-1],
		})
		x--
		y--
	}

	// Reverse to restore forward sequential order
	for i, j := 0, len(rawOps)-1; i < j; i, j = i+1, j-1 {
		rawOps[i], rawOps[j] = rawOps[j], rawOps[i]
	}

	// Pair consecutive dels and adds into DiffMod
	var finalLines []DiffLine
	var addCount, delCount, modCount int

	idx := 0
	for idx < len(rawOps) {
		if rawOps[idx].opType == DiffSame {
			finalLines = append(finalLines, DiffLine{
				Type:       DiffSame,
				OldLineNum: rawOps[idx].oldLineNum,
				NewLineNum: rawOps[idx].newLineNum,
				Content:    rawOps[idx].content,
			})
			idx++
			continue
		}

		// Gather contiguous block of deletes and adds
		var chunkDels []rawDiffOp
		var chunkAdds []rawDiffOp
		for idx < len(rawOps) && rawOps[idx].opType != DiffSame {
			if rawOps[idx].opType == DiffDel {
				chunkDels = append(chunkDels, rawOps[idx])
			} else if rawOps[idx].opType == DiffAdd {
				chunkAdds = append(chunkAdds, rawOps[idx])
			}
			idx++
		}

		minLen := len(chunkDels)
		if len(chunkAdds) < minLen {
			minLen = len(chunkAdds)
		}

		// Pair into DiffMod
		for k := 0; k < minLen; k++ {
			finalLines = append(finalLines, DiffLine{
				Type:       DiffMod,
				OldLineNum: chunkDels[k].oldLineNum,
				NewLineNum: chunkAdds[k].newLineNum,
				Content:    chunkAdds[k].content,
				OldContent: chunkDels[k].content,
			})
			modCount++
		}

		// Remaining dels
		for k := minLen; k < len(chunkDels); k++ {
			finalLines = append(finalLines, DiffLine{
				Type:       DiffDel,
				OldLineNum: chunkDels[k].oldLineNum,
				NewLineNum: 0,
				Content:    chunkDels[k].content,
			})
			delCount++
		}

		// Remaining adds
		for k := minLen; k < len(chunkAdds); k++ {
			finalLines = append(finalLines, DiffLine{
				Type:       DiffAdd,
				OldLineNum: 0,
				NewLineNum: chunkAdds[k].newLineNum,
				Content:    chunkAdds[k].content,
			})
			addCount++
		}
	}

	return &DiffResult{
		Lines:    finalLines,
		AddCount: addCount,
		DelCount: delCount,
		ModCount: modCount,
	}
}

// GetTextDiff compares two strings and returns a DiffResult.
func GetTextDiff(textA, textB string) (*DiffResult, error) {
	linesA := splitLines(textA)
	linesB := splitLines(textB)
	return ComputeTextDiff(linesA, linesB), nil
}

// CompareConfigFiles reads and compares two files line-by-line.
func CompareConfigFiles(pathA, pathB string) (*DiffResult, error) {
	var linesA, linesB []string

	if pathA != "" {
		if data, err := os.ReadFile(pathA); err == nil {
			linesA = splitLines(string(data))
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read file A %s: %w", pathA, err)
		}
	}

	if pathB != "" {
		if data, err := os.ReadFile(pathB); err == nil {
			linesB = splitLines(string(data))
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read file B %s: %w", pathB, err)
		}
	}

	return ComputeTextDiff(linesA, linesB), nil
}

// CompareSnapshotWithLive compares all files in a snapshot against live Steam userdata.
func CompareSnapshotWithLive(steamPath, backupPath, accountID, snapshotID string) ([]FileDiffItem, error) {
	snapshotDir := filepath.Join(backupPath, "snapshots", accountID, snapshotID)
	fi, err := os.Stat(snapshotDir)
	if err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("快照目录不存在: %s", snapshotDir)
	}

	liveDir := filepath.Join(steamPath, "userdata", accountID, "730")

	allRelPaths := make(map[string]bool)

	// Collect files from snapshot
	_ = filepath.WalkDir(snapshotDir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(snapshotDir, path)
			allRelPaths[filepath.ToSlash(rel)] = true
		}
		return nil
	})

	// Collect files from live userdata if present
	if lfi, err := os.Stat(liveDir); err == nil && lfi.IsDir() {
		_ = filepath.WalkDir(liveDir, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				rel, _ := filepath.Rel(liveDir, path)
				allRelPaths[filepath.ToSlash(rel)] = true
			}
			return nil
		})
	}

	sortedPaths := make([]string, 0, len(allRelPaths))
	for rel := range allRelPaths {
		sortedPaths = append(sortedPaths, rel)
	}
	sort.Strings(sortedPaths)

	items := make([]FileDiffItem, 0, len(sortedPaths))

	for _, rel := range sortedPaths {
		snapFile := filepath.Join(snapshotDir, filepath.FromSlash(rel))
		liveFile := filepath.Join(liveDir, filepath.FromSlash(rel))

		hasSnap := false
		if sfi, err := os.Stat(snapFile); err == nil && !sfi.IsDir() {
			hasSnap = true
		}

		hasLive := false
		if lfi, err := os.Stat(liveFile); err == nil && !lfi.IsDir() {
			hasLive = true
		}

		item := FileDiffItem{
			RelPath: rel,
		}

		if hasSnap && hasLive {
			snapHash, _, _ := computeFileSHA256(snapFile)
			liveHash, _, _ := computeFileSHA256(liveFile)
			item.SnapshotHash = snapHash
			item.LiveHash = liveHash

			diff, _ := CompareConfigFiles(snapFile, liveFile)
			item.Diff = diff

			if snapHash == liveHash {
				item.Status = "identical"
			} else {
				item.Status = "modified"
			}
		} else if hasSnap && !hasLive {
			item.Status = "deleted"
			snapHash, _, _ := computeFileSHA256(snapFile)
			item.SnapshotHash = snapHash
			diff, _ := CompareConfigFiles(snapFile, "")
			item.Diff = diff
		} else if !hasSnap && hasLive {
			item.Status = "added"
			liveHash, _, _ := computeFileSHA256(liveFile)
			item.LiveHash = liveHash
			diff, _ := CompareConfigFiles("", liveFile)
			item.Diff = diff
		}

		items = append(items, item)
	}

	return items, nil
}
