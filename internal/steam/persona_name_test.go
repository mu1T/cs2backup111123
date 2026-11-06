package steam

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"cs2backup/internal/testutil"
)

// countingClient 记录被调用次数，用来断言「有没有去网上补名字」。
type countingClient struct {
	calls int32
}

func (c *countingClient) Do(req *http.Request) (*http.Response, error) {
	atomic.AddInt32(&c.calls, 1)
	return nil, fmt.Errorf("network disabled in test")
}

func (c *countingClient) count() int { return int(atomic.LoadInt32(&c.calls)) }

const sampleLocalConfig = `"UserLocalConfigStore"
{
	"friends"
	{
		"avatar"		"e577b32c660abc3c4fc96221fcad81c490155336"
		"PersonaName"		"風の便り"
	}
	"WebStorage"
	{
		"PersonaName"		"不该取到这个"
	}
}
`

func TestParsePersonaName(t *testing.T) {
	got, err := ParsePersonaName(strings.NewReader(sampleLocalConfig))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	// 取第一个出现的 PersonaName（friends 段里那个才是真的）
	if got != "風の便り" {
		t.Errorf("PersonaName = %q, 期望 %q", got, "風の便り")
	}
}

func TestParsePersonaNameMissingKey(t *testing.T) {
	got, err := ParsePersonaName(strings.NewReader(`"root" { "a" "b" }`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if got != "" {
		t.Errorf("没有该键时应返回空串，实际 %q", got)
	}
}

func TestReadLocalPersonaName(t *testing.T) {
	steamDir := testutil.TempDir(t)
	cfgDir := filepath.Join(steamDir, "userdata", "1199192343", "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "localconfig.vdf"), []byte(sampleLocalConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := ReadLocalPersonaName(steamDir, "1199192343"); got != "風の便り" {
		t.Errorf("读到 %q, 期望 %q", got, "風の便り")
	}
	// 没有该账号的目录时返回空串，不报错
	if got := ReadLocalPersonaName(steamDir, "999"); got != "" {
		t.Errorf("不存在的账号应返回空串，实际 %q", got)
	}
}

// TestCacheRejectsAccountIDAsUsername 是本轮 bug 的护栏：
// 旧版本把账号 ID 当成昵称写进了缓存，界面于是一直显示一串数字。
func TestCacheRejectsAccountIDAsUsername(t *testing.T) {
	cm, err := NewCacheManager(testutil.TempDir(t))
	if err != nil {
		t.Fatalf("构造缓存失败: %v", err)
	}

	if err := cm.SetUsername("1092783898", "1092783898"); err != nil {
		t.Fatalf("SetUsername 不应报错: %v", err)
	}
	if _, ok := cm.GetUsername("1092783898"); ok {
		t.Error("账号 ID 不应被当成昵称缓存下来")
	}

	// 正常昵称照常写入
	if err := cm.SetUsername("1092783898", "akuGapapaWin"); err != nil {
		t.Fatal(err)
	}
	if got, ok := cm.GetUsername("1092783898"); !ok || got != "akuGapapaWin" {
		t.Errorf("GetUsername = %q/%v, 期望 akuGapapaWin/true", got, ok)
	}
}

// TestCacheTreatsPoisonedEntryAsMissing 覆盖「历史脏数据已经在文件里」的情况。
func TestCacheTreatsPoisonedEntryAsMissing(t *testing.T) {
	dir := testutil.TempDir(t)
	poisoned := map[string]CachedUser{
		"1199192343": {Username: "1199192343", UpdatedAt: "2026-11-04T21:41:26"},
		"987654321": {Username: "骂我死全家", UpdatedAt: "2026-11-06T12:51:14"},
	}
	raw, _ := json.MarshalIndent(poisoned, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "user_cache.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	cm, err := NewCacheManager(dir)
	if err != nil {
		t.Fatalf("构造缓存失败: %v", err)
	}

	if _, ok := cm.GetUsername("1199192343"); ok {
		t.Error("被污染的条目应被视为无效")
	}
	if got, ok := cm.GetUsername("987654321"); !ok || got != "骂我死全家" {
		t.Errorf("正常条目不应被误删，实际 %q/%v", got, ok)
	}

	// 脏数据应当已经落盘清掉
	data, err := os.ReadFile(filepath.Join(dir, "user_cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"1199192343"`) {
		t.Errorf("脏条目应已从文件里删除：%s", data)
	}
}

// buildFakeSteam 造一棵最小可用的 Steam 目录：userdata + loginusers + localconfig。
func buildFakeSteam(t *testing.T, localConfigFor map[string]string) (steamDir string) {
	t.Helper()
	steamDir = testutil.TempDir(t)

	for aid, local := range localConfigFor {
		dir := filepath.Join(steamDir, "userdata", aid)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if local != "" {
			cfgDir := filepath.Join(dir, "config")
			if err := os.MkdirAll(cfgDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(cfgDir, "localconfig.vdf"), []byte(local), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	if err := os.MkdirAll(filepath.Join(steamDir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	// loginusers 里刻意不放这两个账号，模拟「没在本机登录过、只在 userdata 里留了目录」
	if err := os.WriteFile(filepath.Join(steamDir, "config", "loginusers.vdf"), []byte(`"users"
{
	"76561198083722517"
	{
		"AccountName"	"test_account_one"
		"PersonaName"	"TestPersonaOne"
		"MostRecent"	"1"
	}
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	return steamDir
}

// TestUserCardPrefersLocalConfigOverPoisonedCache 是本轮 bug 的正向回归：
// 缓存里是账号 ID、loginusers 里没有，但 localconfig.vdf 有真名 → 必须显示真名。
func TestUserCardPrefersLocalConfigOverPoisonedCache(t *testing.T) {
	steamDir := buildFakeSteam(t, map[string]string{
		"1199192343": sampleLocalConfig,
	})

	configDir := testutil.TempDir(t)
	poisoned := map[string]CachedUser{
		"1199192343": {Username: "1199192343", UpdatedAt: "2026-11-04T21:41:26"},
	}
	raw, _ := json.MarshalIndent(poisoned, "", "  ")
	if err := os.WriteFile(filepath.Join(configDir, "user_cache.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	svc, err := NewService(configDir, &countingClient{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.WaitBackground()
	if err := svc.SetSteamPath(steamDir); err != nil {
		t.Fatal(err)
	}

	cards, err := svc.GetUsers()
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	var found *UserCardInfo
	for i := range cards {
		if cards[i].AccountID == "1199192343" {
			found = &cards[i]
		}
	}
	if found == nil {
		t.Fatal("没找到 1199192343 的卡片")
	}
	if found.Username != "風の便り" {
		t.Errorf("昵称 = %q, 期望从 localconfig 读到的 風の便り", found.Username)
	}
}

// TestFetchTriggeredWhenAvatarCachedButNameMissing 守住「头像有缓存就不抓名字」这个老问题。
func TestFetchTriggeredWhenAvatarCachedButNameMissing(t *testing.T) {
	// localconfig 里没有 PersonaName，loginusers 里也没有 → 只能靠网络补
	steamDir := buildFakeSteam(t, map[string]string{
		"1199192343": `"UserLocalConfigStore" { "friends" { "avatar" "abc" } }`,
	})

	configDir := testutil.TempDir(t)
	// 预置头像缓存，模拟「头像早就抓过了」
	avatarDir := filepath.Join(configDir, "avatarcache")
	if err := os.MkdirAll(avatarDir, 0o755); err != nil {
		t.Fatal(err)
	}
	steam64 := AccountIDToSteam64(1199192343)
	if err := os.WriteFile(filepath.Join(avatarDir, fmt.Sprintf("%d.png", steam64)), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}

	client := &countingClient{}
	svc, err := NewService(configDir, client)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.WaitBackground()
	if err := svc.SetSteamPath(steamDir); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.GetUsers(); err != nil {
		t.Fatalf("扫描失败: %v", err)
	}
	svc.WaitBackground()

	if client.count() == 0 {
		t.Error("头像已缓存但昵称缺失时，仍应发起一次资料抓取")
	}
}

// TestXMLNameBeatsLocalVDF 覆盖用户要求的优先级：接口给的昵称压过本地文件。
func TestXMLNameBeatsLocalVDF(t *testing.T) {
	// localconfig 里是旧名字
	steamDir := buildFakeSteam(t, map[string]string{
		"1199192343": sampleLocalConfig, // PersonaName = 風の便り
	})

	configDir := testutil.TempDir(t)
	seedUserCache(t, configDir, map[string]CachedUser{
		"1199192343": {
			Username:  "改过的名字",
			UpdatedAt: time.Now().Format("2006-01-02T15:04:05.000000"),
			Source:    UsernameSourceXML,
		},
	})

	svc, err := NewService(configDir, &countingClient{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.WaitBackground()
	if err := svc.SetSteamPath(steamDir); err != nil {
		t.Fatal(err)
	}

	cards, err := svc.GetUsers()
	if err != nil {
		t.Fatal(err)
	}
	if got := cardName(cards, "1199192343"); got != "改过的名字" {
		t.Errorf("昵称 = %q, 期望接口给的「改过的名字」", got)
	}
}

// TestNoFetchWhenXMLNameIsFresh 反向确认：接口版本还新鲜时不该再打网络。
func TestNoFetchWhenXMLNameIsFresh(t *testing.T) {
	steamDir := buildFakeSteam(t, map[string]string{
		"1199192343": sampleLocalConfig,
	})

	configDir := testutil.TempDir(t)
	seedAvatar(t, configDir, 1199192343)
	seedUserCache(t, configDir, map[string]CachedUser{
		"1199192343": {
			Username:  "風の便り",
			UpdatedAt: time.Now().Format("2006-01-02T15:04:05.000000"),
			Source:    UsernameSourceXML,
		},
	})

	client := &countingClient{}
	svc, err := NewService(configDir, client)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.WaitBackground()
	if err := svc.SetSteamPath(steamDir); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.GetUsers(); err != nil {
		t.Fatal(err)
	}
	svc.WaitBackground()

	if n := client.count(); n != 0 {
		t.Errorf("接口版本还新鲜时不该再请求网络，实际 %d 次", n)
	}
}

// TestStaleXMLNameTriggersRefresh 昵称超过一天就再去核对一次。
func TestStaleXMLNameTriggersRefresh(t *testing.T) {
	steamDir := buildFakeSteam(t, map[string]string{
		"1199192343": sampleLocalConfig,
	})

	configDir := testutil.TempDir(t)
	seedAvatar(t, configDir, 1199192343)
	seedUserCache(t, configDir, map[string]CachedUser{
		"1199192343": {
			Username:  "風の便り",
			UpdatedAt: time.Now().Add(-48 * time.Hour).Format("2006-01-02T15:04:05.000000"),
			Source:    UsernameSourceXML,
		},
	})

	client := &countingClient{}
	svc, err := NewService(configDir, client)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.WaitBackground()
	if err := svc.SetSteamPath(steamDir); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.GetUsers(); err != nil {
		t.Fatal(err)
	}
	svc.WaitBackground()

	if client.count() == 0 {
		t.Error("昵称超过一天应触发一次核对")
	}
}

func TestNeedsUsernameRefresh(t *testing.T) {
	fresh := time.Now().Format("2006-01-02T15:04:05.000000")
	stale := time.Now().Add(-30 * time.Hour).Format("2006-01-02T15:04:05.000000")

	cases := []struct {
		name  string
		entry CachedUser
		ok    bool
		want  bool
	}{
		{"没有缓存", CachedUser{}, false, true},
		{"接口版本且新鲜", CachedUser{Username: "a", UpdatedAt: fresh, Source: UsernameSourceXML}, true, false},
		{"接口版本但过期", CachedUser{Username: "a", UpdatedAt: stale, Source: UsernameSourceXML}, true, true},
		{"查过但对方没资料且新鲜", CachedUser{UpdatedAt: fresh, Source: UsernameSourceNone}, true, false},
		{"查过但对方没资料且过期", CachedUser{UpdatedAt: stale, Source: UsernameSourceNone}, true, true},
		{"本地来源", CachedUser{Username: "a", UpdatedAt: fresh, Source: ""}, true, true},
		{"时间戳坏了", CachedUser{Username: "a", UpdatedAt: "not-a-time", Source: UsernameSourceXML}, true, true},
	}
	for _, tc := range cases {
		if got := needsUsernameRefresh(tc.entry, tc.ok); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestEmptyXMLNameIsRemembered(t *testing.T) {
	cm, err := NewCacheManager(testutil.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	// 「查过，对方没设置社区资料」要落盘一个 none 标记，否则每次启动都去问
	if err := cm.SetUsernameFrom("1092783898", "", UsernameSourceNone); err != nil {
		t.Fatal(err)
	}
	entry, ok := cm.GetUsernameEntry("1092783898")
	if !ok || entry.Source != UsernameSourceNone {
		t.Fatalf("应记下 none 标记，实际 %+v / %v", entry, ok)
	}
	if entry.Username != "" {
		t.Errorf("昵称应为空，实际 %q", entry.Username)
	}
	// 普通来源写空值应当被忽略
	if err := cm.SetUsernameFrom("1199192343", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, ok := cm.GetUsernameEntry("1199192343"); ok {
		t.Error("普通来源的空昵称不该落盘")
	}
}

// ---- 测试辅助 ----

func cardName(cards []UserCardInfo, accountID string) string {
	for _, c := range cards {
		if c.AccountID == accountID {
			return c.Username
		}
	}
	return ""
}

func seedAvatar(t *testing.T, configDir string, accountID uint32) {
	t.Helper()
	dir := filepath.Join(configDir, "avatarcache")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	steam64 := AccountIDToSteam64(accountID)
	if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d.png", steam64)), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func seedUserCache(t *testing.T, configDir string, users map[string]CachedUser) {
	t.Helper()
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "user_cache.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}
