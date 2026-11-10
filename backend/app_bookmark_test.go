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

func TestBookmarkListAlwaysIncludesVerificationBookmarks(t *testing.T) {
	t.Parallel()

	appRoot := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Browser.DefaultBookmarks = []config.BrowserBookmark{
		{Name: "用户默认书签", URL: "https://user.example/"},
	}

	app := NewApp(appRoot)
	app.config = cfg
	app.browserMgr = internalbrowser.NewManager(cfg, appRoot)

	bookmarks := app.BookmarkList()
	for _, url := range []string{"https://ippure.com/", "https://iplark.com/", "https://ping0.cc/"} {
		if countBookmarkItemsByURL(bookmarks, url) != 1 {
			t.Fatalf("expected verification bookmark %s exactly once, got %+v", url, bookmarks)
		}
	}
}

func TestBookmarkSavePersistsVerificationBookmarks(t *testing.T) {
	t.Parallel()

	appRoot := t.TempDir()
	cfg := config.DefaultConfig()
	app := NewApp(appRoot)
	app.config = cfg
	app.browserMgr = internalbrowser.NewManager(cfg, appRoot)

	if err := app.BookmarkSave([]config.BrowserBookmark{{Name: "用户默认书签", URL: "https://user.example/", OpenOnStart: true}}); err != nil {
		t.Fatalf("BookmarkSave returned error: %v", err)
	}
	if item, ok := findBookmarkItemByURL(app.config.Browser.DefaultBookmarks, "https://user.example/"); !ok || !item.OpenOnStart {
		t.Fatalf("expected open_on_start to be preserved, got %+v", app.config.Browser.DefaultBookmarks)
	}
	for _, url := range []string{"https://ippure.com/", "https://iplark.com/", "https://ping0.cc/"} {
		if countBookmarkItemsByURL(app.config.Browser.DefaultBookmarks, url) != 1 {
			t.Fatalf("expected saved verification bookmark %s exactly once, got %+v", url, app.config.Browser.DefaultBookmarks)
		}
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
