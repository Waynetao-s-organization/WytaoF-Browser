package backend

import (
	internalbrowser "ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/database"
	"os"
	"path/filepath"
	"testing"
)

func newCoreScanTestApp(t *testing.T, coreDirs ...string) *App {
	t.Helper()

	appRoot := t.TempDir()
	for _, dir := range coreDirs {
		exePath := filepath.Join(appRoot, "chrome", dir, filepath.FromSlash(internalbrowser.CoreExecutableCandidates()[0]))
		if err := os.MkdirAll(filepath.Dir(exePath), 0o755); err != nil {
			t.Fatalf("创建内核目录失败: %v", err)
		}
		if err := os.WriteFile(exePath, []byte("stub"), 0o755); err != nil {
			t.Fatalf("写入内核可执行文件失败: %v", err)
		}
	}

	if err := os.MkdirAll(filepath.Join(appRoot, "data"), 0o755); err != nil {
		t.Fatalf("创建 data 目录失败: %v", err)
	}
	db, err := database.NewDB(filepath.Join(appRoot, "data", "app.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("测试数据库迁移失败: %v", err)
	}

	cfg := config.DefaultConfig()
	app := NewApp(appRoot)
	app.config = cfg
	app.browserMgr = internalbrowser.NewManager(cfg, appRoot)
	app.browserMgr.CoreDAO = internalbrowser.NewSQLiteCoreDAO(db.GetConn())
	return app
}

func findCoreByID(cores []BrowserCore, coreID string) (BrowserCore, bool) {
	for _, core := range cores {
		if core.CoreId == coreID {
			return core, true
		}
	}
	return BrowserCore{}, false
}

func TestRegisterDetectedCoresAddsNewDirectoryWithoutChangingDefault(t *testing.T) {
	t.Parallel()

	app := newCoreScanTestApp(t, "Chrom-144", "Chrom-150")
	if err := app.browserMgr.CoreDAO.Upsert(BrowserCore{CoreId: "core-Chrom-144", CoreName: "Chrome Chrom-144", CorePath: `chrome\Chrom-144`, IsDefault: true}); err != nil {
		t.Fatalf("预置内核失败: %v", err)
	}

	if got := app.registerDetectedCores(false); got != 1 {
		t.Fatalf("应当只登记 1 个新内核，实际 %d", got)
	}
	cores := app.browserMgr.ListCores()
	if len(cores) != 2 {
		t.Fatalf("内核表应当有 2 条，实际 %+v", cores)
	}
	added, ok := findCoreByID(cores, "core-Chrom-150")
	if !ok || added.IsDefault {
		t.Fatalf("新内核应当登记为非默认，实际 %+v", cores)
	}
	if existing, ok := findCoreByID(cores, "core-Chrom-144"); !ok || !existing.IsDefault {
		t.Fatalf("原来的默认内核不能被改动，实际 %+v", cores)
	}

	if got := app.registerDetectedCores(false); got != 0 {
		t.Fatalf("重复扫描不应再登记，实际 %d", got)
	}
}

func TestRegisterDetectedCoresMarksFirstCoreDefaultWhenTableEmpty(t *testing.T) {
	t.Parallel()

	app := newCoreScanTestApp(t, "Chrom-150")
	if got := app.registerDetectedCores(false); got != 1 {
		t.Fatalf("应当登记 1 个内核，实际 %d", got)
	}
	cores := app.browserMgr.ListCores()
	if len(cores) != 1 || !cores[0].IsDefault {
		t.Fatalf("内核表为空时第一个登记的内核应当是默认内核，实际 %+v", cores)
	}
}

func TestDeletedCoreStaysDeletedUntilManualScan(t *testing.T) {
	t.Parallel()

	app := newCoreScanTestApp(t, "Chrom-144", "Chrom-150")
	if got := app.registerDetectedCores(false); got != 2 {
		t.Fatalf("应当登记 2 个内核，实际 %d", got)
	}

	if err := app.BrowserCoreDelete("core-Chrom-150"); err != nil {
		t.Fatalf("删除内核失败: %v", err)
	}
	if got := app.registerDetectedCores(false); got != 0 {
		t.Fatalf("用户删掉的内核不应在启动扫描时自己回来，实际登记 %d", got)
	}
	saved, err := config.Load(filepath.Join(app.appRoot, "config.yaml"))
	if err != nil {
		t.Fatalf("读取 config.yaml 失败: %v", err)
	}
	if len(saved.Browser.IgnoredCorePaths) != 1 {
		t.Fatalf("删除记录应当写进 config.yaml，实际 %+v", saved.Browser.IgnoredCorePaths)
	}

	cores := app.BrowserCoreScan()
	if _, ok := findCoreByID(cores, "core-Chrom-150"); !ok {
		t.Fatalf("手动点扫描内核应当把目录重新登记回来，实际 %+v", cores)
	}
	if len(app.config.Browser.IgnoredCorePaths) != 0 {
		t.Fatalf("重新登记后删除记录应当清掉，实际 %+v", app.config.Browser.IgnoredCorePaths)
	}
}

func TestRegisterDetectedCoresKeepsCoreIDUnique(t *testing.T) {
	t.Parallel()

	app := newCoreScanTestApp(t, "Chrom-150")
	if err := app.browserMgr.CoreDAO.Upsert(BrowserCore{CoreId: "core-Chrom-150", CoreName: "别处的 150", CorePath: filepath.Join(t.TempDir(), "elsewhere"), IsDefault: true}); err != nil {
		t.Fatalf("预置内核失败: %v", err)
	}

	if got := app.registerDetectedCores(false); got != 1 {
		t.Fatalf("应当登记 1 个新内核，实际 %d", got)
	}
	cores := app.browserMgr.ListCores()
	if _, ok := findCoreByID(cores, "core-Chrom-150-2"); !ok {
		t.Fatalf("内核 ID 撞车时应当换一个不重复的，实际 %+v", cores)
	}
	if existing, _ := findCoreByID(cores, "core-Chrom-150"); existing.CoreName != "别处的 150" {
		t.Fatalf("已有的同名 ID 内核不能被覆盖，实际 %+v", cores)
	}
}
