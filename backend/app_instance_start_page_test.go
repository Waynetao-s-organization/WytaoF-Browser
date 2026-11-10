package backend

import (
	internalbrowser "ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newStartPageTestApp(t *testing.T) *App {
	t.Helper()

	appRoot := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.App.Name = "WytaoF Browser"
	cfg.Browser.RestoreLastSession = false
	cfg.Browser.DefaultStartURLs = []string{}
	cfg.Browser.Proxies = []config.BrowserProxy{
		{ProxyId: "__direct__", ProxyName: "直连（不走代理）", ProxyConfig: "direct://"},
		{ProxyId: "proxy-1", ProxyName: "家宽 1 号", ProxyConfig: "socks5://someone:secret-pass@203.0.113.10:1080"},
		{ProxyId: "proxy-2", ProxyName: "节点 2 号", ProxyConfig: "vless://11111111-2222-3333-4444-555555555555@node.example.com:443?security=reality"},
	}
	app := NewApp(appRoot, "9.9.9")
	app.config = cfg
	app.browserMgr = internalbrowser.NewManager(cfg, appRoot)
	return app
}

func TestShouldOpenStartPageOnlyForManualDefaultLaunch(t *testing.T) {
	t.Parallel()

	manual := newBrowserStartInput("p", nil, nil, false, false, false, "", "")
	manual.ShowStartPage = true
	if !shouldOpenStartPage(manual, true) {
		t.Fatal("界面上手动启动应当打开环境检测页")
	}
	if shouldOpenStartPage(manual, false) {
		t.Fatal("设置里关掉以后不应打开")
	}

	fromAPI := newBrowserStartInput("p", nil, nil, false, false, false, "", "")
	if shouldOpenStartPage(fromAPI, true) {
		t.Fatal("接口启动不应打开环境检测页")
	}

	withURLs := newBrowserStartInput("p", nil, []string{"https://example.com/"}, false, true, false, "", "")
	withURLs.ShowStartPage = true
	if shouldOpenStartPage(withURLs, true) {
		t.Fatal("指定了启动页面时不应再插入环境检测页")
	}

	skipDefaults := newBrowserStartInput("p", nil, nil, true, false, false, "", "")
	skipDefaults.ShowStartPage = true
	if shouldOpenStartPage(skipDefaults, true) {
		t.Fatal("跳过默认启动页时不应打开环境检测页")
	}
}

func TestDescribeProxyConfigNeverLeaksCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		config   string
		wantType string
		wantAddr string
	}{
		{config: "socks5://127.0.0.1:7890", wantType: "SOCKS5", wantAddr: "127.0.0.1:7890"},
		{config: "http://user:p%40ss@203.0.113.10:8080", wantType: "HTTP", wantAddr: "203.0.113.10:8080"},
		{config: "vless://uuid-secret@node.example.com:443?security=reality&pbk=key", wantType: "VLESS", wantAddr: ""},
		{config: "vmess://eyJhZGQiOiJub2RlIn0=", wantType: "VMESS", wantAddr: ""},
		{config: "proxies:\n  - name: a\n    server: node.example.com", wantType: "自定义", wantAddr: ""},
		{config: "", wantType: "自定义", wantAddr: ""},
	}
	for _, tt := range tests {
		gotType, gotAddr := describeProxyConfig(tt.config)
		if gotType != tt.wantType || gotAddr != tt.wantAddr {
			t.Fatalf("describeProxyConfig(%q) = (%q, %q), want (%q, %q)", tt.config, gotType, gotAddr, tt.wantType, tt.wantAddr)
		}
		for _, secret := range []string{"p%40ss", "p@ss", "uuid-secret", "pbk=key", "user:"} {
			if strings.Contains(gotAddr, secret) {
				t.Fatalf("地址里带出了敏感内容 %q: %q", secret, gotAddr)
			}
		}
	}
}

func TestPrepareStartPageURLWritesPageWithoutSecrets(t *testing.T) {
	t.Parallel()

	app := newStartPageTestApp(t)
	profile := &BrowserProfile{
		ProfileId:       "profile-1",
		ProfileName:     "样例1",
		Tags:            []string{"样例"},
		GroupId:         "group-x",
		LaunchCode:      "AB12CD",
		ProxyId:         "proxy-1",
		ProxyConfig:     "socks5://someone:secret-pass@203.0.113.10:1080",
		FingerprintArgs: []string{"--fingerprint=42", "--timezone=America/Los_Angeles"},
		LaunchArgs:      []string{"--secret-launch-arg=do-not-show"},
	}
	userDataDir := filepath.Join(app.appRoot, "data", "profile-1")

	input := newBrowserStartInput(profile.ProfileId, nil, nil, false, false, false, "", "")
	if got := app.prepareStartPageURL(input, profile, userDataDir, "socks5://127.0.0.1:40001"); got != "" {
		t.Fatalf("接口启动不应生成环境检测页，实际 %q", got)
	}
	if _, err := os.Stat(filepath.Join(userDataDir, internalbrowser.StartPageFileName)); !os.IsNotExist(err) {
		t.Fatal("接口启动不应写检测页文件")
	}

	input.ShowStartPage = true
	pageURL := app.prepareStartPageURL(input, profile, userDataDir, "socks5://127.0.0.1:40001")
	if !strings.HasPrefix(pageURL, "file://") {
		t.Fatalf("手动启动应当返回检测页地址，实际 %q", pageURL)
	}
	content, err := os.ReadFile(filepath.Join(userDataDir, internalbrowser.StartPageFileName))
	if err != nil {
		t.Fatalf("检测页文件没有生成: %v", err)
	}
	text := string(content)
	for _, secret := range []string{"secret-pass", "someone", "do-not-show"} {
		if strings.Contains(text, secret) {
			t.Fatalf("检测页里出现了不该出现的内容: %s", secret)
		}
	}
	for _, expected := range []string{"样例1", "家宽 1 号", "203.0.113.10:1080", "AB12CD", "group-x", "--timezone=America/Los_Angeles", "9.9.9"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("检测页里缺少 %q", expected)
		}
	}

	app.config.Browser.StartPageDisabled = true
	if got := app.prepareStartPageURL(input, profile, userDataDir, "socks5://127.0.0.1:40001"); got != "" {
		t.Fatalf("设置里关掉以后不应再生成，实际 %q", got)
	}
}

func TestDescribeStartPageProxyHandlesDirectAndBridgedProxy(t *testing.T) {
	t.Parallel()

	app := newStartPageTestApp(t)
	input := newBrowserStartInput("p", nil, nil, false, false, false, "", "")

	direct := &BrowserProfile{ProfileId: "p", ProxyId: "__direct__", ProxyConfig: "direct://"}
	if name, kind, addr, isDirect := app.describeStartPageProxy(input, direct, "direct://"); !isDirect || name != "" || kind != "直连" || addr != "" {
		t.Fatalf("直连实例描述不对: %q %q %q %v", name, kind, addr, isDirect)
	}

	bridged := &BrowserProfile{ProfileId: "p", ProxyId: "proxy-2"}
	name, kind, addr, isDirect := app.describeStartPageProxy(input, bridged, "socks5://127.0.0.1:40002")
	if isDirect || name != "节点 2 号" || kind != "VLESS" || addr != "" {
		t.Fatalf("经本地转接的代理应当显示代理池里的名称和类型: %q %q %q %v", name, kind, addr, isDirect)
	}

	forced := newBrowserStartInput("p", nil, nil, false, false, true, "", "")
	if _, _, _, isDirect := app.describeStartPageProxy(forced, bridged, "direct://"); !isDirect {
		t.Fatal("本次直连启动应当显示为直连")
	}
}

func TestManualStartPutsStartPageBeforeOtherDefaultURLs(t *testing.T) {
	t.Parallel()

	profile := &BrowserProfile{ProfileId: "profile-1", ProfileName: "样例1"}
	args := buildBrowserLaunchArgs(profile, `D:\data\profile-1`, 9222, "direct://", nil, nil, nil,
		[]string{"file:///D:/data/profile-1/wytaof-start.html", "https://chatgpt.example/"}, false, false)
	if len(args) < 2 {
		t.Fatalf("启动参数太少: %v", args)
	}
	tail := args[len(args)-2:]
	if tail[0] != "file:///D:/data/profile-1/wytaof-start.html" || tail[1] != "https://chatgpt.example/" {
		t.Fatalf("环境检测页应当排在其它启动页前面: %v", tail)
	}
}
