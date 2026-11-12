package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// BuildAcceptLanguageList 根据主语言标签生成浏览器使用的语言列表（例如 "en-US" -> "en-US,en"）
func BuildAcceptLanguageList(rawLang string) string {
	trimmed := strings.TrimSpace(rawLang)
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, ",")
	out := make([]string, 0, len(parts)+1)
	seen := make(map[string]struct{}, len(parts)+1)
	add := func(tag string) {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			return
		}
		key := strings.ToLower(tag)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		out = append(out, tag)
	}
	for _, part := range parts {
		// 去掉可能附带的 ;q=0.9 权重后缀
		if semi := strings.IndexByte(part, ';'); semi >= 0 {
			part = part[:semi]
		}
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		add(part)
		if dash := strings.IndexByte(part, '-'); dash > 0 {
			base := part[:dash]
			if !strings.EqualFold(base, "zh") || strings.EqualFold(part, "zh-CN") {
				add(strings.ToLower(base))
			}
		}
	}
	return strings.Join(out, ",")
}

// ExtractConfiguredLanguage 从指纹参数与启动参数中提取最后生效的语言配置（支持 --accept-lang= 与 --lang=）
func ExtractConfiguredLanguage(argGroups ...[]string) string {
	lang := ""
	acceptLang := ""
	for _, group := range argGroups {
		for _, raw := range group {
			arg := strings.TrimSpace(raw)
			lower := strings.ToLower(arg)
			switch {
			case strings.HasPrefix(lower, "--accept-lang="):
				val := strings.TrimSpace(arg[len("--accept-lang="):])
				if val != "" {
					acceptLang = val
				}
			case strings.HasPrefix(lower, "--accept-language="):
				val := strings.TrimSpace(arg[len("--accept-language="):])
				if val != "" {
					acceptLang = val
				}
			case strings.HasPrefix(lower, "--lang="):
				val := strings.TrimSpace(arg[len("--lang="):])
				if val != "" {
					lang = val
				}
			}
		}
	}
	if acceptLang != "" {
		return BuildAcceptLanguageList(acceptLang)
	}
	return BuildAcceptLanguageList(lang)
}

// SyncProfileLanguagePreferences 在实例启动前将语言设置同步到 Default/Preferences，
// 解决实例首次启动写入 intl.selected_languages 后，后续修改 --lang 不再生效的问题。
func SyncProfileLanguagePreferences(userDataDir string, configuredLangList string) (bool, error) {
	if strings.TrimSpace(userDataDir) == "" {
		return false, nil
	}
	if userDataDirInUse(userDataDir) {
		return false, nil
	}

	prefsPath := filepath.Join(userDataDir, "Default", "Preferences")
	desired := BuildAcceptLanguageList(configuredLangList)

	data, err := os.ReadFile(prefsPath)
	if os.IsNotExist(err) {
		if desired == "" {
			return false, nil
		}
		if err := os.MkdirAll(filepath.Dir(prefsPath), 0o755); err != nil {
			return false, err
		}
		initial := map[string]any{
			"intl": map[string]any{
				"accept_languages":   desired,
				"selected_languages": desired,
			},
			"wytaof": map[string]any{
				"managed_lang": desired,
			},
		}
		encoded, err := json.Marshal(initial)
		if err != nil {
			return false, err
		}
		return true, writeFileAtomic(prefsPath, encoded, 0o644)
	}
	if err != nil {
		return false, err
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil || root == nil {
		return false, nil
	}

	intlMap, _ := root["intl"].(map[string]any)
	wytaofMap, _ := root["wytaof"].(map[string]any)

	if desired == "" {
		// 用户未指定语言（或从有语言改回「不设置」）：仅当之前由软件写入过语言时清理，恢复默认
		if wytaofMap == nil {
			return false, nil
		}
		prevManaged, _ := wytaofMap["managed_lang"].(string)
		if strings.TrimSpace(prevManaged) == "" {
			return false, nil
		}
		delete(wytaofMap, "managed_lang")
		if len(wytaofMap) == 0 {
			delete(root, "wytaof")
		} else {
			root["wytaof"] = wytaofMap
		}
		if intlMap != nil {
			delete(intlMap, "accept_languages")
			delete(intlMap, "selected_languages")
			if len(intlMap) == 0 {
				delete(root, "intl")
			} else {
				root["intl"] = intlMap
			}
		}
		encoded, err := json.Marshal(root)
		if err != nil {
			return false, err
		}
		return true, writeFileAtomic(prefsPath, encoded, 0o644)
	}

	if intlMap == nil {
		intlMap = make(map[string]any)
	}
	curAccept, _ := intlMap["accept_languages"].(string)
	curSelected, _ := intlMap["selected_languages"].(string)
	curManaged := ""
	if wytaofMap != nil {
		curManaged, _ = wytaofMap["managed_lang"].(string)
	}
	if curAccept == desired && curSelected == desired && curManaged == desired {
		return false, nil
	}

	intlMap["accept_languages"] = desired
	intlMap["selected_languages"] = desired
	root["intl"] = intlMap

	if wytaofMap == nil {
		wytaofMap = make(map[string]any)
	}
	wytaofMap["managed_lang"] = desired
	root["wytaof"] = wytaofMap

	encoded, err := json.Marshal(root)
	if err != nil {
		return false, err
	}
	return true, writeFileAtomic(prefsPath, encoded, 0o644)
}
