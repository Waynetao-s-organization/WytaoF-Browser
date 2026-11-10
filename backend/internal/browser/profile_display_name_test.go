package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleLocalState = `{"browser":{"enabled_labs_experiments":[]},"os_crypt":{"audit_enabled":true,"encrypted_key":"RFBBUEkBAAAA0Iyd3wEV0RGMegDAT8KX6wEAAAA=="},"profile":{"info_cache":{"Default":{"active_time":1784806910.219944,"avatar_icon":"chrome://theme/IDR_PROFILE_AVATAR_26","is_using_default_avatar":true,"is_using_default_name":true,"metrics_bucket_index":1,"name":"您的 Chromium","profile_color_seed":-16033840,"user_name":""}},"last_used":"Default","profile_counts_reported":"13380000000000000"},"big_number":18446744073709551615,"weird":"a\u2028b\ud83d\ude00\"}{"}`

func readJSONPath(t *testing.T, data []byte, path ...string) any {
	t.Helper()

	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("解析 JSON 失败: %v", err)
	}
	current := root
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("路径 %v 不是对象", path)
		}
		current = object[key]
	}
	return current
}

func TestPatchLocalStateProfileNameOnlyTouchesNameFields(t *testing.T) {
	t.Parallel()

	original := []byte(sampleLocalState)
	patched, changed, err := patchLocalStateProfileName(original, "样例1")
	if err != nil || !changed {
		t.Fatalf("应当改动成功: changed=%v err=%v", changed, err)
	}
	if got := readJSONPath(t, patched, "profile", "info_cache", "Default", "name"); got != "样例1" {
		t.Fatalf("账户名称没有改成实例名称，实际 %v", got)
	}
	if got := readJSONPath(t, patched, "profile", "info_cache", "Default", "is_using_default_name"); got != false {
		t.Fatalf("is_using_default_name 应当改成 false，实际 %v", got)
	}

	withoutName := strings.Replace(string(original), `"name":"您的 Chromium"`, `"name":"样例1"`, 1)
	expected := strings.Replace(withoutName, `"is_using_default_name":true`, `"is_using_default_name":false`, 1)
	if string(patched) != expected {
		t.Fatalf("除了两个名称字段，其余内容必须一个字节都不变:\n got=%s\nwant=%s", patched, expected)
	}
	if !strings.Contains(string(patched), `"encrypted_key":"RFBBUEkBAAAA0Iyd3wEV0RGMegDAT8KX6wEAAAA=="`) {
		t.Fatal("加密密钥字段被改动了")
	}
	if !strings.Contains(string(patched), `"big_number":18446744073709551615`) {
		t.Fatal("大整数被改写了")
	}
}

func TestPatchLocalStateProfileNameSkipsWhenAlreadySynced(t *testing.T) {
	t.Parallel()

	first, changed, err := patchLocalStateProfileName([]byte(sampleLocalState), "样例1")
	if err != nil || !changed {
		t.Fatalf("第一次应当改动: changed=%v err=%v", changed, err)
	}
	second, changed, err := patchLocalStateProfileName(first, "样例1")
	if err != nil || changed {
		t.Fatalf("名称已经一致时不应再改: changed=%v err=%v", changed, err)
	}
	if string(second) != string(first) {
		t.Fatal("没有改动时应当原样返回")
	}
}

func TestPatchLocalStateProfileNameEscapesSpecialCharacters(t *testing.T) {
	t.Parallel()

	name := `Team "A" \ <b>&'测试😀`
	patched, changed, err := patchLocalStateProfileName([]byte(sampleLocalState), name)
	if err != nil || !changed {
		t.Fatalf("应当改动成功: changed=%v err=%v", changed, err)
	}
	if got := readJSONPath(t, patched, "profile", "info_cache", "Default", "name"); got != name {
		t.Fatalf("特殊字符没有原样写回，实际 %v", got)
	}
}

func TestPatchLocalStateProfileNameLeavesUnexpectedFilesAlone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{name: "没有 info_cache", data: `{"profile":{"last_used":"Default"}}`},
		{name: "Default 不是对象", data: `{"profile":{"info_cache":{"Default":"x"}}}`},
		{name: "name 不是字符串", data: `{"profile":{"info_cache":{"Default":{"name":123}}}}`},
		{name: "不是合法 JSON", data: `{"profile":{"info_cache":`, wantErr: true},
		{name: "键重复", data: `{"profile":{"info_cache":{"Default":{"name":"a","name":"b"}}}}`, wantErr: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, changed, err := patchLocalStateProfileName([]byte(tt.data), "样例1")
			if changed {
				t.Fatal("结构不符合预期的文件不应改动")
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestPatchLocalStateProfileNameKeepsFlagWhenAbsent(t *testing.T) {
	t.Parallel()

	data := []byte(`{ "profile" : { "info_cache" : { "Default" : { "name" : "工作" , "avatar_icon" : "x" } } } }`)
	patched, changed, err := patchLocalStateProfileName(data, "样例1")
	if err != nil || !changed {
		t.Fatalf("应当改动成功: changed=%v err=%v", changed, err)
	}
	want := `{ "profile" : { "info_cache" : { "Default" : { "name" : "样例1" , "avatar_icon" : "x" } } } }`
	if string(patched) != want {
		t.Fatalf("带空白的写法也只应替换名称:\n got=%s\nwant=%s", patched, want)
	}
}

func TestEnsureProfileDisplayNamePatchesExistingLocalState(t *testing.T) {
	t.Parallel()

	userDataDir := t.TempDir()
	localStatePath := filepath.Join(userDataDir, "Local State")
	if err := os.WriteFile(localStatePath, []byte(sampleLocalState), 0o644); err != nil {
		t.Fatalf("写入 Local State 失败: %v", err)
	}

	changed, err := EnsureProfileDisplayName(userDataDir, "  办公室\n自用1 ")
	if err != nil || !changed {
		t.Fatalf("应当改动成功: changed=%v err=%v", changed, err)
	}
	data, err := os.ReadFile(localStatePath)
	if err != nil {
		t.Fatalf("读取 Local State 失败: %v", err)
	}
	if got := readJSONPath(t, data, "profile", "info_cache", "Default", "name"); got != "办公室 自用1" {
		t.Fatalf("账户名称不对，实际 %v", got)
	}
	if _, err := os.Stat(filepath.Join(userDataDir, "Default", "Preferences")); !os.IsNotExist(err) {
		t.Fatal("已经启动过的实例不应再生成 Preferences")
	}
	entries, err := os.ReadDir(userDataDir)
	if err != nil {
		t.Fatalf("读取目录失败: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("临时文件没有清理: %s", entry.Name())
		}
	}

	changed, err = EnsureProfileDisplayName(userDataDir, "办公室 自用1")
	if err != nil || changed {
		t.Fatalf("名称一致时不应再写文件: changed=%v err=%v", changed, err)
	}

	backupPath := localStatePath + LocalStateBackupSuffix
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("第一次改之前应当留一份原件: %v", err)
	}
	if string(backup) != sampleLocalState {
		t.Fatal("留下的原件和改之前的内容不一致")
	}

	changed, err = EnsureProfileDisplayName(userDataDir, "又改了一次")
	if err != nil || !changed {
		t.Fatalf("再次改名应当成功: changed=%v err=%v", changed, err)
	}
	backup, _ = os.ReadFile(backupPath)
	if string(backup) != sampleLocalState {
		t.Fatal("再次改名不应覆盖最早留下的原件")
	}
}

func TestEnsureProfileDisplayNameGivesUpWhenBackupFails(t *testing.T) {
	t.Parallel()

	userDataDir := t.TempDir()
	localStatePath := filepath.Join(userDataDir, "Local State")
	if err := os.WriteFile(localStatePath, []byte(sampleLocalState), 0o644); err != nil {
		t.Fatalf("写入 Local State 失败: %v", err)
	}
	if err := os.Mkdir(localStatePath+LocalStateBackupSuffix, 0o755); err != nil {
		t.Fatalf("准备占位目录失败: %v", err)
	}

	changed, err := EnsureProfileDisplayName(userDataDir, "样例1")
	if changed || err == nil {
		t.Fatalf("留不下原件时应当报错并且不修改: changed=%v err=%v", changed, err)
	}
	data, _ := os.ReadFile(localStatePath)
	if string(data) != sampleLocalState {
		t.Fatal("留不下原件时 Local State 不应被改动")
	}
}

func TestEnsureProfileDisplayNameSeedsPreferencesForNewProfile(t *testing.T) {
	t.Parallel()

	userDataDir := t.TempDir()
	changed, err := EnsureProfileDisplayName(userDataDir, "样例1")
	if err != nil || !changed {
		t.Fatalf("新实例应当预置账户名称: changed=%v err=%v", changed, err)
	}
	data, err := os.ReadFile(filepath.Join(userDataDir, "Default", "Preferences"))
	if err != nil {
		t.Fatalf("读取 Preferences 失败: %v", err)
	}
	if got := readJSONPath(t, data, "profile", "name"); got != "样例1" {
		t.Fatalf("预置的账户名称不对，实际 %v", got)
	}
	if got := readJSONPath(t, data, "profile", "using_default_name"); got != false {
		t.Fatalf("using_default_name 应当是 false，实际 %v", got)
	}

	localState, err := os.ReadFile(filepath.Join(userDataDir, "Local State"))
	if err != nil {
		t.Fatalf("新实例应当一并预置 Local State: %v", err)
	}
	if got := readJSONPath(t, localState, "browser", "first_run_finished"); got != true {
		t.Fatalf("Local State 里应当标记首次运行已完成，实际 %v", got)
	}
	if string(localState) != `{"browser":{"first_run_finished":true}}` {
		t.Fatalf("预置的 Local State 不应带别的内容: %s", localState)
	}

	changed, err = EnsureProfileDisplayName(userDataDir, "样例1")
	if err != nil || changed {
		t.Fatalf("浏览器还没启动过、Local State 里没有账户信息时不应再改: changed=%v err=%v", changed, err)
	}
	again, _ := os.ReadFile(filepath.Join(userDataDir, "Local State"))
	if string(again) != string(localState) {
		t.Fatalf("第二次调用不应改动预置的 Local State: %s", again)
	}
}

func TestEnsureProfileDisplayNameNeverOverwritesExistingPreferences(t *testing.T) {
	t.Parallel()

	userDataDir := t.TempDir()
	prefsPath := filepath.Join(userDataDir, "Default", "Preferences")
	if err := os.MkdirAll(filepath.Dir(prefsPath), 0o755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	const existing = `{"profile":{"name":"原来的","exit_type":"Normal"}}`
	if err := os.WriteFile(prefsPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("写入 Preferences 失败: %v", err)
	}

	changed, err := EnsureProfileDisplayName(userDataDir, "样例1")
	if err != nil || changed {
		t.Fatalf("已有 Preferences 时不应改动: changed=%v err=%v", changed, err)
	}
	data, _ := os.ReadFile(prefsPath)
	if string(data) != existing {
		t.Fatalf("Preferences 被改动了: %s", data)
	}
	if _, err := os.Stat(filepath.Join(userDataDir, "Local State")); !os.IsNotExist(err) {
		t.Fatal("已有 Preferences 时不应生成 Local State")
	}
}

func TestEnsureProfileDisplayNameIgnoresBlankName(t *testing.T) {
	t.Parallel()

	userDataDir := t.TempDir()
	changed, err := EnsureProfileDisplayName(userDataDir, " \t\n ")
	if err != nil || changed {
		t.Fatalf("空名称不应写任何文件: changed=%v err=%v", changed, err)
	}
	entries, _ := os.ReadDir(userDataDir)
	if len(entries) != 0 {
		t.Fatalf("空名称不应生成文件，实际 %d 个", len(entries))
	}
}
