package backend

import (
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"fmt"
	"strings"
)

func (a *App) GetBrowserSettings() BrowserSettings {
	return BrowserSettings{
		UserDataRoot:           a.config.Browser.UserDataRoot,
		DefaultFingerprintArgs: append([]string{}, a.config.Browser.DefaultFingerprintArgs...),
		DefaultLaunchArgs:      append([]string{}, a.config.Browser.DefaultLaunchArgs...),
		DefaultStartURLs:       append([]string{}, a.config.Browser.DefaultStartURLs...),
		RestoreLastSession:     a.config.Browser.RestoreLastSession,
		StartPageDisabled:      a.config.Browser.StartPageDisabled,
		WindowTitleDisabled:    a.config.Browser.WindowTitleDisabled,
		StartReadyTimeoutMs:    browserStartReadyTimeoutMillis(a.config),
		StartStableWindowMs:    browserStartStableWindowMillis(a.config),
	}
}

func (a *App) SaveBrowserSettings(settings BrowserSettings) error {
	log := logger.New("Browser")
	a.config.Browser.UserDataRoot = strings.TrimSpace(settings.UserDataRoot)
	a.config.Browser.DefaultFingerprintArgs = append([]string{}, settings.DefaultFingerprintArgs...)
	a.config.Browser.DefaultLaunchArgs = append([]string{}, settings.DefaultLaunchArgs...)
	if settings.DefaultStartURLs != nil {
		a.config.Browser.DefaultStartURLs = normalizeNonEmptyStrings(settings.DefaultStartURLs)
	} else if a.config.Browser.DefaultStartURLs == nil {
		a.config.Browser.DefaultStartURLs = config.DefaultBrowserStartURLs()
	}
	a.config.Browser.RestoreLastSession = settings.RestoreLastSession
	a.config.Browser.StartPageDisabled = settings.StartPageDisabled
	a.config.Browser.WindowTitleDisabled = settings.WindowTitleDisabled
	if settings.StartReadyTimeoutMs > 0 {
		a.config.Browser.StartReadyTimeoutMs = settings.StartReadyTimeoutMs
	} else if a.config.Browser.StartReadyTimeoutMs <= 0 {
		a.config.Browser.StartReadyTimeoutMs = browserStartReadyTimeoutMillis(nil)
	}
	if settings.StartStableWindowMs > 0 {
		a.config.Browser.StartStableWindowMs = settings.StartStableWindowMs
	} else if a.config.Browser.StartStableWindowMs <= 0 {
		a.config.Browser.StartStableWindowMs = browserStartStableWindowMillis(nil)
	}
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		log.Error("浏览器配置保存失败", logger.F("error", err))
		return err
	}
	return nil
}

func (a *App) BrowserCoreList() []BrowserCore {
	return a.browserMgr.ListCores()
}

func (a *App) BrowserCoreSave(input BrowserCoreInput) error {
	if err := a.browserMgr.SaveCore(input); err != nil {
		return err
	}
	if a.setCorePathIgnored(input.CorePath, false) {
		a.saveIgnoredCorePaths()
	}
	return nil
}

func (a *App) BrowserCoreDelete(coreId string) error {
	core, found := a.browserMgr.GetCore(coreId)
	if err := a.browserMgr.DeleteCore(coreId); err != nil {
		return err
	}
	if found {
		a.ignoreDeletedCoreIfScannable(core.CorePath)
	}
	return nil
}

func (a *App) BrowserCoreSetDefault(coreId string) error {
	return a.browserMgr.SetDefaultCore(coreId)
}

func (a *App) BrowserCoreValidate(corePath string) BrowserCoreValidateResult {
	return a.browserMgr.ValidateCorePath(corePath)
}

func (a *App) BrowserCoreExtendedInfo() []BrowserCoreExtendedInfo {
	return a.browserMgr.GetCoresExtendedInfo()
}

// BrowserCoreScan 重新扫描 chrome 目录，自动注册新内核
func (a *App) BrowserCoreScan() []BrowserCore {
	a.registerDetectedCores(true)
	a.autoDetectCores()
	return a.browserMgr.ListCores()
}

// BrowserCoreDownload 在线下载并自动解压配置内核
func (a *App) BrowserCoreDownload(coreName, url, proxyConfig string) error {
	if a.ctx == nil {
		return fmt.Errorf("app context is nil")
	}
	go a.browserMgr.DownloadAndExtractCore(a.ctx, coreName, url, proxyConfig)
	return nil
}
