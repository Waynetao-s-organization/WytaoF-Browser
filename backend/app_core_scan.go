package backend

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	"path/filepath"
	goruntime "runtime"
	"strings"
)

const coreScanRoot = "chrome"

func (a *App) corePathKey(corePath string) string {
	corePath = strings.TrimSpace(corePath)
	if corePath == "" {
		return ""
	}
	key := filepath.Clean(a.resolveAppPath(corePath))
	if goruntime.GOOS == "windows" || goruntime.GOOS == "darwin" {
		key = strings.ToLower(key)
	}
	return key
}

func (a *App) isIgnoredCorePath(corePath string) bool {
	key := a.corePathKey(corePath)
	if key == "" || a.config == nil {
		return false
	}
	for _, item := range a.config.Browser.IgnoredCorePaths {
		if a.corePathKey(item) == key {
			return true
		}
	}
	return false
}

func (a *App) setCorePathIgnored(corePath string, ignored bool) bool {
	key := a.corePathKey(corePath)
	if key == "" || a.config == nil {
		return false
	}
	kept := make([]string, 0, len(a.config.Browser.IgnoredCorePaths)+1)
	found := false
	for _, item := range a.config.Browser.IgnoredCorePaths {
		if a.corePathKey(item) == key {
			found = true
			continue
		}
		kept = append(kept, item)
	}
	if ignored {
		kept = append(kept, strings.TrimSpace(corePath))
	}
	if found == ignored {
		return false
	}
	a.config.Browser.IgnoredCorePaths = kept
	return true
}

func (a *App) saveIgnoredCorePaths() {
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		logger.New("Browser").Error("内核忽略名单保存失败", logger.F("error", err))
	}
}

func uniqueCoreID(base string, taken map[string]struct{}) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "core"
	}
	candidate := base
	for i := 2; ; i++ {
		if _, ok := taken[strings.ToLower(candidate)]; !ok {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
}

// registerDetectedCores 把 chrome 目录下还没登记的内核写进内核表，返回新登记的数量。
// includeIgnored 为 true 时连用户在界面上删除过的内核目录也重新登记。
func (a *App) registerDetectedCores(includeIgnored bool) int {
	if a.browserMgr == nil || a.browserMgr.CoreDAO == nil {
		return 0
	}
	log := logger.New("Browser")

	detected := a.scanChromeDir(coreScanRoot)
	if len(detected) == 0 {
		return 0
	}
	existing, err := a.browserMgr.CoreDAO.List()
	if err != nil {
		log.Error("读取内核表失败，跳过新内核登记", logger.F("error", err))
		return 0
	}

	knownPaths := make(map[string]struct{}, len(existing))
	knownIDs := make(map[string]struct{}, len(existing))
	for _, core := range existing {
		knownPaths[a.corePathKey(core.CorePath)] = struct{}{}
		knownIDs[strings.ToLower(strings.TrimSpace(core.CoreId))] = struct{}{}
	}

	registered := 0
	ignoreChanged := false
	for _, core := range detected {
		key := a.corePathKey(core.CorePath)
		if _, ok := knownPaths[key]; ok {
			continue
		}
		if a.isIgnoredCorePath(core.CorePath) {
			if !includeIgnored {
				continue
			}
			if a.setCorePathIgnored(core.CorePath, false) {
				ignoreChanged = true
			}
		}

		core.CoreId = uniqueCoreID(core.CoreId, knownIDs)
		core.IsDefault = len(existing) == 0 && registered == 0
		if err := a.browserMgr.CoreDAO.Upsert(core); err != nil {
			log.Error("新内核登记失败", logger.F("path", core.CorePath), logger.F("error", err))
			continue
		}
		knownPaths[key] = struct{}{}
		knownIDs[strings.ToLower(core.CoreId)] = struct{}{}
		registered++
		log.Info("发现新内核，已登记", logger.F("core_id", core.CoreId), logger.F("path", core.CorePath))
	}

	if registered > 0 {
		a.browserMgr.ListCores()
	}
	if ignoreChanged {
		a.saveIgnoredCorePaths()
	}
	return registered
}

func (a *App) ignoreDeletedCoreIfScannable(corePath string) {
	key := a.corePathKey(corePath)
	if key == "" {
		return
	}
	for _, core := range a.scanChromeDir(coreScanRoot) {
		if a.corePathKey(core.CorePath) != key {
			continue
		}
		if a.setCorePathIgnored(core.CorePath, true) {
			a.saveIgnoredCorePaths()
		}
		return
	}
}
