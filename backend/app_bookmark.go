package backend

import (
	"ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"ant-chrome/backend/internal/logger"
	"strings"
)

type BrowserBookmark = config.BrowserBookmark

type BookmarkSyncResult struct {
	Total       int      `json:"total"`
	Synced      int      `json:"synced"`
	Skipped     int      `json:"skipped"`
	Failed      int      `json:"failed"`
	SkippedList []string `json:"skippedList"`
	FailedList  []string `json:"failedList"`
}

var defaultBookmarkList = []BrowserBookmark{
	{Name: "Google", URL: "https://www.google.com/"},
	{Name: "Gmail", URL: "https://mail.google.com/"},
	{Name: "Claude", URL: "https://claude.ai/"},
	{Name: "ChatGPT", URL: "https://chatgpt.com/"},
	{Name: "YouTube", URL: "https://www.youtube.com/"},
	{Name: "IPPure", URL: "https://ippure.com/"},
	{Name: "IPLark", URL: "https://iplark.com/"},
	{Name: "Ping0", URL: "https://ping0.cc/"},
}

// BookmarkList 获取默认书签列表（优先 SQLite，降级 config.yaml）
func (a *App) BookmarkList() []BrowserBookmark {
	if a.browserMgr != nil && a.browserMgr.BookmarkDAO != nil {
		list, err := a.browserMgr.BookmarkDAO.List()
		if err == nil {
			return mergeBookmarksByURL(list, nil)
		}
	}
	if a.config != nil && len(a.config.Browser.DefaultBookmarks) > 0 {
		return mergeBookmarksByURL(a.config.Browser.DefaultBookmarks, nil)
	}
	return mergeBookmarksByURL(defaultBookmarkList, nil)
}

// BookmarkSave 保存默认书签列表（优先 SQLite，降级 config.yaml）
func (a *App) BookmarkSave(items []BrowserBookmark) error {
	log := logger.New("Bookmark")
	previous := a.BookmarkList()

	valid := make([]BrowserBookmark, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		url := strings.TrimSpace(item.URL)
		if name != "" && url != "" {
			valid = append(valid, BrowserBookmark{
				Name:         name,
				URL:          url,
				OpenOnStart:  item.OpenOnStart,
				OpenTags:     browser.NormalizeBookmarkScope(item.OpenTags),
				OpenGroupIds: browser.NormalizeBookmarkScope(item.OpenGroupIds),
			})
		}
	}
	valid = mergeBookmarksByURL(valid, nil)

	if a.browserMgr != nil {
		a.recordRemovedBookmarks(previous, valid)
	}

	if a.browserMgr != nil && a.browserMgr.BookmarkDAO != nil {
		if err := a.browserMgr.BookmarkDAO.ReplaceAll(valid); err != nil {
			log.Error("书签保存到数据库失败", logger.F("error", err.Error()))
			return err
		}
		log.Info("书签已保存到数据库", logger.F("count", len(valid)))
		return nil
	}

	// 降级：写入 config.yaml
	a.config.Browser.DefaultBookmarks = valid
	if err := a.config.Save(a.resolveAppPath("config.yaml")); err != nil {
		log.Error("书签保存失败", logger.F("error", err.Error()))
		return err
	}
	log.Info("书签已保存到 config.yaml", logger.F("count", len(valid)))
	return nil
}

func (a *App) recordRemovedBookmarks(previous []BrowserBookmark, current []BrowserBookmark) {
	if a.browserMgr == nil {
		return
	}
	currentKeys := make(map[string]struct{}, len(current))
	for _, item := range current {
		if key := browser.NormalizeBookmarkURLKey(item.URL); key != "" {
			currentKeys[key] = struct{}{}
		}
	}
	merged := make([]BrowserBookmark, 0, len(a.browserMgr.RemovedBookmarks)+len(previous))
	seen := make(map[string]struct{}, len(a.browserMgr.RemovedBookmarks)+len(previous))
	appendRemoved := func(item BrowserBookmark) {
		key := browser.NormalizeBookmarkURLKey(item.URL)
		if key == "" {
			return
		}
		if _, stillActive := currentKeys[key]; stillActive {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		merged = append(merged, item)
	}
	for _, item := range a.browserMgr.RemovedBookmarks {
		appendRemoved(item)
	}
	for _, item := range previous {
		appendRemoved(item)
	}
	a.browserMgr.RemovedBookmarks = merged
}

// BookmarkReset 恢复默认书签
func (a *App) BookmarkReset() error {
	return a.BookmarkSave(append([]BrowserBookmark{}, defaultBookmarkList...))
}

func mergeBookmarksByURL(items []BrowserBookmark, required []BrowserBookmark) []BrowserBookmark {
	merged := make([]BrowserBookmark, 0, len(items)+len(required))
	seen := make(map[string]struct{}, len(items)+len(required))
	appendOne := func(item BrowserBookmark) {
		name := strings.TrimSpace(item.Name)
		url := strings.TrimSpace(item.URL)
		if name == "" || url == "" {
			return
		}
		key := browser.NormalizeBookmarkURLKey(url)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		merged = append(merged, BrowserBookmark{
			Name:         name,
			URL:          url,
			OpenOnStart:  item.OpenOnStart,
			OpenTags:     browser.NormalizeBookmarkScope(item.OpenTags),
			OpenGroupIds: browser.NormalizeBookmarkScope(item.OpenGroupIds),
		})
	}
	for _, item := range items {
		appendOne(item)
	}
	for _, item := range required {
		appendOne(item)
	}
	return merged
}

func bookmarkOpensForProfile(bookmark BrowserBookmark, profile *BrowserProfile) bool {
	if !bookmark.OpenOnStart {
		return false
	}
	tags := browser.NormalizeBookmarkScope(bookmark.OpenTags)
	groupIds := browser.NormalizeBookmarkScope(bookmark.OpenGroupIds)
	if len(tags) == 0 && len(groupIds) == 0 {
		return true
	}
	if profile == nil {
		return false
	}
	for _, tag := range tags {
		for _, owned := range profile.Tags {
			if strings.EqualFold(strings.TrimSpace(owned), tag) {
				return true
			}
		}
	}
	groupId := strings.TrimSpace(profile.GroupId)
	if groupId != "" {
		for _, item := range groupIds {
			if item == groupId {
				return true
			}
		}
	}
	return false
}

func (a *App) renameBookmarkScopeTag(oldName string, newName string) error {
	if a.browserMgr == nil || a.browserMgr.BookmarkDAO == nil {
		return nil
	}
	list, err := a.browserMgr.BookmarkDAO.List()
	if err != nil {
		return err
	}
	changed := false
	for i := range list {
		for j, tag := range list[i].OpenTags {
			if strings.EqualFold(tag, oldName) {
				list[i].OpenTags[j] = newName
				changed = true
			}
		}
		list[i].OpenTags = browser.NormalizeBookmarkScope(list[i].OpenTags)
	}
	if !changed {
		return nil
	}
	return a.browserMgr.BookmarkDAO.ReplaceAll(list)
}

// BookmarkSyncToProfiles 将当前默认书签同步到已有未运行实例。
func (a *App) BookmarkSyncToProfiles() BookmarkSyncResult {
	result := BookmarkSyncResult{}
	log := logger.New("Bookmark")
	if a.browserMgr == nil {
		return result
	}
	bookmarks := a.BookmarkList()
	if len(bookmarks) == 0 && len(a.browserMgr.RemovedBookmarks) == 0 {
		return result
	}

	a.browserMgr.InitData()
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	result.Total = len(a.browserMgr.Profiles)
	for _, profile := range a.browserMgr.Profiles {
		if profile == nil {
			continue
		}
		if isBrowserProfileLive(profile, a.browserMgr.BrowserProcesses[profile.ProfileId]) {
			result.Skipped++
			result.SkippedList = append(result.SkippedList, profile.ProfileName)
			continue
		}

		userDataDir := a.browserMgr.ResolveUserDataDir(profile)
		if err := browser.SyncDefaultBookmarks(userDataDir, bookmarks, a.browserMgr.RemovedBookmarks); err != nil {
			result.Failed++
			name := profile.ProfileName
			if name == "" {
				name = profile.ProfileId
			}
			result.FailedList = append(result.FailedList, name)
			log.Error("同步默认书签到实例失败", logger.F("profile_id", profile.ProfileId), logger.F("error", err.Error()))
			continue
		}
		result.Synced++
	}
	return result
}
