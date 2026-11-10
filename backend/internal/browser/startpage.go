package browser

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

//go:embed assets/start_page.html
var startPageTemplate string

// StartPageFileName 环境检测页在实例数据目录里的文件名
const StartPageFileName = "wytaof-start.html"

// StartPageData 写进环境检测页的实例信息，不包含代理账号密码等敏感内容
type StartPageData struct {
	AppName         string   `json:"appName"`
	AppVersion      string   `json:"appVersion"`
	ProfileID       string   `json:"profileId"`
	ProfileName     string   `json:"profileName"`
	Tags            []string `json:"tags"`
	GroupName       string   `json:"groupName"`
	LaunchCode      string   `json:"launchCode"`
	CoreName        string   `json:"coreName"`
	CoreVersion     string   `json:"coreVersion"`
	ProxyName       string   `json:"proxyName"`
	ProxyType       string   `json:"proxyType"`
	ProxyAddress    string   `json:"proxyAddress"`
	ProxyDirect     bool     `json:"proxyDirect"`
	FingerprintArgs []string `json:"fingerprintArgs"`
	GeneratedAt     string   `json:"generatedAt"`
}

// RenderStartPage 生成环境检测页的完整 HTML
func RenderStartPage(data StartPageData) ([]byte, error) {
	if data.Tags == nil {
		data.Tags = []string{}
	}
	if data.FingerprintArgs == nil {
		data.FingerprintArgs = []string{}
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("环境检测页数据序列化失败: %w", err)
	}
	title := strings.TrimSpace(data.ProfileName)
	if title == "" {
		title = "未命名实例"
	}
	replacer := strings.NewReplacer(
		"__WYTAOF_TITLE__", html.EscapeString(title+" · 环境检测"),
		"__WYTAOF_DATA__", string(payload),
	)
	return []byte(replacer.Replace(startPageTemplate)), nil
}

// WriteStartPage 把环境检测页写进实例数据目录，返回可以直接交给浏览器打开的地址
func WriteStartPage(userDataDir string, data StartPageData) (string, error) {
	userDataDir = strings.TrimSpace(userDataDir)
	if userDataDir == "" {
		return "", fmt.Errorf("实例数据目录为空")
	}
	content, err := RenderStartPage(data)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		return "", err
	}
	pagePath := filepath.Join(userDataDir, StartPageFileName)
	if err := writeFileAtomic(pagePath, content, 0o644); err != nil {
		return "", err
	}
	return FileURLFromPath(pagePath), nil
}

// FileURLFromPath 把本地路径转成 file:// 地址，空格和中文按 URL 规则转义
func FileURLFromPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	slashed := filepath.ToSlash(path)
	result := url.URL{Scheme: "file"}
	if strings.HasPrefix(slashed, "//") {
		host, rest, _ := strings.Cut(strings.TrimPrefix(slashed, "//"), "/")
		result.Host = host
		result.Path = "/" + rest
	} else {
		if !strings.HasPrefix(slashed, "/") {
			slashed = "/" + slashed
		}
		result.Path = slashed
	}
	return result.String()
}

// SameFileURL 判断两个 file:// 地址是不是同一个文件（忽略大小写和转义写法的差别）
func SameFileURL(a string, b string) bool {
	normalize := func(raw string) string {
		parsed, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || !strings.EqualFold(parsed.Scheme, "file") {
			return ""
		}
		return strings.ToLower(parsed.Host + parsed.Path)
	}
	left, right := normalize(a), normalize(b)
	return left != "" && left == right
}
