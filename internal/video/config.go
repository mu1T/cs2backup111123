package video

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"cs2backup/internal/atomicfile"
)

var bufPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 32*1024))
	},
}

// VideoConfigData encapsulates the video configuration exposed to frontend and callers.
type VideoConfigData struct {
	FilePath string            `json:"file_path"`
	ReadOnly bool              `json:"read_only"`
	Settings map[string]string `json:"settings"`
}

// ConfigLineType distinguishes between KeyValue pairs and raw lines (comments, brackets, section names).
type ConfigLineType int

const (
	LineRaw ConfigLineType = iota
	LineKV
)

// ConfigLine represents an individual line in cs2_video.txt.
type ConfigLine struct {
	Type   ConfigLineType
	Raw    string
	Indent string
	Key    string
	Value  string
}

// ConfigFile contains the parsed structure of cs2_video.txt.
type ConfigFile struct {
	FilePath   string
	Lines      []ConfigLine
	Settings   map[string]string
	ReadOnly   bool
	LineEnding string
}

// GetAccountVideoPath returns the full path to cs2_video.txt for the given account ID.
func GetAccountVideoPath(steamPath, accountID string) string {
	return filepath.Join(steamPath, "userdata", accountID, "730", "local", "cfg", "cs2_video.txt")
}

// parseKVLine parses a KeyValues line without regular expression backtracking.
// It returns indent, key, value, and true if the line is a valid quoted key-value pair.
func parseKVLine(line string) (indent, key, val string, ok bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, "\"") {
		return "", "", "", false
	}
	indent = line[:len(line)-len(trimmed)]

	// Find the closing quote of key
	secondQuote := strings.Index(trimmed[1:], "\"")
	if secondQuote < 0 {
		return "", "", "", false
	}
	secondQuote += 1 // offset by 1 because we sliced trimmed[1:]
	key = trimmed[1:secondQuote]

	rem := strings.TrimLeft(trimmed[secondQuote+1:], " \t")
	if !strings.HasPrefix(rem, "\"") {
		return "", "", "", false
	}
	fourthQuote := strings.Index(rem[1:], "\"")
	if fourthQuote < 0 {
		return "", "", "", false
	}
	fourthQuote += 1
	val = rem[1:fourthQuote]

	return indent, key, val, true
}

// LoadConfigFile reads and parses a cs2_video.txt file, detecting its read-only status and line endings.
func LoadConfigFile(path string) (*ConfigFile, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("视频配置文件不存在: %w", err)
	}

	ro, _ := IsReadOnly(path)

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("无法读取视频配置文件: %w", err)
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	var lines []ConfigLine
	settings := make(map[string]string)
	detectedLineEnding := "\n"
	hasCRLF := false

	for {
		lineWithEnding, err := reader.ReadString('\n')
		if len(lineWithEnding) > 0 {
			if strings.HasSuffix(lineWithEnding, "\r\n") {
				hasCRLF = true
			}
			// Strip line endings for parsing
			cleanLine := strings.TrimRight(lineWithEnding, "\r\n")

			indent, key, val, isKV := parseKVLine(cleanLine)
			if isKV {
				settings[key] = val
				lines = append(lines, ConfigLine{
					Type:   LineKV,
					Indent: indent,
					Key:    key,
					Value:  val,
				})
			} else {
				lines = append(lines, ConfigLine{
					Type: LineRaw,
					Raw:  lineWithEnding,
				})
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("解析视频配置文件失败: %w", err)
		}
	}

	if hasCRLF {
		detectedLineEnding = "\r\n"
	}

	return &ConfigFile{
		FilePath:   path,
		Lines:      lines,
		Settings:   settings,
		ReadOnly:   ro,
		LineEnding: detectedLineEnding,
	}, nil
}

// SaveConfigFile writes or updates cs2_video.txt with updated settings and sets the read-only attribute.
func SaveConfigFile(path string, updatedSettings map[string]string, setReadOnly bool) error {
	// 建目录、解只读都由 atomicfile 负责；这里只关心内容怎么变。
	var cf *ConfigFile
	if _, err := os.Stat(path); err == nil {
		loaded, loadErr := LoadConfigFile(path)
		if loadErr == nil {
			cf = loaded
		}
	}

	if cf == nil {
		// Create default skeleton
		cf = &ConfigFile{
			FilePath:   path,
			LineEnding: "\r\n",
			Settings:   make(map[string]string),
			Lines: []ConfigLine{
				{Type: LineRaw, Raw: "\"video.cfg\"\r\n"},
				{Type: LineRaw, Raw: "{\r\n"},
				{Type: LineRaw, Raw: "}\r\n"},
			},
		}
	}

	// Update existing KV lines and record updated keys
	updatedKeys := make(map[string]bool)
	for i := range cf.Lines {
		if cf.Lines[i].Type == LineKV {
			k := cf.Lines[i].Key
			if newVal, exists := updatedSettings[k]; exists {
				cf.Lines[i].Value = newVal
				updatedKeys[k] = true
			}
		}
	}

	// Check if there are new settings not yet in lines
	var missingLines []ConfigLine
	for k, v := range updatedSettings {
		if !updatedKeys[k] {
			missingLines = append(missingLines, ConfigLine{
				Type:   LineKV,
				Indent: "\t",
				Key:    k,
				Value:  v,
			})
		}
	}

	// Insert missing lines before the closing brace '}' if found
	if len(missingLines) > 0 {
		insertIdx := -1
		for i := len(cf.Lines) - 1; i >= 0; i-- {
			if cf.Lines[i].Type == LineRaw && strings.TrimSpace(cf.Lines[i].Raw) == "}" {
				insertIdx = i
				break
			}
		}

		if insertIdx >= 0 {
			tail := append([]ConfigLine{}, cf.Lines[insertIdx:]...)
			cf.Lines = append(cf.Lines[:insertIdx], missingLines...)
			cf.Lines = append(cf.Lines, tail...)
		} else {
			cf.Lines = append(cf.Lines, missingLines...)
		}
	}

	// Buffer write using sync.Pool for low memory overhead
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	for _, line := range cf.Lines {
		if line.Type == LineKV {
			indent := line.Indent
			if indent == "" {
				indent = "\t"
			}
			buf.WriteString(fmt.Sprintf("%s\"%s\"\t\t\"%s\"%s", indent, line.Key, line.Value, cf.LineEnding))
		} else {
			buf.WriteString(line.Raw)
		}
	}

	// Write out to file
	// cs2_video.txt 在游戏运行时被 Steam 持有，rename 会失败，只能原地覆盖。
	if _, err := atomicfile.WriteInPlace(path, buf.Bytes(), atomicfile.Options{Mode: 0o666}); err != nil {
		return err
	}

	// Apply read-only status
	if err := SetReadOnly(path, setReadOnly); err != nil {
		return fmt.Errorf("设置文件只读属性失败: %w", err)
	}

	return nil
}
