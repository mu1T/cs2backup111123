package backup

import (
	"os"
	"path/filepath"
	"testing"

	"cs2backup/internal/testutil"
)

func setupTestBackupService(t *testing.T) (*Service, string, string) {
	tempDir := testutil.TempDir(t)
	steamDir := filepath.Join(tempDir, "Steam")
	backupDir := filepath.Join(tempDir, "Backup")

	// Create user cfg directory: userdata/123456/730/local/cfg
	userCfgDir := filepath.Join(steamDir, "userdata", "123456", "730", "local", "cfg")
	if err := os.MkdirAll(userCfgDir, 0755); err != nil {
		t.Fatalf("failed to create user cfg dir: %v", err)
	}

	// Create game cfg directory: steamapps/common/Counter-Strike Global Offensive/game/csgo/cfg
	gameCfgDir := filepath.Join(steamDir, "steamapps", "common", "Counter-Strike Global Offensive", "game", "csgo", "cfg")
	if err := os.MkdirAll(gameCfgDir, 0755); err != nil {
		t.Fatalf("failed to create game cfg dir: %v", err)
	}

	svc, err := NewService(tempDir, nil)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	svc.SetSteamPath(steamDir)
	if err := svc.SetBackupPath(backupDir); err != nil {
		t.Fatalf("SetBackupPath failed: %v", err)
	}

	return svc, userCfgDir, gameCfgDir
}

func TestReadConfigFile_UserConfig_Success(t *testing.T) {
	svc, userCfgDir, _ := setupTestBackupService(t)

	testContent := "bind \"w\" \"+forward\"\nbind \"s\" \"+back\"\n"
	filePath := filepath.Join(userCfgDir, "cs2_user_keys.vcfg")
	if err := os.WriteFile(filePath, []byte(testContent), 0644); err != nil {
		t.Fatal(err)
	}

	content, err := svc.ReadConfigFile("123456", "cs2_user_keys.vcfg")
	if err != nil {
		t.Fatalf("ReadConfigFile failed: %v", err)
	}
	if content != testContent {
		t.Errorf("content mismatch: got %q, want %q", content, testContent)
	}
}

func TestReadConfigFile_GameCFG_Success(t *testing.T) {
	svc, _, gameCfgDir := setupTestBackupService(t)

	testContent := "echo \"Autoexec Loaded Successfully\"\nfps_max 0\n"
	filePath := filepath.Join(gameCfgDir, "autoexec.cfg")
	if err := os.WriteFile(filePath, []byte(testContent), 0644); err != nil {
		t.Fatal(err)
	}

	content, err := svc.ReadConfigFile("game_cfg", "autoexec.cfg")
	if err != nil {
		t.Fatalf("ReadConfigFile failed: %v", err)
	}
	if content != testContent {
		t.Errorf("content mismatch: got %q, want %q", content, testContent)
	}
}

func TestReadConfigFile_SecurityTraversals(t *testing.T) {
	svc, _, _ := setupTestBackupService(t)

	badCases := []struct {
		name      string
		accountID string
		relPath   string
	}{
		{"dotdot prefix slash", "123456", "../secret.txt"},
		{"dotdot prefix backslash", "123456", `..\secret.txt`},
		{"nested traversal", "123456", `sub/../../secret.txt`},
		{"absolute unix path", "123456", "/etc/passwd"},
		{"absolute win path", "123456", `\windows\win.ini`},
		{"win drive path", "123456", `C:\Windows\System32\cmd.exe`},
		{"win drive rel path", "123456", `C:secret.txt`},
		{"current dir dot", "123456", "."},
		{"parent dir dotdot", "123456", ".."},
		{"empty relPath", "123456", ""},
		{"null byte relPath", "123456", "file\x00.cfg"},
		{"empty accountID", "", "cs2_video.txt"},
		{"accountID traversal unix", "../123456", "cs2_video.txt"},
		{"accountID traversal win", `..\123456`, "cs2_video.txt"},
		{"accountID slash", "123/456", "cs2_video.txt"},
		{"accountID backslash", `123\456`, "cs2_video.txt"},
		{"accountID colon", "C:123456", "cs2_video.txt"},
	}

	for _, tc := range badCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.ReadConfigFile(tc.accountID, tc.relPath)
			if err == nil {
				t.Fatalf("expected error for path traversal attack (%s, %s), got nil", tc.accountID, tc.relPath)
			}
		})
	}
}

func TestReadConfigFile_NotFoundAndDirectory(t *testing.T) {
	svc, userCfgDir, _ := setupTestBackupService(t)

	// File not found
	_, err := svc.ReadConfigFile("123456", "non_existent.cfg")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}

	// Directory target
	subDir := filepath.Join(userCfgDir, "some_dir")
	_ = os.MkdirAll(subDir, 0755)
	_, err = svc.ReadConfigFile("123456", "some_dir")
	if err == nil {
		t.Fatal("expected error when reading a directory, got nil")
	}
}

func TestSaveConfigFileText_UserConfig_Success(t *testing.T) {
	svc, userCfgDir, _ := setupTestBackupService(t)

	newContent := "bind \"mouse1\" \"+attack\"\nbind \"space\" \"+jump\"\n"
	err := svc.SaveConfigFileText("123456", "test_binds.cfg", newContent)
	if err != nil {
		t.Fatalf("SaveConfigFileText failed: %v", err)
	}

	targetFile := filepath.Join(userCfgDir, "test_binds.cfg")
	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read saved file on disk: %v", err)
	}
	if string(data) != newContent {
		t.Errorf("saved content mismatch: got %q, want %q", string(data), newContent)
	}
}

func TestSaveConfigFileText_GameCFG_Success(t *testing.T) {
	svc, _, gameCfgDir := setupTestBackupService(t)

	newContent := "sv_cheats 1\nmp_warmup_end\n"
	err := svc.SaveConfigFileText("game_cfg", "practice.cfg", newContent)
	if err != nil {
		t.Fatalf("SaveConfigFileText game_cfg failed: %v", err)
	}

	targetFile := filepath.Join(gameCfgDir, "practice.cfg")
	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read saved game cfg file: %v", err)
	}
	if string(data) != newContent {
		t.Errorf("saved content mismatch: got %q, want %q", string(data), newContent)
	}
}

func TestSaveConfigFileText_SubdirectoryCreation(t *testing.T) {
	svc, userCfgDir, _ := setupTestBackupService(t)

	content := "echo \"Nested Config\"\n"
	err := svc.SaveConfigFileText("123456", "sub/nested/deep.cfg", content)
	if err != nil {
		t.Fatalf("SaveConfigFileText with subdirs failed: %v", err)
	}

	targetFile := filepath.Join(userCfgDir, "sub", "nested", "deep.cfg")
	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read nested file: %v", err)
	}
	if string(data) != content {
		t.Errorf("content mismatch: got %q, want %q", string(data), content)
	}
}

func TestSaveConfigFileText_ReadOnlyOverwrite(t *testing.T) {
	svc, userCfgDir, _ := setupTestBackupService(t)

	targetFile := filepath.Join(userCfgDir, "readonly.cfg")
	if err := os.WriteFile(targetFile, []byte("old content"), 0444); err != nil {
		t.Fatal(err)
	}

	newContent := "new overwritten content"
	err := svc.SaveConfigFileText("123456", "readonly.cfg", newContent)
	if err != nil {
		t.Fatalf("SaveConfigFileText failed on readonly file: %v", err)
	}

	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != newContent {
		t.Errorf("content mismatch: got %q, want %q", string(data), newContent)
	}
}

func TestSaveConfigFileText_SecurityTraversals(t *testing.T) {
	svc, _, _ := setupTestBackupService(t)

	badCases := []struct {
		name      string
		accountID string
		relPath   string
	}{
		{"dotdot prefix slash", "123456", "../hacked.cfg"},
		{"dotdot prefix backslash", "123456", `..\hacked.cfg`},
		{"nested traversal", "123456", `sub/../../hacked.cfg`},
		{"absolute win path", "123456", `\hacked.cfg`},
		{"win drive path", "123456", `C:\Windows\hacked.cfg`},
		{"empty relPath", "123456", ""},
		{"empty accountID", "", "test.cfg"},
		{"malicious accountID", "../evil", "test.cfg"},
	}

	for _, tc := range badCases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.SaveConfigFileText(tc.accountID, tc.relPath, "dangerous payload")
			if err == nil {
				t.Fatalf("expected error for save traversal (%s, %s), got nil", tc.accountID, tc.relPath)
			}
		})
	}
}

func TestConfigEditor_UnsetSteamPath(t *testing.T) {
	tempDir := testutil.TempDir(t)
	svc, _ := NewService(tempDir, nil)
	// steamPath is not set

	if _, err := svc.ReadConfigFile("123456", "cs2_video.txt"); err == nil {
		t.Error("expected error when steamPath not set, got nil")
	}
	if err := svc.SaveConfigFileText("123456", "cs2_video.txt", "content"); err == nil {
		t.Error("expected error when steamPath not set, got nil")
	}
}

func TestNormalizeNewCFGName(t *testing.T) {
	cases := []struct {
		raw     string
		want    string
		wantErr bool
		why     string
	}{
		{"practice", "practice.cfg", false, "没写后缀自动补"},
		{"practice.cfg", "practice.cfg", false, "写了后缀不重复补"},
		{"Practice.CFG", "Practice.cfg", false, "后缀大小写不敏感，主名保留原样"},
		{"  surf  ", "surf.cfg", false, "首尾空格去掉"},
		{"nade line", "nade line.cfg", false, "中间空格是合法的"},

		{"", "", true, "空"},
		{"   ", "", true, "全空白"},
		{"a/b", "", true, "斜杠"},
		{`a\b`, "", true, "反斜杠"},
		{"a:b", "", true, "盘符/冒号"},
		{"a?b", "", true, "Windows 保留字符"},
		{"practice.txt", "", true, "只接受 .cfg，别拼成 practice.txt.cfg"},
		{".cfg", "", true, "只有后缀"},
		{"trail.", "", true, "结尾是点：用户看得见的字符，报错比悄悄截掉好"},
		// 行尾空格会被 trim 掉而不是报错——用户看不见它，去掉才是他要的结果
		{"trail ", "trail.cfg", false, "行尾空格静默去掉"},
		{"CON", "", true, "Windows 设备名"},
		{"con.cfg", "", true, "设备名带后缀也不行"},
		{"NUL", "", true, "设备名"},
		{"COM1", "", true, "设备名"},
		{"com9.cfg", "", true, "设备名带后缀"},
		{"a\x01b", "", true, "控制字符"},
	}

	for _, c := range cases {
		got, err := NormalizeNewCFGName(c.raw)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: 期望报错，实际返回 %q", c.why, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: 期望成功，实际报错 %v", c.why, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.why, got, c.want)
		}
	}
}

func TestCreateConfigFile_UserConfig(t *testing.T) {
	svc, userCfgDir, _ := setupTestBackupService(t)

	name, err := svc.CreateConfigFile("123456", "practice")
	if err != nil {
		t.Fatalf("CreateConfigFile failed: %v", err)
	}
	if name != "practice.cfg" {
		t.Errorf("got %q, want practice.cfg", name)
	}

	info, err := os.Stat(filepath.Join(userCfgDir, "practice.cfg"))
	if err != nil {
		t.Fatalf("文件没建出来: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("新建的文件应该是空的，实际 %d 字节", info.Size())
	}
}

func TestCreateConfigFile_GameCFG(t *testing.T) {
	svc, _, gameCfgDir := setupTestBackupService(t)

	if _, err := svc.CreateConfigFile("game_cfg", "practice.cfg"); err != nil {
		t.Fatalf("CreateConfigFile failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(gameCfgDir, "practice.cfg")); err != nil {
		t.Fatalf("通用 cfg 目录里没建出来: %v", err)
	}
}

// 这条是这个功能的核心：新建**绝不能**覆盖已有文件。
// 用户最可能输的就是 autoexec，一旦被清空，工具的前提就破了。
func TestCreateConfigFile_RefusesToOverwrite(t *testing.T) {
	svc, userCfgDir, _ := setupTestBackupService(t)

	original := "bind MOUSE1 +attack\n"
	target := filepath.Join(userCfgDir, "autoexec.cfg")
	if err := os.WriteFile(target, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.CreateConfigFile("123456", "autoexec"); err == nil {
		t.Fatal("同名文件已存在，应该报错")
	}

	// 内容必须原封不动
	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != original {
		t.Errorf("原文件被改动了！got %q", string(after))
	}
}

// Windows 文件系统不区分大小写，Autoexec.cfg 和 autoexec.cfg 是同一个文件。
func TestCreateConfigFile_CaseInsensitiveCollision(t *testing.T) {
	svc, userCfgDir, _ := setupTestBackupService(t)

	original := "sensitivity 2\n"
	if err := os.WriteFile(filepath.Join(userCfgDir, "Autoexec.cfg"), []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.CreateConfigFile("123456", "autoexec.cfg"); err == nil {
		t.Fatal("只差大小写也算重名，应该报错")
	}
	after, _ := os.ReadFile(filepath.Join(userCfgDir, "Autoexec.cfg"))
	if string(after) != original {
		t.Errorf("原文件被改动了！got %q", string(after))
	}
}
