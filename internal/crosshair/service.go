package crosshair

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cs2backup/internal/atomicfile"
)

const (
	// AutoexecFileName 是游戏启动时唯一会自动执行的那份配置。
	AutoexecFileName = "autoexec.cfg"
	// ExportFileName 是导出给用户的独立准星配置文件名。
	ExportFileName = "crosshair.cfg"
)

// ErrCFGDirUnset 表示还没配置 CS2 游戏 cfg 目录。
var ErrCFGDirUnset = errors.New("尚未定位到 CS2 游戏 cfg 目录，请先在路径设置里指定 Steam 安装目录")

// Service 负责 game/csgo/cfg 下的准星读写。
//
// 刻意只操作这一个目录：userdata/<id>/730/local/cfg 下放的是游戏自己读写的
// .vcfg，不会被 exec，写进去等于没设置。
type Service struct {
	cfgDir string
}

// NewService 用 CS2 游戏 cfg 目录构造服务。
func NewService(cfgDir string) *Service {
	return &Service{cfgDir: strings.TrimSpace(cfgDir)}
}

// CFGDir 返回当前使用的 cfg 目录。
func (s *Service) CFGDir() string { return s.cfgDir }

// AutoexecPath 返回 autoexec.cfg 的绝对路径。
func (s *Service) AutoexecPath() string {
	if s.cfgDir == "" {
		return ""
	}
	return filepath.Join(s.cfgDir, AutoexecFileName)
}

// ExportPath 返回 crosshair.cfg 的绝对路径。
func (s *Service) ExportPath() string {
	if s.cfgDir == "" {
		return ""
	}
	return filepath.Join(s.cfgDir, ExportFileName)
}

// State 是 autoexec.cfg 的读取结果。
type State struct {
	// Path 是 autoexec.cfg 的绝对路径。
	Path string `json:"path"`
	// Exists 表示文件是否已存在。
	Exists bool `json:"exists"`
	// Parsed 是解析出的准星现状。
	Parsed ParsedCFG `json:"parsed"`
	// ShareCode 是当前设置编码出的分享码，便于前端展示。
	ShareCode string `json:"share_code"`
}

// Load 读取 autoexec.cfg 并解析准星现状。
//
// 文件不存在不算错误：用户第一次用这个功能时本来就没有 autoexec.cfg，
// 此时返回默认值并标记 Exists=false，让前端提示清楚。
func (s *Service) Load() (*State, error) {
	if s.cfgDir == "" {
		return nil, ErrCFGDirUnset
	}
	path := s.AutoexecPath()

	st := &State{Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// 两个清单都要初始化成空切片：nil 会变成 JSON null，前端按数组用会崩。
			st.Parsed = ParsedCFG{Crosshair: Default(), FoundKeys: []string{}, MissingKeys: []string{}}
			for _, c := range st.Parsed.Crosshair.Convars() {
				st.Parsed.MissingKeys = append(st.Parsed.MissingKeys, c.Key)
			}
			st.ShareCode = Encode(st.Parsed.Crosshair)
			return st, nil
		}
		return nil, fmt.Errorf("读取 %s 失败: %w", AutoexecFileName, err)
	}

	st.Exists = true
	st.Parsed = ParseCFG(string(data))
	st.ShareCode = Encode(st.Parsed.Crosshair)
	return st, nil
}

// SaveOptions 控制写入行为。
type SaveOptions struct {
	// Backup 为 true 时先备份原文件。
	Backup bool
}

// SaveResult 是写入结果。
type SaveResult struct {
	// Path 是写入的 autoexec.cfg 路径。
	Path string `json:"path"`
	// Report 描述具体改了哪些键。
	Report WriteReport `json:"report"`
	// BackupPath 是备份文件路径，未备份时为空。
	BackupPath string `json:"backup_path"`
	// ShareCode 是写入设置对应的分享码。
	ShareCode string `json:"share_code"`
	// Created 表示 autoexec.cfg 是本次新建的。
	Created bool `json:"created"`
}

// Save 把准星写进 autoexec.cfg。
func (s *Service) Save(ch Crosshair, opts SaveOptions) (*SaveResult, error) {
	if s.cfgDir == "" {
		return nil, ErrCFGDirUnset
	}
	if err := os.MkdirAll(s.cfgDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建 cfg 目录失败: %w", err)
	}

	path := s.AutoexecPath()
	original, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("读取 %s 失败: %w", AutoexecFileName, err)
	}
	exists := err == nil

	shareCode := Encode(ch)
	updated, report := ApplyToCFG(string(original), ch, shareCode)

	res := &SaveResult{
		Path:      path,
		Report:    report,
		ShareCode: shareCode,
		Created:   !exists,
	}

	writeRes, werr := atomicfile.Write(path, []byte(updated), atomicfile.Options{Backup: opts.Backup})
	if werr != nil {
		return nil, werr
	}
	res.BackupPath = writeRes.BackupPath
	return res, nil
}

// ExportCFG 写出独立的 crosshair.cfg，返回文件路径。
func (s *Service) ExportCFG(ch Crosshair, generatedAt string) (string, error) {
	if s.cfgDir == "" {
		return "", ErrCFGDirUnset
	}
	path := s.ExportPath()
	content := RenderCFG(ch, Encode(ch), generatedAt)
	if _, err := atomicfile.Write(path, []byte(content), atomicfile.Options{}); err != nil {
		return "", err
	}
	return path, nil
}

