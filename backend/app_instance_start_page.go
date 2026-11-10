package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const startPageRestoreCheckDelay = 1500 * time.Millisecond

func (a *App) startPageEnabled() bool {
	return a.config == nil || !a.config.Browser.StartPageDisabled
}

func (a *App) instanceNameOnWindowEnabled() bool {
	return a.config == nil || !a.config.Browser.WindowTitleDisabled
}

func shouldOpenStartPage(input browserStartInput, enabled bool) bool {
	return enabled && input.ShowStartPage && len(input.StartURLs) == 0 && !input.SkipDefaultStartURLs
}

func (a *App) prepareStartPageURL(input browserStartInput, profile *BrowserProfile, userDataDir string, effectiveProxy string) string {
	if profile == nil || !shouldOpenStartPage(input, a.startPageEnabled()) {
		return ""
	}
	pageURL, err := browser.WriteStartPage(userDataDir, a.buildStartPageData(input, profile, effectiveProxy))
	if err != nil {
		logger.New("Browser").Warn("环境检测页生成失败，本次启动不打开",
			logger.F("profile_id", profile.ProfileId),
			logger.F("error", err.Error()),
		)
		return ""
	}
	return pageURL
}

func (a *App) buildStartPageData(input browserStartInput, profile *BrowserProfile, effectiveProxy string) browser.StartPageData {
	data := browser.StartPageData{
		AppName:         a.appName(),
		ProfileID:       profile.ProfileId,
		ProfileName:     profile.ProfileName,
		Tags:            append([]string{}, profile.Tags...),
		LaunchCode:      strings.TrimSpace(profile.LaunchCode),
		FingerprintArgs: append([]string{}, profile.FingerprintArgs...),
		GeneratedAt:     time.Now().Format(time.RFC3339),
	}
	if version := a.appVersion(); version != "unknown" {
		data.AppVersion = version
	}
	if data.LaunchCode == "" && a.launchCodeSvc != nil {
		if code, err := a.launchCodeSvc.EnsureCode(profile.ProfileId); err == nil {
			data.LaunchCode = code
		}
	}

	if groupID := strings.TrimSpace(profile.GroupId); groupID != "" {
		data.GroupName = groupID
		if a.browserMgr != nil && a.browserMgr.GroupDAO != nil {
			if group, err := a.browserMgr.GroupDAO.GetById(groupID); err == nil && group != nil && strings.TrimSpace(group.GroupName) != "" {
				data.GroupName = group.GroupName
			}
		}
	}

	if a.browserMgr != nil {
		core, ok := a.browserMgr.GetCore(profile.CoreId)
		if !ok {
			core, ok = a.browserMgr.GetDefaultCore()
		}
		if ok {
			data.CoreName = core.CoreName
			data.CoreVersion = a.browserMgr.GetChromeVersion(core.CorePath)
		}
	}

	data.ProxyName, data.ProxyType, data.ProxyAddress, data.ProxyDirect = a.describeStartPageProxy(input, profile, effectiveProxy)
	return data
}

func (a *App) describeStartPageProxy(input browserStartInput, profile *BrowserProfile, effectiveProxy string) (string, string, string, bool) {
	effective := strings.TrimSpace(effectiveProxy)
	if input.ForceDirectProxy || effective == "" || strings.EqualFold(effective, "direct://") {
		return "", "直连", "", true
	}

	proxyID := strings.TrimSpace(profile.ProxyId)
	proxyConfig := strings.TrimSpace(profile.ProxyConfig)
	if input.hasTemporaryProxy() {
		proxyID = strings.TrimSpace(input.TemporaryProxyID)
		proxyConfig = strings.TrimSpace(input.TemporaryProxyConfig)
	}

	name := ""
	if proxyID != "" && a.browserMgr != nil {
		for _, item := range a.getLatestProxies() {
			if strings.EqualFold(item.ProxyId, proxyID) {
				name = strings.TrimSpace(item.ProxyName)
				proxyConfig = strings.TrimSpace(item.ProxyConfig)
				break
			}
		}
	}
	if name == "" && !input.hasTemporaryProxy() {
		name = strings.TrimSpace(profile.ProxyBindName)
	}
	proxyType, address := describeProxyConfig(proxyConfig)
	return name, proxyType, address, false
}

// describeProxyConfig 只返回代理类型和“主机:端口”，账号密码、节点密钥一律不带出来
func describeProxyConfig(proxyConfig string) (string, string) {
	cfg := strings.TrimSpace(proxyConfig)
	scheme, _, found := strings.Cut(cfg, "://")
	if !found || scheme == "" || strings.ContainsAny(scheme, " \t\r\n:{[") {
		return "自定义", ""
	}
	scheme = strings.ToLower(scheme)
	switch scheme {
	case "http", "https", "socks", "socks4", "socks4a", "socks5", "socks5h":
		if parsed, err := url.Parse(cfg); err == nil && parsed.Host != "" {
			return strings.ToUpper(scheme), parsed.Host
		}
	}
	return strings.ToUpper(scheme), ""
}

type cdpPageTarget struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

func listCDPPageTargets(debugPort int) ([]cdpPageTarget, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", debugPort))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	var targets []cdpPageTarget
	if err := json.Unmarshal(body, &targets); err != nil {
		return nil, err
	}
	return targets, nil
}

// ensureStartPageTab 用在“恢复上次标签页”开着的时候：上次的检测页如果被恢复出来就不再重复打开。
// 调试接口还没准备好时，在 wait 时间内每隔一会儿再试。
func (a *App) ensureStartPageTab(debugPort int, pageURL string, wait time.Duration) {
	if debugPort <= 0 || strings.TrimSpace(pageURL) == "" {
		return
	}
	log := logger.New("Browser")
	deadline := time.Now().Add(wait)
	for {
		time.Sleep(startPageRestoreCheckDelay)
		targets, err := listCDPPageTargets(debugPort)
		if err != nil {
			if time.Now().Before(deadline) {
				continue
			}
			log.Warn("环境检测页检查失败", logger.F("debug_port", debugPort), logger.F("error", err.Error()))
			return
		}
		for _, target := range targets {
			if target.Type == "page" && browser.SameFileURL(target.URL, pageURL) {
				return
			}
		}
		if err := cdpBrowserCall(debugPort, "Target.createTarget", map[string]any{"url": pageURL}); err != nil {
			log.Warn("环境检测页打开失败", logger.F("debug_port", debugPort), logger.F("error", err.Error()))
		}
		return
	}
}
