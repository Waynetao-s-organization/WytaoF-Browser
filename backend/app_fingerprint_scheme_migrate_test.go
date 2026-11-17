package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFingerprintSchemeMigrationBindsLegacyProfiles(t *testing.T) {
	appRoot := t.TempDir()
	_ = os.MkdirAll(filepath.Join(appRoot, "data"), 0o755)
	db, err := database.NewDB(filepath.Join(appRoot, "data", "app.db"))
	if err != nil {
		t.Fatalf("建库失败: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.Browser.Cores = []config.BrowserCore{{CoreId: "core-Chrom-139", CoreName: "139", CorePath: "chrome/Chrom-139", IsDefault: true}, {CoreId: "core-Chrom-144", CoreName: "144", CorePath: "chrome/Chrom-144"}}
	app := NewApp(appRoot)
	app.config = cfg
	app.browserMgr = browser.NewManager(cfg, appRoot)
	conn := db.GetConn()
	app.browserMgr.ProfileDAO = browser.NewSQLiteProfileDAO(conn)
	app.browserMgr.CoreDAO = browser.NewSQLiteCoreDAO(conn)
	app.browserMgr.SchemeDAO = browser.NewSQLiteFingerprintSchemeDAO(conn)
	for _, c := range cfg.Browser.Cores {
		_ = app.browserMgr.SaveCore(browser.CoreInput{CoreId: c.CoreId, CoreName: c.CoreName, CorePath: c.CorePath, IsDefault: c.IsDefault})
	}
	laptop := &browser.Profile{ProfileId: "laptop-1", ProfileName: "样例1", UserDataDir: "laptop-1", CoreId: "core-Chrom-139", FingerprintArgs: []string{"--lang=en-US", "--timezone=America/Los_Angeles"}}
	desktop := &browser.Profile{ProfileId: "desktop-1", ProfileName: "样例2", UserDataDir: "desktop-1", CoreId: "core-Chrom-144", FingerprintArgs: []string{"--fingerprint=2031806230", "--fingerprint-brand=Chrome", "--fingerprint-platform=windows", "--lang=en-US", "--timezone=America/New_York", "--window-size=1920,1080", "--fingerprint-color-depth=24", "--fingerprint-hardware-concurrency=8", "--fingerprint-device-memory=8", "--fingerprint-canvas-noise=true", "--fingerprint-audio-noise=true", "--fingerprint-do-not-track=true", "--webrtc-ip-handling-policy=disable_non_proxied_udp"}}
	for _, p := range []*browser.Profile{laptop, desktop} {
		if err := app.browserMgr.ProfileDAO.Upsert(p); err != nil {
			t.Fatalf("写入样例实例失败: %v", err)
		}
	}
	app.browserMgr.InitData()
	app.browserMgr.EnsureBuiltinSchemes()
	app.migrateLegacyFingerprintSchemes()

	if !app.config.Browser.FingerprintSchemeMigrated {
		t.Fatalf("迁移标记未写入")
	}
	entries, _ := os.ReadDir(filepath.Join(appRoot, "data", "backups"))
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "fpscheme-") {
		t.Fatalf("迁移前应备份数据库，实际 %v", entries)
	}
	reloaded, err := app.browserMgr.ProfileDAO.List()
	if err != nil || len(reloaded) != 2 {
		t.Fatalf("重新读取实例失败: %v %d", err, len(reloaded))
	}
	for _, p := range reloaded {
		joined := " " + strings.Join(p.FingerprintArgs, " ") + " "
		if p.FingerprintSchemeId != "builtin-us-west-desktop" {
			t.Fatalf("%s 未绑定默认方案: %q", p.ProfileId, p.FingerprintSchemeId)
		}
		if p.CoreId != "core-Chrom-144" {
			t.Fatalf("%s 内核应随方案改为 144，实际 %s", p.ProfileId, p.CoreId)
		}
		for _, want := range []string{" --timezone=America/Los_Angeles ", " --fingerprint-brand=Chrome ", " --fingerprint-platform=windows ", " --disable-speech-api ", " --disable-features=WebGPUService ", " --start-maximized "} {
			if !strings.Contains(joined, want) {
				t.Fatalf("%s 缺少 %s：%v", p.ProfileId, want, p.FingerprintArgs)
			}
		}
		if strings.Contains(joined, "America/New_York") || strings.Contains(joined, "--fingerprint-color-depth") || strings.Contains(joined, "--window-size=") {
			t.Fatalf("%s 仍残留旧参数：%v", p.ProfileId, p.FingerprintArgs)
		}
	}
	byID := map[string]*browser.Profile{}
	for _, p := range reloaded {
		byID[p.ProfileId] = p
	}
	if browser.ExtractFingerprintSeed(byID["desktop-1"].FingerprintArgs) != "2031806230" {
		t.Fatalf("已有种子必须保留")
	}
	if browser.ExtractFingerprintSeed(byID["laptop-1"].FingerprintArgs) != browser.LegacyFallbackSeed("laptop-1") {
		t.Fatalf("无种子实例应固化旧版回退种子，实际 %s", browser.ExtractFingerprintSeed(byID["laptop-1"].FingerprintArgs))
	}
	if !strings.Contains(" "+strings.Join(byID["desktop-1"].FingerprintArgs, " ")+" ", " --fingerprint-hardware-concurrency=8 ") {
		t.Fatalf("已有核心数必须保留")
	}
	// 第二次启动不再重复迁移
	updated, _ := app.browserMgr.MigrateLegacyProfilesToDefaultScheme()
	if updated != 0 {
		t.Fatalf("重复迁移不应再改动实例，实际改了 %d 个", updated)
	}
	// 新建实例：不指定方案时自动套默认方案并随机设备
	created, err := app.browserMgr.Create(browser.ProfileInput{ProfileName: "样例3"})
	if err != nil {
		t.Fatalf("创建实例失败: %v", err)
	}
	if created.FingerprintSchemeId != "builtin-us-west-desktop" || browser.ExtractFingerprintSeed(created.FingerprintArgs) == "" || browser.ExtractFingerprintSeed(created.FingerprintArgs) == "2031806230" {
		t.Fatalf("新实例应绑定默认方案并拿到新种子: %+v", created.FingerprintArgs)
	}
}
func TestSchemeLaunchHelpersDoNotDeadlockUnderManagerLock(t *testing.T) {
	cfg := config.DefaultConfig()
	mgr := browser.NewManager(cfg, t.TempDir())
	mgr.InitData()
	profile := &browser.Profile{ProfileId: "lock-1", ProfileName: "样例", FingerprintArgs: []string{"--fingerprint=123456"}, FingerprintSchemeId: "builtin-us-west-desktop"}
	mgr.Profiles[profile.ProfileId] = profile
	done := make(chan struct{})
	go func() {
		// 启动流程 browserInstanceStartWithInput 就是持着这把锁调用准备逻辑的
		mgr.Mutex.Lock()
		defer mgr.Mutex.Unlock()
		_ = mgr.EffectiveFingerprintArgsLocked(profile)
		_ = mgr.ResolveProfileCoreIDLocked(profile)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("持有 Manager.Mutex 时调用启动期方案助手发生死锁")
	}
}
