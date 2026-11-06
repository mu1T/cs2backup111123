package crosshair

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cs2backup/internal/atomicfile"
)

// ErrPresetNotFound 表示要删除 / 修改的预设不存在。
var ErrPresetNotFound = errors.New("预设不存在")

// UserPreset 是用户自己存的准星模板。
type UserPreset struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt string    `json:"created_at"`
	Crosshair Crosshair `json:"crosshair"`
}

// ToPreset 把用户预设转成统一的 Preset 形状，供前端列表与新增接口共用。
func (u UserPreset) ToPreset() *Preset {
	return &Preset{
		ID:          u.ID,
		Name:        u.Name,
		Description: "我的预设 · " + u.CreatedAt,
		Builtin:     false,
		Crosshair:   u.Crosshair,
	}
}

// presetStoreData 是落盘结构。
//
// 内置预设不落盘（代码里就是真值，改了版本自动生效），只存「隐藏了哪些」
// 和「用户加了哪些」，这样升级不会把用户的自定义预设冲掉。
type presetStoreData struct {
	HiddenBuiltin []string     `json:"hidden_builtin"`
	User          []UserPreset `json:"user"`
}

// PresetStore 管理内置预设的显隐与用户预设的增删。
type PresetStore struct {
	mu   sync.RWMutex
	path string
	data presetStoreData
}

// NewPresetStore 在 <configDir>/crosshair_presets.json 上构造存储。
func NewPresetStore(configDir string) (*PresetStore, error) {
	if strings.TrimSpace(configDir) == "" {
		return nil, errors.New("配置目录不能为空")
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建配置目录失败: %w", err)
	}
	s := &PresetStore{path: filepath.Join(configDir, "crosshair_presets.json")}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *PresetStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.data = presetStoreData{}
			return nil
		}
		return fmt.Errorf("读取准星预设失败: %w", err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		s.data = presetStoreData{}
		return nil
	}
	var data presetStoreData
	if err := json.Unmarshal(raw, &data); err != nil {
		// 文件损坏时不让整个功能挂掉，退回空集合并保留原文件供用户排查。
		s.data = presetStoreData{}
		return nil
	}
	s.data = data
	return nil
}

func (s *PresetStore) persistLocked() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化准星预设失败: %w", err)
	}
	if _, err := atomicfile.Write(s.path, raw, atomicfile.Options{}); err != nil {
		return fmt.Errorf("保存准星预设失败: %w", err)
	}
	return nil
}

// List 返回「可见的内置预设 + 全部用户预设」。
//
// 内置在前、用户在后，各自保持插入顺序，UI 不需要再排。
func (s *PresetStore) List() []Preset {
	s.mu.RLock()
	hidden := make(map[string]bool, len(s.data.HiddenBuiltin))
	for _, id := range s.data.HiddenBuiltin {
		hidden[id] = true
	}
	user := append([]UserPreset(nil), s.data.User...)
	s.mu.RUnlock()

	out := make([]Preset, 0, len(user)+10)
	for _, p := range BuiltinPresets() {
		if hidden[p.ID] {
			continue
		}
		out = append(out, p)
	}
	for _, u := range user {
		out = append(out, *u.ToPreset())
	}
	return out
}

// HiddenBuiltins 返回被隐藏的内置预设 ID，供 UI 提供「恢复」入口。
func (s *PresetStore) HiddenBuiltins() []Preset {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Preset, 0, len(s.data.HiddenBuiltin))
	for _, id := range s.data.HiddenBuiltin {
		if p, ok := FindBuiltinPreset(id); ok {
			out = append(out, p)
		}
	}
	return out
}

// AddUser 存一份用户预设，返回落盘后的记录。
func (s *PresetStore) AddUser(name string, ch Crosshair) (UserPreset, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return UserPreset{}, errors.New("预设名称不能为空")
	}
	if len([]rune(trimmed)) > 40 {
		trimmed = string([]rune(trimmed)[:40])
	}
	ch.Normalize()

	now := time.Now()
	preset := UserPreset{
		ID:        fmt.Sprintf("user-%d", now.UnixNano()),
		Name:      trimmed,
		CreatedAt: now.Format("2006-01-02 15:04"),
		Crosshair: ch,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.User = append(s.data.User, preset)
	if err := s.persistLocked(); err != nil {
		// 落盘失败就回滚内存，避免「界面有、重启没了」。
		s.data.User = s.data.User[:len(s.data.User)-1]
		return UserPreset{}, err
	}
	return preset, nil
}

// DeleteUser 删除一份用户预设。
func (s *PresetStore) DeleteUser(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i, u := range s.data.User {
		if u.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrPresetNotFound
	}
	prev := append([]UserPreset(nil), s.data.User...)
	s.data.User = append(s.data.User[:idx], s.data.User[idx+1:]...)
	if err := s.persistLocked(); err != nil {
		// 回滚要放回原位置：按 ID 重排会打乱用户看到的顺序。
		s.data.User = prev
		return err
	}
	return nil
}

// SetHidden 设置某个内置预设是否隐藏。用户预设不支持隐藏，只能删。
func (s *PresetStore) SetHidden(id string, hidden bool) error {
	if _, ok := FindBuiltinPreset(id); !ok {
		return ErrPresetNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	has := false
	for _, x := range s.data.HiddenBuiltin {
		if x == id {
			has = true
			break
		}
	}
	if hidden == has {
		return nil
	}

	prev := append([]string(nil), s.data.HiddenBuiltin...)
	if hidden {
		s.data.HiddenBuiltin = append(s.data.HiddenBuiltin, id)
	} else {
		kept := make([]string, 0, len(s.data.HiddenBuiltin))
		for _, x := range s.data.HiddenBuiltin {
			if x != id {
				kept = append(kept, x)
			}
		}
		s.data.HiddenBuiltin = kept
	}
	if err := s.persistLocked(); err != nil {
		s.data.HiddenBuiltin = prev
		return err
	}
	return nil
}
