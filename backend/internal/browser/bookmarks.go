package browser

import (
	"ant-chrome/backend/internal/config"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// chromiumEpoch 是 Chrome FILETIME 的起始时间（1601-01-01 UTC）
var chromiumEpoch = time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC)

var builtInDefaultBookmarkURLs = []string{
	"https://www.google.com/",
	"https://mail.google.com/",
	"https://claude.ai/",
	"https://chatgpt.com/",
	"https://www.youtube.com/",
	"https://ippure.com/",
	"https://iplark.com/",
	"https://ping0.cc/",
}

func toChromiumTime(t time.Time) string {
	return fmt.Sprintf("%d", t.Sub(chromiumEpoch).Microseconds())
}

// NormalizeBookmarkURLKey 将书签 URL 统一成可比对的键（忽略协议/主机大小写与根路径末尾斜杠差异）
func NormalizeBookmarkURLKey(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.ToLower(trimmed)
	}
	scheme := strings.ToLower(parsed.Scheme)
	host := strings.ToLower(parsed.Host)
	path := parsed.EscapedPath()
	if (scheme == "http" || scheme == "https") && (path == "" || path == "/") && parsed.RawQuery == "" && parsed.Fragment == "" {
		return scheme + "://" + host + "/"
	}
	rebuilt := scheme + "://" + host + path
	if parsed.RawQuery != "" {
		rebuilt += "?" + parsed.RawQuery
	}
	if parsed.Fragment != "" {
		rebuilt += "#" + parsed.Fragment
	}
	return rebuilt
}

// EnsureDefaultBookmarks 将默认书签同步到书签栏
func EnsureDefaultBookmarks(userDataDir string, bookmarks []config.BrowserBookmark) error {
	return SyncDefaultBookmarks(userDataDir, bookmarks, nil)
}

// SyncDefaultBookmarks 将默认书签同步到实例书签栏（支持顺序调整、名称更新、移除已删默认书签，同时保留用户自建书签）
func SyncDefaultBookmarks(userDataDir string, bookmarks []config.BrowserBookmark, removedBookmarks []config.BrowserBookmark) error {
	profileDir := filepath.Join(userDataDir, "Default")
	bookmarksPath := filepath.Join(profileDir, "Bookmarks")

	// 尝试读取已有书签文件
	var root map[string]interface{}
	fileExisted := false
	if data, err := os.ReadFile(bookmarksPath); err == nil {
		fileExisted = true
		_ = json.Unmarshal(data, &root)
	}

	validBookmarks := normalizeBookmarkItems(bookmarks)
	if len(validBookmarks) == 0 && !fileExisted {
		return nil
	}

	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return fmt.Errorf("创建 profile 目录失败: %w", err)
	}

	now := toChromiumTime(time.Now())

	// 初始化空结构
	if root == nil {
		root = newEmptyBookmarkRoot(now)
	}

	barChildren := extractBarChildren(root)
	nonBarURLs := collectNonBarTopURLs(root)
	maxID := findMaxID(root)

	targetKeys := make(map[string]config.BrowserBookmark, len(validBookmarks))
	for _, b := range validBookmarks {
		targetKeys[NormalizeBookmarkURLKey(b.URL)] = b
	}

	knownManagedKeys := make(map[string]bool, len(validBookmarks)+len(removedBookmarks)+len(builtInDefaultBookmarkURLs))
	for k := range targetKeys {
		knownManagedKeys[k] = true
	}
	for _, b := range removedBookmarks {
		if k := NormalizeBookmarkURLKey(b.URL); k != "" {
			knownManagedKeys[k] = true
		}
	}
	for _, rawURL := range builtInDefaultBookmarkURLs {
		if k := NormalizeBookmarkURLKey(rawURL); k != "" {
			knownManagedKeys[k] = true
		}
	}

	existingManagedByKey := make(map[string]map[string]interface{}, len(barChildren))
	userNodes := make([]interface{}, 0, len(barChildren))

	for _, rawNode := range barChildren {
		node, ok := rawNode.(map[string]interface{})
		if !ok {
			continue
		}
		if nodeType, _ := node["type"].(string); nodeType != "url" {
			userNodes = append(userNodes, node)
			continue
		}
		nodeURL, _ := node["url"].(string)
		key := NormalizeBookmarkURLKey(nodeURL)
		if isManagedBookmarkNode(node, key, knownManagedKeys) {
			if key != "" {
				if _, exists := existingManagedByKey[key]; !exists {
					existingManagedByKey[key] = node
				}
			}
			continue
		}
		userNodes = append(userNodes, node)
	}

	managedNodes := make([]interface{}, 0, len(validBookmarks))
	for _, b := range validBookmarks {
		key := NormalizeBookmarkURLKey(b.URL)
		if key == "" {
			continue
		}
		// 如果用户把该默认书签放到了其它书签文件夹里（不在书签栏顶层），不重复往书签栏顶层塞
		if nonBarURLs[key] {
			if _, inBar := existingManagedByKey[key]; !inBar {
				continue
			}
		}
		if existing, ok := existingManagedByKey[key]; ok {
			cloned := cloneBookmarkNode(existing)
			cloned["name"] = b.Name
			if existingURL, _ := cloned["url"].(string); NormalizeBookmarkURLKey(existingURL) != key || strings.TrimSpace(existingURL) == "" {
				cloned["url"] = b.URL
			}
			markNodeAsManaged(cloned)
			managedNodes = append(managedNodes, cloned)
			continue
		}
		maxID++
		managedNodes = append(managedNodes, map[string]interface{}{
			"date_added":     now,
			"date_last_used": "0",
			"guid":           bookmarkGUID(b.URL),
			"id":             fmt.Sprintf("%d", maxID),
			"meta_info":      map[string]string{"power_bookmark_meta": "", "wytaof_default": "1"},
			"name":           b.Name,
			"type":           "url",
			"url":            b.URL,
		})
	}

	newBarChildren := make([]interface{}, 0, len(managedNodes)+len(userNodes))
	newBarChildren = append(newBarChildren, managedNodes...)
	newBarChildren = append(newBarChildren, userNodes...)

	if fileExisted && bookmarkSliceEqual(barChildren, newBarChildren) {
		return nil
	}

	// 写回并清空旧 checksum，让 Chrome 启动时按新书签重算
	root["checksum"] = ""
	roots := root["roots"].(map[string]interface{})
	bar := roots["bookmark_bar"].(map[string]interface{})
	bar["children"] = newBarChildren
	bar["date_modified"] = now
	roots["bookmark_bar"] = bar
	root["roots"] = roots

	out, err := json.MarshalIndent(root, "", "   ")
	if err != nil {
		return fmt.Errorf("序列化书签失败: %w", err)
	}
	if err := os.WriteFile(bookmarksPath, out, 0644); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(profileDir, "Bookmarks.bak"))
	return nil
}

func normalizeBookmarkItems(bookmarks []config.BrowserBookmark) []config.BrowserBookmark {
	out := make([]config.BrowserBookmark, 0, len(bookmarks))
	seen := make(map[string]struct{}, len(bookmarks))
	for _, item := range bookmarks {
		name := strings.TrimSpace(item.Name)
		rawURL := strings.TrimSpace(item.URL)
		if name == "" || rawURL == "" {
			continue
		}
		key := NormalizeBookmarkURLKey(rawURL)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, config.BrowserBookmark{
			Name:         name,
			URL:          rawURL,
			OpenOnStart:  item.OpenOnStart,
			OpenTags:     NormalizeBookmarkScope(item.OpenTags),
			OpenGroupIds: NormalizeBookmarkScope(item.OpenGroupIds),
		})
	}
	return out
}

func isManagedBookmarkNode(node map[string]interface{}, normalizedKey string, knownManagedKeys map[string]bool) bool {
	if normalizedKey != "" && knownManagedKeys[normalizedKey] {
		return true
	}
	if hasManagedMetaFlag(node["meta_info"]) {
		return true
	}
	guid, _ := node["guid"].(string)
	nodeURL, _ := node["url"].(string)
	return isWytaoFBookmarkGUID(guid, nodeURL)
}

func hasManagedMetaFlag(meta interface{}) bool {
	switch m := meta.(type) {
	case map[string]interface{}:
		if v, ok := m["wytaof_default"].(string); ok && v == "1" {
			return true
		}
	case map[string]string:
		if m["wytaof_default"] == "1" {
			return true
		}
	}
	return false
}

func markNodeAsManaged(node map[string]interface{}) {
	switch m := node["meta_info"].(type) {
	case map[string]interface{}:
		m["wytaof_default"] = "1"
	case map[string]string:
		m["wytaof_default"] = "1"
	default:
		node["meta_info"] = map[string]string{"power_bookmark_meta": "", "wytaof_default": "1"}
	}
}

func cloneBookmarkNode(node map[string]interface{}) map[string]interface{} {
	cloned := make(map[string]interface{}, len(node))
	for k, v := range node {
		if k == "meta_info" {
			switch m := v.(type) {
			case map[string]interface{}:
				cm := make(map[string]interface{}, len(m)+1)
				for mk, mv := range m {
					cm[mk] = mv
				}
				cloned[k] = cm
				continue
			case map[string]string:
				cm := make(map[string]string, len(m)+1)
				for mk, mv := range m {
					cm[mk] = mv
				}
				cloned[k] = cm
				continue
			}
		}
		cloned[k] = v
	}
	return cloned
}

// isWytaoFBookmarkGUID 判断 GUID 是否由 bookmarkGUID 生成。
// bookmarkGUID 的格式为 %08x-%04x-%04x-%04x-%012x，其中第 5 段（12 位 hex）恰好等于 第 2 段（4 位）+ 第 1 段（8 位）。
func isWytaoFBookmarkGUID(guid string, rawURL string) bool {
	guid = strings.ToLower(strings.TrimSpace(guid))
	if len(guid) != 36 {
		return false
	}
	if rawURL != "" {
		trimmed := strings.TrimSpace(rawURL)
		if guid == bookmarkGUID(trimmed) || guid == bookmarkGUID(strings.TrimSuffix(trimmed, "/")) || guid == bookmarkGUID(trimmed+"/") {
			return true
		}
	}
	parts := strings.Split(guid, "-")
	if len(parts) != 5 || len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[4]) != 12 {
		return false
	}
	return parts[4] == parts[1]+parts[0]
}

func bookmarkSliceEqual(oldNodes []interface{}, newNodes []interface{}) bool {
	if len(oldNodes) != len(newNodes) {
		return false
	}
	for i := range oldNodes {
		oldMap, ok1 := oldNodes[i].(map[string]interface{})
		newMap, ok2 := newNodes[i].(map[string]interface{})
		if !ok1 || !ok2 {
			return false
		}
		if strField(oldMap, "type") != strField(newMap, "type") ||
			strField(oldMap, "id") != strField(newMap, "id") ||
			strField(oldMap, "name") != strField(newMap, "name") ||
			strField(oldMap, "url") != strField(newMap, "url") {
			return false
		}
	}
	return true
}

func strField(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

// newEmptyBookmarkRoot 构建一个空的书签根结构
func newEmptyBookmarkRoot(now string) map[string]interface{} {
	return map[string]interface{}{
		"checksum": "",
		"version":  1,
		"roots": map[string]interface{}{
			"bookmark_bar": map[string]interface{}{
				"children":       []interface{}{},
				"date_added":     now,
				"date_last_used": "0",
				"date_modified":  now,
				"guid":           "0bc5d13f-2cba-5d74-951f-3f233fe6c908",
				"id":             "1",
				"name":           "书签栏",
				"type":           "folder",
			},
			"other": map[string]interface{}{
				"children":       []interface{}{},
				"date_added":     now,
				"date_last_used": "0",
				"date_modified":  "0",
				"guid":           "82b081ec-3dd3-529c-8475-ab6c344590dd",
				"id":             "2",
				"name":           "其他书签",
				"type":           "folder",
			},
			"synced": map[string]interface{}{
				"children":       []interface{}{},
				"date_added":     now,
				"date_last_used": "0",
				"date_modified":  "0",
				"guid":           "4cf2e351-0e85-532b-bb37-df045d8f8d0f",
				"id":             "3",
				"name":           "移动设备书签",
				"type":           "folder",
			},
		},
	}
}

// extractBarChildren 从根结构中提取书签栏 children
func extractBarChildren(root map[string]interface{}) []interface{} {
	var children []interface{}

	roots, ok := root["roots"].(map[string]interface{})
	if !ok {
		root["roots"] = map[string]interface{}{
			"bookmark_bar": map[string]interface{}{
				"children": []interface{}{},
				"type":     "folder",
				"name":     "书签栏",
			},
		}
		return children
	}

	bar, ok := roots["bookmark_bar"].(map[string]interface{})
	if !ok {
		roots["bookmark_bar"] = map[string]interface{}{
			"children": []interface{}{},
			"type":     "folder",
			"name":     "书签栏",
		}
		root["roots"] = roots
		return children
	}

	if c, ok := bar["children"].([]interface{}); ok {
		children = c
	}
	return children
}

func collectNonBarTopURLs(root map[string]interface{}) map[string]bool {
	existing := map[string]bool{}
	roots, ok := root["roots"].(map[string]interface{})
	if !ok {
		return existing
	}
	for rootKey, item := range roots {
		folder, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		children, ok := folder["children"].([]interface{})
		if !ok {
			continue
		}
		if rootKey == "bookmark_bar" {
			for _, n := range children {
				node, ok := n.(map[string]interface{})
				if !ok || node["type"] != "folder" {
					continue
				}
				if sub, ok := node["children"].([]interface{}); ok {
					collectURLs(sub, existing)
				}
			}
			continue
		}
		collectURLs(children, existing)
	}
	return existing
}

// collectURLs 递归收集所有书签 URL（统一格式后的键）
func collectURLs(nodes []interface{}, out map[string]bool) {
	for _, n := range nodes {
		node, ok := n.(map[string]interface{})
		if !ok {
			continue
		}
		if node["type"] == "url" {
			if u, ok := node["url"].(string); ok {
				if key := NormalizeBookmarkURLKey(u); key != "" {
					out[key] = true
				}
			}
		} else if node["type"] == "folder" {
			if sub, ok := node["children"].([]interface{}); ok {
				collectURLs(sub, out)
			}
		}
	}
}

// findMaxID 遍历整个书签树找到最大数字 id
func findMaxID(root map[string]interface{}) int {
	max := 0
	roots, ok := root["roots"].(map[string]interface{})
	if !ok {
		return max
	}
	for _, v := range roots {
		if folder, ok := v.(map[string]interface{}); ok {
			scanMaxID(folder, &max)
		}
	}
	return max
}

func scanMaxID(node map[string]interface{}, max *int) {
	if idStr, ok := node["id"].(string); ok {
		var n int
		fmt.Sscanf(idStr, "%d", &n)
		if n > *max {
			*max = n
		}
	}
	if children, ok := node["children"].([]interface{}); ok {
		for _, c := range children {
			if child, ok := c.(map[string]interface{}); ok {
				scanMaxID(child, max)
			}
		}
	}
}

// bookmarkGUID 根据 URL 生成稳定伪 GUID
func bookmarkGUID(url string) string {
	h := uint64(14695981039346656037)
	for _, c := range url {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		h&0xffffffff, (h>>32)&0xffff,
		(h>>48)&0x0fff|0x4000,
		(h>>16)&0x3fff|0x8000,
		h&0xffffffffffff,
	)
}
