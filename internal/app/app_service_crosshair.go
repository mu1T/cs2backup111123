package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"cs2backup/internal/crosshair"
)

// crosshairService 用当前配置的 CS2 游戏 cfg 目录构造准星服务。
//
// 只有 game/csgo/cfg 下的 cfg 会被游戏自动执行，userdata 下的 .vcfg 是游戏自己
// 读写的状态文件，所以这里固定只认这一个目录。
func (s *AppService) crosshairService() (*crosshair.Service, error) {
	cfgDir, err := s.GetCS2CFGPath()
	if err != nil {
		return nil, err
	}
	if cfgDir == "" {
		return nil, crosshair.ErrCFGDirUnset
	}
	return crosshair.NewService(cfgDir), nil
}

// GetCrosshairState 读取 autoexec.cfg 里的准星现状。
func (s *AppService) GetCrosshairState() (*crosshair.State, error) {
	svc, err := s.crosshairService()
	if err != nil {
		return nil, err
	}
	state, err := svc.Load()
	if err != nil {
		s.LogError(fmt.Sprintf("读取准星设置失败: %v", err))
		return nil, err
	}
	return state, nil
}

// SaveCrosshair 把准星写进 autoexec.cfg，写入前自动备份。
func (s *AppService) SaveCrosshair(ch crosshair.Crosshair) (*crosshair.SaveResult, error) {
	svc, err := s.crosshairService()
	if err != nil {
		return nil, err
	}

	s.LogInfo("正在把准星设置写入 autoexec.cfg...")
	res, err := svc.Save(ch, crosshair.SaveOptions{Backup: true})
	if err != nil {
		s.LogError(fmt.Sprintf("写入准星设置失败: %v", err))
		return nil, err
	}

	verb := "更新"
	if res.Created {
		verb = "创建"
	}
	s.LogSuccess(fmt.Sprintf(
		"已%s autoexec.cfg：原位替换 %d 项，追加 %d 项，注释旧键 %d 项（准星开关保留为 %s）",
		verb, len(res.Report.ReplacedKeys), len(res.Report.AppendedKeys),
		len(res.Report.CommentedKeys), describeToggle(res.Report),
	))
	return res, nil
}

// describeToggle 把准星开关的保留情况转成人话。
func describeToggle(r crosshair.WriteReport) string {
	if !r.PreservedToggle {
		return "文件未设置，保持原样"
	}
	return r.DisplayToggle
}

// DecodeCrosshairCode 解析分享码，兼容旧 CSGO- 格式。
//
// accountID 用来取该账号的 CS2 分辨率：legacy-v1 的尺寸是「480 像素高屏幕的比例」，
// 换算成像素需要知道「当前高度」。拿不到就退回 1080（旧行为的默认值）。
func (s *AppService) DecodeCrosshairCode(accountID, code string) (*crosshair.Crosshair, error) {
	ch, err := crosshair.DecodeWithRef(code, s.referenceHeightFor(accountID))
	if err != nil {
		s.LogError(fmt.Sprintf("解析准星分享码失败: %v", err))
		return nil, err
	}
	format := string(ch.Format)
	if ch.Format != crosshair.FormatCS2V1 {
		// 旧码的尺寸基准与现在不同，换算只是近似，必须提示用户核对。
		s.LogInfo(fmt.Sprintf("已导入 %s 旧版分享码，尺寸按 1080 基准近似换算，请在预览里核对", format))
	}
	return ch, nil
}

// referenceHeightFor 取该账号 CS2 视频配置里的分辨率高度，拿不到就用 1080。
func (s *AppService) referenceHeightFor(accountID string) int {
	const fallback = 1080
	if accountID == "" || accountID == "game_cfg" {
		return fallback
	}
	data, err := s.GetVideoSettings(accountID)
	if err != nil || data == nil {
		return fallback
	}
	raw := strings.TrimSpace(data.Settings["setting.defaultresheight"])
	h, convErr := strconv.Atoi(raw)
	if convErr != nil {
		return fallback
	}
	return crosshair.ScreenHeightFromDisplay(h)
}

// EncodeCrosshairCode 把准星编码成当前可用的分享码。
func (s *AppService) EncodeCrosshairCode(ch crosshair.Crosshair) (string, error) {
	return crosshair.Encode(ch), nil
}

// ExportCrosshairCFG 写出独立的 crosshair.cfg 并返回路径。
func (s *AppService) ExportCrosshairCFG(ch crosshair.Crosshair) (string, error) {
	svc, err := s.crosshairService()
	if err != nil {
		return "", err
	}
	path, err := svc.ExportCFG(ch, time.Now().Format("2006-01-02 15:04"))
	if err != nil {
		s.LogError(fmt.Sprintf("导出 crosshair.cfg 失败: %v", err))
		return "", err
	}
	s.LogSuccess(fmt.Sprintf("已导出准星配置: %s", path))
	return path, nil
}

// GetCrosshairConsole 返回单行控制台指令，方便用户直接粘贴。
func (s *AppService) GetCrosshairConsole(ch crosshair.Crosshair) string {
	return crosshair.RenderConsole(ch)
}

// GetCrosshairPresets 返回可见的内置预设 + 用户预设。
func (s *AppService) GetCrosshairPresets() []crosshair.Preset {
	s.mu.RLock()
	store := s.crosshairPresets
	s.mu.RUnlock()
	if store == nil {
		// 存储初始化失败时至少还能用内置预设，不让整块功能空掉。
		return crosshair.BuiltinPresets()
	}
	return store.List()
}

// GetHiddenCrosshairPresets 返回被隐藏的内置预设，供「恢复」入口使用。
func (s *AppService) GetHiddenCrosshairPresets() []crosshair.Preset {
	s.mu.RLock()
	store := s.crosshairPresets
	s.mu.RUnlock()
	if store == nil {
		// 空切片而不是 nil：nil 会序列化成 null，前端按数组用会抛异常。
		return []crosshair.Preset{}
	}
	return store.HiddenBuiltins()
}

// SaveCrosshairPreset 把一份准星存成用户预设。
//
// 返回 Preset 而不是 UserPreset：前端拿到的预设列表元素就是这个形状，
// 返回同一种结构才不会出现「字段少一半」的运行时惊喜。
func (s *AppService) SaveCrosshairPreset(name string, ch crosshair.Crosshair) (*crosshair.Preset, error) {
	s.mu.RLock()
	store := s.crosshairPresets
	s.mu.RUnlock()
	if store == nil {
		return nil, fmt.Errorf("准星预设存储未初始化")
	}
	preset, err := store.AddUser(name, ch)
	if err != nil {
		s.LogError(fmt.Sprintf("保存准星预设失败: %v", err))
		return nil, err
	}
	s.LogSuccess(fmt.Sprintf("已保存准星预设「%s」", preset.Name))
	return preset.ToPreset(), nil
}

// DeleteCrosshairPreset 删除一份用户预设。
func (s *AppService) DeleteCrosshairPreset(id string) error {
	s.mu.RLock()
	store := s.crosshairPresets
	s.mu.RUnlock()
	if store == nil {
		return fmt.Errorf("准星预设存储未初始化")
	}
	if err := store.DeleteUser(id); err != nil {
		s.LogError(fmt.Sprintf("删除准星预设失败: %v", err))
		return err
	}
	s.LogSuccess("已删除准星预设")
	return nil
}

// SetCrosshairPresetHidden 隐藏 / 恢复某个内置预设。
func (s *AppService) SetCrosshairPresetHidden(id string, hidden bool) error {
	s.mu.RLock()
	store := s.crosshairPresets
	s.mu.RUnlock()
	if store == nil {
		return fmt.Errorf("准星预设存储未初始化")
	}
	if err := store.SetHidden(id, hidden); err != nil {
		s.LogError(fmt.Sprintf("调整准星预设显隐失败: %v", err))
		return err
	}
	return nil
}

// GetCrosshairCommands 返回当前设置的逐条指令与单行控制台串。
//
// 编辑器里要「看得见键」，所以这里把 convars 原样返回，而不是让前端自己拼。
func (s *AppService) GetCrosshairCommands(ch crosshair.Crosshair) crosshair.CommandPreview {
	ch.Normalize()
	return crosshair.CommandPreview{
		Lines:   ch.Convars(),
		Console: crosshair.RenderConsole(ch),
		Share:   crosshair.Encode(ch),
	}
}
