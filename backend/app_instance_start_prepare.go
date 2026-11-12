package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/logger"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type browserStartInput struct {
	ProfileID            string
	ExtraLaunchArgs      []string
	StartURLs            []string
	SkipDefaultStartURLs bool
	PreferVisibleWindow  bool
	ForceDirectProxy     bool
	TemporaryProxyID     string
	TemporaryProxyConfig string
	ShowStartPage        bool
}

type browserStartPlan struct {
	profile               *BrowserProfile
	chromeBinaryPath      string
	userDataDir           string
	args                  []string
	startPageURL          string
	restoreLastSession    bool
	effectiveProxy        string
	acquiredXrayBridgeKey string
	releaseXrayBridge     bool
	assignedDebugPort     int
	startReadyTimeout     time.Duration
	startStableWindow     time.Duration
	maxStartAttempts      int
	totalReadyTimeout     time.Duration
}

func newBrowserStartInput(profileID string, extraLaunchArgs []string, startURLs []string, skipDefaultStartURLs bool, preferVisibleWindow bool, forceDirectProxy bool, proxyID string, proxyConfig string) browserStartInput {
	normalizedExtraLaunchArgs := normalizeNonEmptyStrings(extraLaunchArgs)
	if preferVisibleWindow {
		normalizedExtraLaunchArgs = ensureNewWindowLaunchArg(normalizedExtraLaunchArgs)
	}

	return browserStartInput{
		ProfileID:            profileID,
		ExtraLaunchArgs:      normalizedExtraLaunchArgs,
		StartURLs:            normalizeNonEmptyStrings(startURLs),
		SkipDefaultStartURLs: skipDefaultStartURLs,
		PreferVisibleWindow:  preferVisibleWindow,
		ForceDirectProxy:     forceDirectProxy,
		TemporaryProxyID:     strings.TrimSpace(proxyID),
		TemporaryProxyConfig: strings.TrimSpace(proxyConfig),
	}
}

func (input browserStartInput) hasTemporaryProxy() bool {
	return strings.TrimSpace(input.TemporaryProxyID) != "" || strings.TrimSpace(input.TemporaryProxyConfig) != ""
}

func (plan *browserStartPlan) releaseBridgeIfNeeded(a *App) {
	if plan == nil || a == nil {
		return
	}
	if plan.releaseXrayBridge && plan.acquiredXrayBridgeKey != "" && a.xrayMgr != nil {
		a.xrayMgr.ReleaseBridge(plan.acquiredXrayBridgeKey)
	}
}

func (a *App) resolveBrowserStartProfile(input browserStartInput) (*BrowserProfile, bool, error) {
	log := logger.New("Browser")

	profile, exists := a.browserMgr.Profiles[input.ProfileID]
	if !exists {
		err := fmt.Errorf("实例启动失败：未找到实例配置（ID=%s）。请刷新列表后重试。", input.ProfileID)
		log.Error("实例不存在", logger.F("profile_id", input.ProfileID), logger.F("reason", err.Error()))
		return nil, false, err
	}

	if !profile.Running {
		return profile, false, nil
	}

	if !isBrowserProfileLive(profile, a.browserMgr.BrowserProcesses[input.ProfileID]) {
		log.Info("检测到实例运行状态已失效，准备重新启动",
			logger.F("profile_id", input.ProfileID),
			logger.F("pid", profile.Pid),
			logger.F("debug_port", profile.DebugPort),
		)
		a.markProfileStoppedLocked(input.ProfileID, profile)
		return profile, false, nil
	}

	if input.PreferVisibleWindow {
		if err := a.openBrowserWindowForRunningProfile(profile, input.ExtraLaunchArgs, input.StartURLs); err != nil {
			startErr := fmt.Errorf("实例已在运行，但窗口唤起失败：%w", err)
			log.Error("运行中实例窗口唤起失败",
				logger.F("profile_id", input.ProfileID),
				logger.F("debug_port", profile.DebugPort),
				logger.F("error", err.Error()),
				logger.F("reason", startErr.Error()),
			)
			profile.LastError = startErr.Error()
			return profile, true, startErr
		}
	}

	if a.launchServer != nil && profile.DebugReady {
		a.launchServer.SetActiveProfile(profile)
	}
	a.emitBrowserInstanceStarted(profile, true)
	return profile, true, nil
}

func (a *App) prepareBrowserStartPlan(input browserStartInput, profile *BrowserProfile) (*browserStartPlan, error) {
	sanitizedProfileLaunchArgs, sanitizedExtraLaunchArgs, chromeBinaryPath, userDataDir, err := a.prepareBrowserLaunchContext(input, profile)
	if err != nil {
		return nil, err
	}

	effectiveProxy, acquiredXrayBridgeKey, releaseXrayBridge, err := a.resolveBrowserStartProxy(input, profile)
	if err != nil {
		return nil, err
	}

	startReadyTimeout, startStableWindow := a.browserStartTimingSettings()
	maxStartAttempts := browserStartAttemptCount()
	totalReadyTimeout := time.Duration(maxStartAttempts) * startReadyTimeout

	assignedDebugPort, err := nextAvailablePort()
	if err != nil {
		startErr := fmt.Errorf("实例启动失败：本地调试端口分配失败。原因：%v。请关闭占用端口的程序后重试。", err)
		logger.New("Browser").Error("调试端口分配失败",
			logger.F("profile_id", input.ProfileID),
			logger.F("error", err.Error()),
			logger.F("reason", startErr.Error()),
		)
		profile.LastError = startErr.Error()
		return nil, startErr
	}

	restoreLastSession := browserRestoreLastSession(a.config)
	startPageURL := a.prepareStartPageURL(input, profile, userDataDir, effectiveProxy)
	defaultStartURLs := a.browserDefaultStartURLs(profile)
	if startPageURL != "" && !restoreLastSession {
		defaultStartURLs = append([]string{startPageURL}, defaultStartURLs...)
	}

	return &browserStartPlan{
		profile:               profile,
		chromeBinaryPath:      chromeBinaryPath,
		userDataDir:           userDataDir,
		args:                  buildBrowserLaunchArgs(profile, userDataDir, assignedDebugPort, effectiveProxy, sanitizedProfileLaunchArgs, sanitizedExtraLaunchArgs, input.StartURLs, defaultStartURLs, input.SkipDefaultStartURLs, restoreLastSession),
		startPageURL:          startPageURL,
		restoreLastSession:    restoreLastSession,
		effectiveProxy:        effectiveProxy,
		acquiredXrayBridgeKey: acquiredXrayBridgeKey,
		releaseXrayBridge:     releaseXrayBridge,
		assignedDebugPort:     assignedDebugPort,
		startReadyTimeout:     startReadyTimeout,
		startStableWindow:     startStableWindow,
		maxStartAttempts:      maxStartAttempts,
		totalReadyTimeout:     totalReadyTimeout,
	}, nil
}

func (a *App) prepareBrowserLaunchContext(input browserStartInput, profile *BrowserProfile) ([]string, []string, string, string, error) {
	log := logger.New("Browser")

	proxyChanged := a.browserMgr.ApplyDefaults(profile)
	if proxyChanged {
		_ = a.browserMgr.SaveProfiles()
	}

	sanitizedProfileLaunchArgs, managedProfileArgs := sanitizeManagedLaunchArgs(profile.LaunchArgs)
	sanitizedExtraLaunchArgs, managedExtraArgs := sanitizeManagedLaunchArgs(input.ExtraLaunchArgs)
	logManagedLaunchArgOverrides(log, input.ProfileID, "profile.launchArgs", managedProfileArgs)
	logManagedLaunchArgOverrides(log, input.ProfileID, "start.extraLaunchArgs", managedExtraArgs)

	chromeBinaryPath, err := a.browserMgr.ResolveChromeBinary(profile)
	if err != nil {
		startErr := fmt.Errorf("实例启动失败：%w", err)
		log.Error("内核路径解析失败",
			logger.F("profile_id", input.ProfileID),
			logger.F("error", err.Error()),
			logger.F("reason", startErr.Error()),
		)
		profile.LastError = startErr.Error()
		return nil, nil, "", "", startErr
	}

	userDataDir := a.browserMgr.ResolveUserDataDir(profile)
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		startErr := fmt.Errorf("实例启动失败：无法创建用户数据目录 %s。原因：%w。请检查目录权限或路径配置。", userDataDir, err)
		log.Error("用户数据目录创建失败",
			logger.F("profile_id", input.ProfileID),
			logger.F("dir", userDataDir),
			logger.F("error", err.Error()),
			logger.F("reason", startErr.Error()),
		)
		profile.LastError = startErr.Error()
		return nil, nil, "", "", startErr
	}

	if err := browser.SyncDefaultBookmarks(userDataDir, a.BookmarkList(), a.browserMgr.RemovedBookmarks); err != nil {
		log.Error("默认书签写入失败", logger.F("error", err.Error()))
	}

	if a.instanceNameOnWindowEnabled() {
		if _, err := browser.EnsureProfileDisplayName(userDataDir, profile.ProfileName); err != nil {
			log.Warn("账户名称同步失败，不影响启动", logger.F("profile_id", input.ProfileID), logger.F("error", err.Error()))
		}
	}

	configuredLang := browser.ExtractConfiguredLanguage(profile.FingerprintArgs, sanitizedProfileLaunchArgs, sanitizedExtraLaunchArgs)
	if _, err := browser.SyncProfileLanguagePreferences(userDataDir, configuredLang); err != nil {
		log.Warn("实例语言配置同步失败，不影响启动", logger.F("profile_id", input.ProfileID), logger.F("error", err.Error()))
	}

	if !browserRestoreLastSession(a.config) {
		if err := browser.ClearSessionRestoreData(userDataDir); err != nil {
			sessionDir := filepath.Join(userDataDir, "Default", "Sessions")
			startErr := fmt.Errorf("实例启动失败：无法清理上次会话缓存 %s。原因：%w。请关闭占用该目录的浏览器进程后重试。", sessionDir, err)
			log.Error("会话恢复缓存清理失败",
				logger.F("profile_id", input.ProfileID),
				logger.F("dir", sessionDir),
				logger.F("error", err.Error()),
				logger.F("reason", startErr.Error()),
			)
			profile.LastError = startErr.Error()
			return nil, nil, "", "", startErr
		}
	}

	return sanitizedProfileLaunchArgs, sanitizedExtraLaunchArgs, chromeBinaryPath, userDataDir, nil
}

func buildBrowserLaunchArgs(profile *BrowserProfile, userDataDir string, debugPort int, effectiveProxy string, sanitizedProfileLaunchArgs []string, sanitizedExtraLaunchArgs []string, startURLs []string, defaultStartURLs []string, skipDefaultStartURLs bool, restoreLastSession bool) []string {
	args := []string{
		fmt.Sprintf("--user-data-dir=%s", userDataDir),
		fmt.Sprintf("--remote-debugging-port=%d", debugPort),
		"--disable-session-crashed-bubble",
	}

	hasFingerprint := false
	for _, arg := range profile.FingerprintArgs {
		if strings.HasPrefix(arg, "--fingerprint=") {
			hasFingerprint = true
			break
		}
	}
	if !hasFingerprint {
		seed := 0
		for _, char := range profile.ProfileId {
			seed = (seed << 5) - seed + int(char)
		}
		if seed < 0 {
			seed = -seed
		}
		args = append(args, fmt.Sprintf("--fingerprint=%d", seed))
	}

	if effectiveProxy == "direct://" {
		args = append(args, "--proxy-server=direct://")
	} else if effectiveProxy != "" {
		args = append(args, fmt.Sprintf("--proxy-server=%s", effectiveProxy))
	}

	combinedCustomArgs := make([]string, 0, len(profile.FingerprintArgs)+len(sanitizedProfileLaunchArgs)+len(sanitizedExtraLaunchArgs))
	combinedCustomArgs = append(combinedCustomArgs, profile.FingerprintArgs...)
	combinedCustomArgs = append(combinedCustomArgs, sanitizedProfileLaunchArgs...)
	combinedCustomArgs = append(combinedCustomArgs, sanitizedExtraLaunchArgs...)

	args = append(args, normalizeFingerprintLaunchArgs(combinedCustomArgs)...)
	return appendLaunchTargets(args, startURLs, defaultStartURLs, skipDefaultStartURLs, restoreLastSession)
}

// normalizeFingerprintLaunchArgs 兼容不同指纹内核的参数命名差异，并补齐语言/分辨率配套开关
func normalizeFingerprintLaunchArgs(customArgs []string) []string {
	if len(customArgs) == 0 {
		return nil
	}
	out := make([]string, 0, len(customArgs)+6)

	hasAcceptLang := false
	hasScreenWidth := false
	hasScreenHeight := false
	hasMemory := false
	hasGPUVendor := false
	hasGPURenderer := false
	lastLang := ""
	windowWidth := ""
	windowHeight := ""
	deviceMemory := ""
	webglVendor := ""
	webglRenderer := ""

	for _, raw := range customArgs {
		arg := strings.TrimSpace(raw)
		if arg == "" {
			continue
		}
		lower := strings.ToLower(arg)
		switch {
		case lower == "--fingerprint-platform=mac":
			arg = "--fingerprint-platform=macos"
		case strings.HasPrefix(lower, "--accept-lang=") || strings.HasPrefix(lower, "--accept-language="):
			hasAcceptLang = true
		case strings.HasPrefix(lower, "--lang="):
			if v := strings.TrimSpace(arg[len("--lang="):]); v != "" {
				lastLang = v
			}
		case strings.HasPrefix(lower, "--fingerprint-screen-width="):
			hasScreenWidth = true
		case strings.HasPrefix(lower, "--fingerprint-screen-height="):
			hasScreenHeight = true
		case strings.HasPrefix(lower, "--window-size="):
			val := strings.TrimSpace(arg[len("--window-size="):])
			if w, h, ok := parseResolutionPair(val); ok {
				windowWidth, windowHeight = w, h
			}
		case strings.HasPrefix(lower, "--fingerprint-memory="):
			hasMemory = true
		case strings.HasPrefix(lower, "--fingerprint-device-memory="):
			if v := strings.TrimSpace(arg[len("--fingerprint-device-memory="):]); v != "" {
				deviceMemory = v
			}
		case strings.HasPrefix(lower, "--fingerprint-gpu-vendor="):
			hasGPUVendor = true
		case strings.HasPrefix(lower, "--fingerprint-webgl-vendor="):
			if v := strings.TrimSpace(arg[len("--fingerprint-webgl-vendor="):]); v != "" {
				webglVendor = v
			}
		case strings.HasPrefix(lower, "--fingerprint-gpu-renderer="):
			hasGPURenderer = true
		case strings.HasPrefix(lower, "--fingerprint-webgl-renderer="):
			if v := strings.TrimSpace(arg[len("--fingerprint-webgl-renderer="):]); v != "" {
				webglRenderer = v
			}
		}
		out = append(out, arg)
	}

	if !hasAcceptLang && lastLang != "" {
		if acceptList := browser.BuildAcceptLanguageList(lastLang); acceptList != "" {
			out = append(out, "--accept-lang="+acceptList)
		}
	}
	if !hasScreenWidth && windowWidth != "" {
		out = append(out, "--fingerprint-screen-width="+windowWidth)
	}
	if !hasScreenHeight && windowHeight != "" {
		out = append(out, "--fingerprint-screen-height="+windowHeight)
	}
	if !hasMemory && deviceMemory != "" {
		out = append(out, "--fingerprint-memory="+deviceMemory)
	}
	if !hasGPUVendor && webglVendor != "" {
		out = append(out, "--fingerprint-gpu-vendor="+webglVendor)
	}
	if !hasGPURenderer && webglRenderer != "" {
		out = append(out, "--fingerprint-gpu-renderer="+webglRenderer)
	}
	return out
}

func parseResolutionPair(raw string) (string, string, bool) {
	sep := ","
	if !strings.Contains(raw, ",") && strings.Contains(strings.ToLower(raw), "x") {
		sep = "x"
	}
	parts := strings.Split(strings.ToLower(raw), sep)
	if len(parts) != 2 {
		return "", "", false
	}
	w := strings.TrimSpace(parts[0])
	h := strings.TrimSpace(parts[1])
	if w == "" || h == "" {
		return "", "", false
	}
	for _, ch := range w + h {
		if ch < '0' || ch > '9' {
			return "", "", false
		}
	}
	return w, h, true
}
