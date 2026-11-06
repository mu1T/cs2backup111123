// Package atomicfile 统一「把一份文本写到磁盘」这件事。
//
// 这个仓库里原本有五套写法，各自漏掉不同的步骤：有的忘了建目录，有的忘了
// 解只读，有的写一半崩了会留下半截文件。其中 launch options 那份函数名叫
// atomicWriteFile，实现却是 os.WriteFile——因为它曾经用 rename，撞上了
// Steam 持有的目录，只好退回直接覆盖，名字没跟着改。
//
// 这里把「怎么写」收敛成两个入口，因为存在一个真实的区别：文件归谁所有。
//
//   - Write：文件归我们，或游戏只在启动时读一次（状态文件、autoexec.cfg）。
//     临时文件 + rename，中途被打断也不会留下半截内容。
//   - WriteInPlace：目录被别的进程持有（Steam 运行中的 localconfig.vdf、
//     cs2_video.txt）。那里 rename 会失败，而删掉重建会让监视该文件的进程
//     丢掉句柄，只能原地覆盖。
//
// 两者共享的部分——建目录、解只读、备份——只写一遍。
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Options 控制写入时的附加动作。零值即「只写，不备份，权限 0644」。
type Options struct {
	// Backup 为 true 时，先把原文件复制成 <path>.bak.<时间戳>。
	// 原文件不存在则跳过。
	Backup bool

	// Mode 是新建文件（含临时文件）的权限，0 时按 0o644。
	Mode os.FileMode
}

// Result 描述这次写入实际做了什么，供上层拼提示文案。
type Result struct {
	// BackupPath 是备份文件路径，未备份时为空。
	BackupPath string
	// Created 表示目标文件此前不存在。
	Created bool
}

func (o Options) mode() os.FileMode {
	if o.Mode == 0 {
		return 0o644
	}
	return o.Mode
}

// Write 先写临时文件再 rename，适用于我们拥有的文件，以及游戏只在启动时
// 读一次的 cfg。目标只读时会先解锁，否则 rename 会失败。
//
// Windows 上目标被占用时 rename 会失败，这时退回原地覆盖——比直接报错让
// 用户卡住要好，至少内容是对的。
func Write(path string, data []byte, opt Options) (Result, error) {
	res, err := prepare(path, data, opt)
	if err != nil {
		return res, err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*"+filepath.Ext(path))
	if err != nil {
		return res, fmt.Errorf("创建临时文件失败（目录可能不可写）: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		return res, fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return res, fmt.Errorf("刷新临时文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return res, fmt.Errorf("关闭临时文件失败: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		if werr := os.WriteFile(path, data, opt.mode()); werr != nil {
			return res, fmt.Errorf("写入 %s 失败: %w", filepath.Base(path), werr)
		}
	}
	return res, nil
}

// WriteInPlace 直接覆盖目标文件，适用于目录被其它进程持有的场景：
// 那里 rename 会失败，而删掉重建会让监视该文件的进程丢掉句柄。
func WriteInPlace(path string, data []byte, opt Options) (Result, error) {
	res, err := prepare(path, data, opt)
	if err != nil {
		return res, err
	}
	if err := os.WriteFile(path, data, opt.mode()); err != nil {
		return res, fmt.Errorf("写入 %s 失败: %w", filepath.Base(path), err)
	}
	return res, nil
}

// prepare 做两个入口共用的前置动作：建目录、备份、解只读。
func prepare(path string, data []byte, opt Options) (Result, error) {
	var res Result

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return res, fmt.Errorf("创建目录 %s 失败: %w", dir, err)
	}

	fi, statErr := os.Stat(path)
	exists := statErr == nil && !fi.IsDir()
	res.Created = !exists

	if opt.Backup && exists {
		backupPath, err := backup(path)
		if err != nil {
			return res, err
		}
		res.BackupPath = backupPath
	}

	// 目标若是只读文件，先解除只读，否则写入或 rename 都会失败。
	// 只解不改回去：调用方若要保留只读（画质配置就是），写完自己再设。
	if exists && fi.Mode().Perm()&0o200 == 0 {
		_ = os.Chmod(path, opt.mode())
	}

	return res, nil
}

// backup 把原文件另存一份，文件名带时间戳，同秒内多次备份靠序号区分。
func backup(path string) (string, error) {
	original, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取待备份文件失败: %w", err)
	}

	stamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.bak.%s", path, stamp)
	for i := 2; ; i++ {
		if _, statErr := os.Stat(backupPath); os.IsNotExist(statErr) {
			break
		}
		backupPath = fmt.Sprintf("%s.bak.%s-%d", path, stamp, i)
	}

	if err := os.WriteFile(backupPath, original, 0o644); err != nil {
		return "", fmt.Errorf("备份 %s 失败: %w", filepath.Base(path), err)
	}
	return backupPath, nil
}
