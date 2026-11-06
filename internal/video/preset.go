package video

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cs2backup/internal/atomicfile"
)

// VideoPreset represents a saved video configuration template.
type VideoPreset struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	CreatedAt string            `json:"created_at"`
	IsDefault bool              `json:"is_default"`
	FilePath  string            `json:"file_path"`
	Settings  map[string]string `json:"settings"`
}

// PresetManifest manages the collection of saved presets and the default preset pointer.
type PresetManifest struct {
	DefaultPresetID string        `json:"default_preset_id"`
	Presets         []VideoPreset `json:"presets"`
}

// PresetManager provides persistent storage and lifecycle methods for user video presets.
type PresetManager struct {
	mu           sync.RWMutex
	dir          string
	manifestPath string
	// now 只用来打创建时间戳。预设 ID 不读时钟（见 ImportPreset），
	// 测试可以把这里冻结成一个固定时刻，验证 ID 与时间无关。
	now func() time.Time
}

// NewPresetManager initializes a new PresetManager located at <configDir>/video_presets.
func NewPresetManager(configDir string) (*PresetManager, error) {
	dir := filepath.Join(configDir, "video_presets")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建预设目录失败: %w", err)
	}

	manifestPath := filepath.Join(dir, "manifest.json")
	pm := &PresetManager{
		dir:          dir,
		manifestPath: manifestPath,
		now:          time.Now,
	}

	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		initial := PresetManifest{
			DefaultPresetID: "",
			Presets:         []VideoPreset{},
		}
		if err := pm.saveManifestLocked(&initial); err != nil {
			return nil, err
		}
	}

	return pm, nil
}

// loadManifestLocked reads the manifest from disk.
func (pm *PresetManager) loadManifestLocked() (*PresetManifest, error) {
	data, err := os.ReadFile(pm.manifestPath)
	if err != nil {
		return nil, fmt.Errorf("读取预设清单失败: %w", err)
	}

	var m PresetManifest
	if err := json.Unmarshal(data, &m); err != nil {
		// Corrupted or empty manifest recovery
		m = PresetManifest{
			DefaultPresetID: "",
			Presets:         []VideoPreset{},
		}
	}

	// Synchronize IsDefault flag on all presets
	for i := range m.Presets {
		m.Presets[i].IsDefault = (m.Presets[i].ID == m.DefaultPresetID)
	}

	return &m, nil
}

// saveManifestLocked writes the manifest to disk.
func (pm *PresetManager) saveManifestLocked(m *PresetManifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化预设清单失败: %w", err)
	}
	// 清单是我们自己的状态文件，写一半崩掉会让全部预设消失，走原子写。
	_, err = atomicfile.Write(pm.manifestPath, data, atomicfile.Options{Mode: 0o666})
	return err
}

// ImportPreset copies an external .txt file to the preset repository and registers it in the manifest.
func (pm *PresetManager) ImportPreset(srcFilePath, name string, setAsDefault bool) (*VideoPreset, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	cf, err := LoadConfigFile(srcFilePath)
	if err != nil {
		return nil, fmt.Errorf("导入源文件无效: %w", err)
	}

	if name == "" {
		base := filepath.Base(srcFilePath)
		ext := filepath.Ext(base)
		name = base[:len(base)-len(ext)]
	}

	// 预设 ID 曾用 time.Now().UnixNano() 拼，靠时钟精度保证唯一。
	// Windows 上这个时钟的步进远大于 1ns：两次导入落在同一个 tick 就撞 ID，
	// 于是两个预设指向同一个文件（后写的覆盖前一个，删一个等于删两个），
	// 且 IsDefault 会被算成 true。改成让文件系统分配名字，唯一性不再依赖时钟。
	target, err := os.CreateTemp(pm.dir, "preset_*.txt")
	if err != nil {
		return nil, fmt.Errorf("创建预设文件失败: %w", err)
	}
	targetPath := target.Name()
	presetID := strings.TrimSuffix(filepath.Base(targetPath), ".txt")

	// Copy the file content
	srcData, err := os.ReadFile(srcFilePath)
	if err != nil {
		_ = target.Close()
		_ = os.Remove(targetPath)
		return nil, fmt.Errorf("读取源文件失败: %w", err)
	}
	if _, err := target.Write(srcData); err != nil {
		_ = target.Close()
		_ = os.Remove(targetPath)
		return nil, fmt.Errorf("保存预设文件失败: %w", err)
	}
	if err := target.Close(); err != nil {
		_ = os.Remove(targetPath)
		return nil, fmt.Errorf("保存预设文件失败: %w", err)
	}

	m, err := pm.loadManifestLocked()
	if err != nil {
		return nil, err
	}

	if setAsDefault {
		m.DefaultPresetID = presetID
		for i := range m.Presets {
			m.Presets[i].IsDefault = false
		}
	}

	preset := VideoPreset{
		ID:        presetID,
		Name:      name,
		CreatedAt: pm.now().Format("2006-01-02 15:04:05"),
		IsDefault: setAsDefault,
		FilePath:  targetPath,
		Settings:  cf.Settings,
	}

	m.Presets = append(m.Presets, preset)
	if err := pm.saveManifestLocked(m); err != nil {
		_ = os.Remove(targetPath)
		return nil, err
	}

	return &preset, nil
}

// SaveAccountAsPreset copies an account's cs2_video.txt into the preset repository.
func (pm *PresetManager) SaveAccountAsPreset(steamPath, accountID, name string, setAsDefault bool) (*VideoPreset, error) {
	srcPath := GetAccountVideoPath(steamPath, accountID)
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("账号 [%s] 尚不存在 cs2_video.txt 配置文件", accountID)
	}

	if name == "" {
		name = fmt.Sprintf("账号 %s 自定义预设", accountID)
	}

	return pm.ImportPreset(srcPath, name, setAsDefault)
}

// GetPresets returns all registered presets with updated IsDefault flags.
func (pm *PresetManager) GetPresets() ([]VideoPreset, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	m, err := pm.loadManifestLocked()
	if err != nil {
		return nil, err
	}
	return m.Presets, nil
}

// GetPreset finds a specific preset by its unique ID.
func (pm *PresetManager) GetPreset(presetID string) (*VideoPreset, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	m, err := pm.loadManifestLocked()
	if err != nil {
		return nil, err
	}

	for _, p := range m.Presets {
		if p.ID == presetID {
			item := p
			return &item, nil
		}
	}
	return nil, fmt.Errorf("未找到预设: %s", presetID)
}

// SetDefaultPreset designates the specified preset as the default preset.
func (pm *PresetManager) SetDefaultPreset(presetID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	m, err := pm.loadManifestLocked()
	if err != nil {
		return err
	}

	found := false
	for i := range m.Presets {
		if m.Presets[i].ID == presetID {
			found = true
			m.Presets[i].IsDefault = true
		} else {
			m.Presets[i].IsDefault = false
		}
	}

	if !found {
		return fmt.Errorf("未找到预设: %s", presetID)
	}

	m.DefaultPresetID = presetID
	return pm.saveManifestLocked(m)
}

// DeletePreset removes the preset file from disk and manifest.
func (pm *PresetManager) DeletePreset(presetID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	m, err := pm.loadManifestLocked()
	if err != nil {
		return err
	}

	idx := -1
	for i, p := range m.Presets {
		if p.ID == presetID {
			idx = i
			break
		}
	}

	if idx == -1 {
		return fmt.Errorf("未找到预设: %s", presetID)
	}

	targetFile := m.Presets[idx].FilePath
	_ = os.Remove(targetFile)

	m.Presets = append(m.Presets[:idx], m.Presets[idx+1:]...)

	if m.DefaultPresetID == presetID {
		m.DefaultPresetID = ""
	}

	return pm.saveManifestLocked(m)
}

// ApplyPreset applies the preset settings to the target account's cs2_video.txt and sets read-only lock.
func (pm *PresetManager) ApplyPreset(steamPath, accountID, presetID string, setReadOnly bool) error {
	preset, err := pm.GetPreset(presetID)
	if err != nil {
		return err
	}

	targetPath := GetAccountVideoPath(steamPath, accountID)
	return SaveConfigFile(targetPath, preset.Settings, setReadOnly)
}

// ApplyDefaultPreset applies the default preset to the target account's cs2_video.txt.
func (pm *PresetManager) ApplyDefaultPreset(steamPath, accountID string, setReadOnly bool) error {
	pm.mu.RLock()
	m, err := pm.loadManifestLocked()
	pm.mu.RUnlock()
	if err != nil {
		return err
	}

	if m.DefaultPresetID == "" {
		return fmt.Errorf("尚未设置全局默认视频预设，请先指定默认预设")
	}

	return pm.ApplyPreset(steamPath, accountID, m.DefaultPresetID, setReadOnly)
}
