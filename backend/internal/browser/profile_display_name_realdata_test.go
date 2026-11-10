package browser

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func decodeJSONKeepingNumbers(t *testing.T, data []byte) map[string]any {
	t.Helper()

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		t.Fatalf("解析 JSON 失败: %v", err)
	}
	return root
}

func TestPatchLocalStateProfileNameOnRealProfiles(t *testing.T) {
	roots := strings.TrimSpace(os.Getenv("WYTAOF_REAL_PROFILE_ROOTS"))
	if roots == "" {
		t.Skip("真实数据只读测试：把环境变量 WYTAOF_REAL_PROFILE_ROOTS 设成实例数据目录（多个用分号隔开）后才会跑")
	}

	const newName = `核对用名称 "A" \ 样例1`
	total, changed, untouched := 0, 0, 0
	for _, root := range strings.Split(roots, ";") {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatalf("读取目录失败 %s: %v", root, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			original, err := os.ReadFile(filepath.Join(root, entry.Name(), "Local State"))
			if err != nil {
				continue
			}
			total++

			patched, ok, err := patchLocalStateProfileName(original, newName)
			if err != nil {
				t.Errorf("%s: 返回错误 %v", entry.Name(), err)
				continue
			}
			if !ok {
				untouched++
				continue
			}
			changed++

			if !json.Valid(patched) {
				t.Errorf("%s: 改完不是合法 JSON", entry.Name())
				continue
			}
			want := decodeJSONKeepingNumbers(t, original)
			got := decodeJSONKeepingNumbers(t, patched)
			profile := want["profile"].(map[string]any)["info_cache"].(map[string]any)["Default"].(map[string]any)
			profile["name"] = newName
			if _, exists := profile["is_using_default_name"]; exists {
				profile["is_using_default_name"] = false
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s: 除了账户名称以外还有别的内容变了", entry.Name())
			}

			again, okAgain, err := patchLocalStateProfileName(patched, newName)
			if err != nil || okAgain || !bytes.Equal(again, patched) {
				t.Errorf("%s: 第二次处理不应再改动 (changed=%v err=%v)", entry.Name(), okAgain, err)
			}
		}
	}
	t.Logf("真实 Local State 文件 %d 个：能改名 %d 个，结构不符合预期原样跳过 %d 个", total, changed, untouched)
	if total == 0 {
		t.Fatal("没有找到任何 Local State 文件，检查目录是否正确")
	}
}
