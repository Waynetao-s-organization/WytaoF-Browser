package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildAcceptLanguageList(t *testing.T) {
	t.Parallel()

	if got := BuildAcceptLanguageList("en-US"); got != "en-US,en" {
		t.Fatalf("en-US 转换结果不对: got=%q", got)
	}
	if got := BuildAcceptLanguageList("zh-CN"); got != "zh-CN,zh" {
		t.Fatalf("zh-CN 转换结果不对: got=%q", got)
	}
	if got := BuildAcceptLanguageList("zh-TW"); got != "zh-TW" {
		t.Fatalf("zh-TW 不应追加泛 zh 以免变简体: got=%q", got)
	}
	if got := BuildAcceptLanguageList("ja-JP"); got != "ja-JP,ja" {
		t.Fatalf("ja-JP 转换结果不对: got=%q", got)
	}
}

func TestSyncProfileLanguagePreferencesOverridesExistingAndClearsWhenUnset(t *testing.T) {
	t.Parallel()

	userDataDir := t.TempDir()
	prefsDir := filepath.Join(userDataDir, "Default")
	if err := os.MkdirAll(prefsDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	prefsPath := filepath.Join(prefsDir, "Preferences")

	// 模拟首次启动未设语言时，浏览器自动写入了 zh-CN,zh 以及其它用户设置
	initial := map[string]any{
		"profile": map[string]any{"name": "样例1"},
		"intl":    map[string]any{"selected_languages": "zh-CN,zh"},
	}
	raw, _ := json.Marshal(initial)
	if err := os.WriteFile(prefsPath, raw, 0o644); err != nil {
		t.Fatalf("write initial prefs failed: %v", err)
	}

	// 后续修改语言为 en-US
	changed, err := SyncProfileLanguagePreferences(userDataDir, "en-US")
	if err != nil || !changed {
		t.Fatalf("期望更新 Preferences 成功: changed=%v err=%v", changed, err)
	}

	updatedRaw, err := os.ReadFile(prefsPath)
	if err != nil {
		t.Fatalf("read updated prefs failed: %v", err)
	}
	var updated map[string]any
	if err := json.Unmarshal(updatedRaw, &updated); err != nil {
		t.Fatalf("unmarshal updated prefs failed: %v", err)
	}
	intlMap := updated["intl"].(map[string]any)
	if intlMap["selected_languages"] != "en-US,en" || intlMap["accept_languages"] != "en-US,en" {
		t.Fatalf("语言未同步更新: %+v", intlMap)
	}
	if updated["profile"].(map[string]any)["name"] != "样例1" {
		t.Fatalf("不应丢失原有的其它 Preferences 字段: %+v", updated)
	}

	// 相同配置再次调用不应重复写文件
	changed, err = SyncProfileLanguagePreferences(userDataDir, "en-US")
	if err != nil || changed {
		t.Fatalf("相同配置不应重写: changed=%v err=%v", changed, err)
	}

	// 用户把语言改回「不设置」时，应清掉之前写入的强制语言
	changed, err = SyncProfileLanguagePreferences(userDataDir, "")
	if err != nil || !changed {
		t.Fatalf("取消语言设置应清理受管字段: changed=%v err=%v", changed, err)
	}
	clearedRaw, _ := os.ReadFile(prefsPath)
	var cleared map[string]any
	_ = json.Unmarshal(clearedRaw, &cleared)
	if _, hasIntl := cleared["intl"]; hasIntl {
		t.Fatalf("清理后不应残留强制 intl 语言: %+v", cleared)
	}
}
