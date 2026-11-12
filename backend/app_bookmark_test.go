package backend

import (
	internalbrowser "ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBookmarkSyncToProfilesAppliesCurrentDefaults(t *testing.T) {
	t.Parallel()

	appRoot := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Browser.UserDataRoot = "data"
	cfg.Browser.DefaultBookmarks = []config.BrowserBookmark{
		{Name: "默认书签", URL: "https://default.example/"},
	}

	app := NewApp(appRoot)
	app.config = cfg
	app.browserMgr = internalbrowser.NewManager(cfg, appRoot)
	app.browserMgr.Profiles["profile-1"] = &internalbrowser.Profile{
		ProfileId:   "profile-1",
		ProfileName: "实例 1",
		UserDataDir: "profile-1",
	}

	result := app.BookmarkSyncToProfiles()
	if result.Total != 1 || result.Synced != 1 || result.Skipped != 0 || result.Failed != 0 {
		t.Fatalf("unexpected sync result: %+v", result)
	}

	bookmarksPath := filepath.Join(appRoot, "data", "profile-1", "Default", "Bookmarks")
	data, err := os.ReadFile(bookmarksPath)
	if err != nil {
		t.Fatalf("read bookmarks: %v", err)
	}
	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("unmarshal bookmarks: %v", err)
	}
	if countBookmarkURLInRoot(root, "https://default.example/") != 1 {
		t.Fatalf("default bookmark was not synced once: %s", string(data))
	}
}

func TestBookmarkSaveRespectsOrderAndDeletionIncludingIPCheckBookmarks(t *testing.T) {
	t.Parallel()

	appRoot := t.TempDir()
	cfg := config.DefaultConfig()
	app := NewApp(appRoot)
	app.config = cfg
	app.browserMgr = internalbrowser.NewManager(cfg, appRoot)

	// 先恢复默认书签（含 Google、IPPure、IPLark、Ping0 等 8 项）
	if err := app.BookmarkReset(); err != nil {
		t.Fatalf("BookmarkReset returned error: %v", err)
	}

	// 用户删除了 IPPure、IPLark、YouTube 等，只留 3 项并调整了顺序
	custom := []config.BrowserBookmark{
		{Name: "Ping0", URL: "https://ping0.cc/"},
		{Name: "自定义站点", URL: "https://custom.example/", OpenOnStart: true},
		{Name: "Google", URL: "https://www.google.com/"},
	}
	if err := app.BookmarkSave(custom); err != nil {
		t.Fatalf("BookmarkSave returned error: %v", err)
	}

	got := app.BookmarkList()
	if len(got) != 3 {
		t.Fatalf("期望保存后只剩 3 项，实际得到 %d 项: %+v", len(got), got)
	}
	wantOrder := []string{"https://ping0.cc/", "https://custom.example/", "https://www.google.com/"}
	for i, wantURL := range wantOrder {
		if got[i].URL != wantURL {
			t.Fatalf("第 %d 项顺序不对: got=%s want=%s (all=%+v)", i, got[i].URL, wantURL, got)
		}
	}
	for _, deletedURL := range []string{"https://ippure.com/", "https://iplark.com/", "https://www.youtube.com/"} {
		if countBookmarkItemsByURL(got, deletedURL) != 0 {
			t.Fatalf("已删除的书签 %s 不应再次出现: %+v", deletedURL, got)
		}
	}
}

func TestBookmarkSyncToProfilesUpdatesOrderAndRemovesDeletedDefaults(t *testing.T) {
	t.Parallel()

	appRoot := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Browser.UserDataRoot = "data"
	app := NewApp(appRoot)
	app.config = cfg
	app.browserMgr = internalbrowser.NewManager(cfg, appRoot)
	app.browserMgr.Profiles["profile-1"] = &internalbrowser.Profile{
		ProfileId:   "profile-1",
		ProfileName: "样例实例1",
		UserDataDir: "profile-1",
	}

	initial := []config.BrowserBookmark{
		{Name: "站点A", URL: "https://a.example/"},
		{Name: "站点B", URL: "https://b.example/"},
		{Name: "站点C", URL: "https://c.example/"},
	}
	if err := app.BookmarkSave(initial); err != nil {
		t.Fatalf("初次保存失败: %v", err)
	}
	if res := app.BookmarkSyncToProfiles(); res.Synced != 1 {
		t.Fatalf("初次同步结果异常: %+v", res)
	}

	// 用户删除了 站点B，把 站点C 移到 站点A 前面，并把 站点A 改名
	updated := []config.BrowserBookmark{
		{Name: "站点C", URL: "https://c.example/"},
		{Name: "站点A新名", URL: "https://a.example/"},
	}
	if err := app.BookmarkSave(updated); err != nil {
		t.Fatalf("二次保存失败: %v", err)
	}
	if res := app.BookmarkSyncToProfiles(); res.Synced != 1 {
		t.Fatalf("二次同步结果异常: %+v", res)
	}

	bookmarksPath := filepath.Join(appRoot, "data", "profile-1", "Default", "Bookmarks")
	data, err := os.ReadFile(bookmarksPath)
	if err != nil {
		t.Fatalf("读取实例书签失败: %v", err)
	}
	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("解析实例书签失败: %v", err)
	}
	roots := root["roots"].(map[string]interface{})
	bar := roots["bookmark_bar"].(map[string]interface{})
	children := bar["children"].([]interface{})
	if len(children) != 2 {
		t.Fatalf("期望实例书签栏有 2 项，实际 %d 项: %s", len(children), string(data))
	}
	first := children[0].(map[string]interface{})
	second := children[1].(map[string]interface{})
	if first["url"] != "https://c.example/" || second["url"] != "https://a.example/" || second["name"] != "站点A新名" {
		t.Fatalf("实例书签栏顺序或名称未更新: first=%+v second=%+v", first, second)
	}
}

func TestBrowserDefaultStartURLsIncludesOpenOnStartBookmarks(t *testing.T) {
	t.Parallel()

	appRoot := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Browser.DefaultStartURLs = []string{"https://home.example/"}
	cfg.Browser.DefaultBookmarks = []config.BrowserBookmark{
		{Name: "启动打开", URL: "https://open.example/", OpenOnStart: true},
		{Name: "普通书签", URL: "https://closed.example/"},
		{Name: "重复启动页", URL: "https://home.example/", OpenOnStart: true},
	}

	app := NewApp(appRoot)
	app.config = cfg
	app.browserMgr = internalbrowser.NewManager(cfg, appRoot)

	got := app.browserDefaultStartURLs(nil)
	want := []string{"https://home.example/", "https://open.example/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default start urls mismatch: got=%v want=%v", got, want)
	}
}

func TestBookmarkOpensForProfileRespectsTagAndGroupScope(t *testing.T) {
	t.Parallel()

	teamProfile := &internalbrowser.Profile{ProfileId: "p-team", Tags: []string{"Team", "测试"}}
	groupProfile := &internalbrowser.Profile{ProfileId: "p-group", GroupId: "group-google"}
	plainProfile := &internalbrowser.Profile{ProfileId: "p-plain"}

	tests := []struct {
		name     string
		bookmark config.BrowserBookmark
		profile  *internalbrowser.Profile
		want     bool
	}{
		{name: "没勾启动打开", bookmark: config.BrowserBookmark{OpenTags: []string{"Team"}}, profile: teamProfile, want: false},
		{name: "不限范围对所有实例生效", bookmark: config.BrowserBookmark{OpenOnStart: true}, profile: plainProfile, want: true},
		{name: "不限范围时没有实例信息也生效", bookmark: config.BrowserBookmark{OpenOnStart: true}, profile: nil, want: true},
		{name: "标签匹配", bookmark: config.BrowserBookmark{OpenOnStart: true, OpenTags: []string{"team"}}, profile: teamProfile, want: true},
		{name: "标签不匹配", bookmark: config.BrowserBookmark{OpenOnStart: true, OpenTags: []string{"Google"}}, profile: teamProfile, want: false},
		{name: "分组匹配", bookmark: config.BrowserBookmark{OpenOnStart: true, OpenGroupIds: []string{"group-google"}}, profile: groupProfile, want: true},
		{name: "分组不匹配", bookmark: config.BrowserBookmark{OpenOnStart: true, OpenGroupIds: []string{"group-other"}}, profile: groupProfile, want: false},
		{name: "标签或分组任一匹配即可", bookmark: config.BrowserBookmark{OpenOnStart: true, OpenTags: []string{"Google"}, OpenGroupIds: []string{"group-google"}}, profile: groupProfile, want: true},
		{name: "限定范围时无标签无分组的实例不打开", bookmark: config.BrowserBookmark{OpenOnStart: true, OpenTags: []string{"Team"}}, profile: plainProfile, want: false},
		{name: "限定范围时没有实例信息不打开", bookmark: config.BrowserBookmark{OpenOnStart: true, OpenTags: []string{"Team"}}, profile: nil, want: false},
		{name: "范围里只有空白等于不限", bookmark: config.BrowserBookmark{OpenOnStart: true, OpenTags: []string{"  "}}, profile: plainProfile, want: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := bookmarkOpensForProfile(tt.bookmark, tt.profile); got != tt.want {
				t.Fatalf("bookmarkOpensForProfile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBrowserDefaultStartURLsFiltersScopedBookmarksByProfile(t *testing.T) {
	t.Parallel()

	appRoot := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Browser.DefaultStartURLs = []string{}
	cfg.Browser.DefaultBookmarks = []config.BrowserBookmark{
		{Name: "ChatGPT", URL: "https://chatgpt.example/", OpenOnStart: true, OpenTags: []string{"Team"}},
		{Name: "Gmail", URL: "https://gmail.example/", OpenOnStart: true, OpenTags: []string{"Google"}},
		{Name: "通用", URL: "https://all.example/", OpenOnStart: true},
	}

	app := NewApp(appRoot)
	app.config = cfg
	app.browserMgr = internalbrowser.NewManager(cfg, appRoot)

	team := &internalbrowser.Profile{ProfileId: "p-team", Tags: []string{"Team"}}
	if got, want := app.browserDefaultStartURLs(team), []string{"https://chatgpt.example/", "https://all.example/"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Team 实例的启动页不对: got=%v want=%v", got, want)
	}
	google := &internalbrowser.Profile{ProfileId: "p-google", Tags: []string{"Google"}}
	if got, want := app.browserDefaultStartURLs(google), []string{"https://gmail.example/", "https://all.example/"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Google 实例的启动页不对: got=%v want=%v", got, want)
	}
	other := &internalbrowser.Profile{ProfileId: "p-other", Tags: []string{"Other"}}
	if got, want := app.browserDefaultStartURLs(other), []string{"https://all.example/"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("其它实例的启动页不对: got=%v want=%v", got, want)
	}
}

func TestBookmarkSaveKeepsOpenScope(t *testing.T) {
	t.Parallel()

	appRoot := t.TempDir()
	cfg := config.DefaultConfig()
	app := NewApp(appRoot)
	app.config = cfg
	app.browserMgr = internalbrowser.NewManager(cfg, appRoot)

	err := app.BookmarkSave([]config.BrowserBookmark{{
		Name:         "ChatGPT",
		URL:          "https://chatgpt.example/",
		OpenOnStart:  true,
		OpenTags:     []string{" Team ", "team", ""},
		OpenGroupIds: []string{"group-a", "group-a"},
	}})
	if err != nil {
		t.Fatalf("BookmarkSave returned error: %v", err)
	}
	item, ok := findBookmarkItemByURL(app.BookmarkList(), "https://chatgpt.example/")
	if !ok {
		t.Fatalf("保存后的书签不见了: %+v", app.BookmarkList())
	}
	if !reflect.DeepEqual(item.OpenTags, []string{"Team"}) || !reflect.DeepEqual(item.OpenGroupIds, []string{"group-a"}) {
		t.Fatalf("启动范围没有按预期保存: tags=%v groups=%v", item.OpenTags, item.OpenGroupIds)
	}
	for _, bookmark := range app.BookmarkList() {
		if bookmark.OpenTags == nil || bookmark.OpenGroupIds == nil {
			t.Fatalf("启动范围应当是空数组而不是 nil: %+v", bookmark)
		}
	}
}

func countBookmarkURLInRoot(root map[string]interface{}, url string) int {
	count := 0
	roots, ok := root["roots"].(map[string]interface{})
	if !ok {
		return count
	}
	for _, item := range roots {
		folder, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if children, ok := folder["children"].([]interface{}); ok {
			count += countBookmarkURLInNodes(children, url)
		}
	}
	return count
}

func countBookmarkURLInNodes(nodes []interface{}, url string) int {
	count := 0
	for _, item := range nodes {
		node, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if node["type"] == "url" && node["url"] == url {
			count++
		}
		if children, ok := node["children"].([]interface{}); ok {
			count += countBookmarkURLInNodes(children, url)
		}
	}
	return count
}

func findBookmarkItemByURL(items []config.BrowserBookmark, url string) (config.BrowserBookmark, bool) {
	for _, item := range items {
		if item.URL == url {
			return item, true
		}
	}
	return config.BrowserBookmark{}, false
}

func countBookmarkItemsByURL(items []config.BrowserBookmark, url string) int {
	count := 0
	for _, item := range items {
		if item.URL == url {
			count++
		}
	}
	return count
}
