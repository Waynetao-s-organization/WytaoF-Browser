package backend

func (a *App) BrowserInstanceStart(profileId string) (*BrowserProfile, error) {
	return a.browserInstanceStartManually(profileId, false)
}

func shouldPreferVisibleWindowForStartWithParams(startURLs []string) bool {
	return len(normalizeNonEmptyStrings(startURLs)) > 0
}

// BrowserInstanceStartDirect 仅本次启动走直连，不落库修改实例代理配置。
func (a *App) BrowserInstanceStartDirect(profileId string) (*BrowserProfile, error) {
	return a.browserInstanceStartManually(profileId, true)
}

// BrowserInstanceStartWithParams 通过额外参数启动实例（仅本次启动生效，不落库）
func (a *App) BrowserInstanceStartWithParams(profileId string, extraLaunchArgs []string, startURLs []string, skipDefaultStartURLs bool) (*BrowserProfile, error) {
	preferVisibleWindow := shouldPreferVisibleWindowForStartWithParams(startURLs)
	return a.browserInstanceStartInternal(profileId, extraLaunchArgs, startURLs, skipDefaultStartURLs, preferVisibleWindow, false, "", "")
}

// browserInstanceStartManually 用户在界面上手动启动实例，会带上环境检测页；接口和脚本启动不走这里。
func (a *App) browserInstanceStartManually(profileId string, forceDirectProxy bool) (*BrowserProfile, error) {
	input := newBrowserStartInput(profileId, nil, nil, false, false, forceDirectProxy, "", "")
	input.ShowStartPage = true
	return a.browserInstanceStartWithInput(input)
}

func (a *App) browserInstanceStartInternal(profileId string, extraLaunchArgs []string, startURLs []string, skipDefaultStartURLs bool, preferVisibleWindow bool, forceDirectProxy bool, proxyId string, proxyConfig string) (*BrowserProfile, error) {
	return a.browserInstanceStartWithInput(newBrowserStartInput(profileId, extraLaunchArgs, startURLs, skipDefaultStartURLs, preferVisibleWindow, forceDirectProxy, proxyId, proxyConfig))
}

func (a *App) browserInstanceStartWithInput(input browserStartInput) (*BrowserProfile, error) {
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	profile, handled, err := a.resolveBrowserStartProfile(input)
	if err != nil || handled {
		return profile, err
	}

	plan, err := a.prepareBrowserStartPlan(input, profile)
	if err != nil {
		return profile, err
	}
	defer plan.releaseBridgeIfNeeded(a)

	return a.startBrowserProfileWithPlan(input, plan)
}
