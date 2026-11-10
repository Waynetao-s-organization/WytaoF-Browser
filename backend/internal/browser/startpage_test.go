package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"
	"testing"
)

var startPageDataBlock = regexp.MustCompile(`(?s)<script id="wytaof-data" type="application/json">(.*?)</script>`)

func extractStartPageData(t *testing.T, page []byte) StartPageData {
	t.Helper()

	match := startPageDataBlock.FindSubmatch(page)
	if match == nil {
		t.Fatal("页面里没有找到数据块")
	}
	var data StartPageData
	if err := json.Unmarshal(match[1], &data); err != nil {
		t.Fatalf("数据块不是合法 JSON: %v\n%s", err, match[1])
	}
	return data
}

func TestRenderStartPageEmbedsProfileData(t *testing.T) {
	t.Parallel()

	page, err := RenderStartPage(StartPageData{
		AppName:      "WytaoF Browser",
		ProfileID:    "profile-1",
		ProfileName:  "样例1",
		Tags:         []string{"样例"},
		LaunchCode:   "AB12CD",
		CoreName:     "Chrome Chrom-144",
		ProxyName:    "SOCKS5-127.0.0.1:7890",
		ProxyType:    "SOCKS5",
		ProxyAddress: "127.0.0.1:7890",
	})
	if err != nil {
		t.Fatalf("RenderStartPage 返回错误: %v", err)
	}
	if strings.Contains(string(page), "__WYTAOF_") {
		t.Fatal("页面里还有没替换的占位符")
	}
	if !strings.Contains(string(page), "<title>样例1 · 环境检测</title>") {
		t.Fatal("页面标题应当以实例名称开头")
	}
	data := extractStartPageData(t, page)
	if data.ProfileName != "样例1" || data.LaunchCode != "AB12CD" || data.ProxyAddress != "127.0.0.1:7890" {
		t.Fatalf("数据块内容不对: %+v", data)
	}
	if data.FingerprintArgs == nil || data.Tags == nil {
		t.Fatalf("数组字段应当是空数组而不是 null: %+v", data)
	}
}

func TestRenderStartPageKeepsHostileNamesInsideDataBlock(t *testing.T) {
	t.Parallel()

	hostile := `</script><script>alert(1)</script><!-- "x" & 'y' __WYTAOF_DATA__`
	page, err := RenderStartPage(StartPageData{ProfileName: hostile, Tags: []string{hostile}})
	if err != nil {
		t.Fatalf("RenderStartPage 返回错误: %v", err)
	}
	text := string(page)
	if strings.Contains(text, "<script>alert(1)") {
		t.Fatal("实例名称里的脚本标签被原样写进了页面")
	}
	if strings.Count(text, `<script id="wytaof-data"`) != 1 {
		t.Fatal("数据块应当只有一个")
	}
	data := extractStartPageData(t, page)
	if data.ProfileName != hostile || len(data.Tags) != 1 || data.Tags[0] != hostile {
		t.Fatalf("特殊字符应当原样留在数据里: %+v", data)
	}
}

func TestWriteStartPageCreatesFileAndReturnsFileURL(t *testing.T) {
	t.Parallel()

	userDataDir := filepath.Join(t.TempDir(), "实例 数据", "profile-1")
	pageURL, err := WriteStartPage(userDataDir, StartPageData{ProfileName: "样例1"})
	if err != nil {
		t.Fatalf("WriteStartPage 返回错误: %v", err)
	}
	if !strings.HasPrefix(pageURL, "file:///") || !strings.HasSuffix(pageURL, "/"+StartPageFileName) {
		t.Fatalf("返回的地址不对: %s", pageURL)
	}
	if strings.ContainsAny(pageURL, " \\") || strings.Contains(pageURL, "实例") {
		t.Fatalf("地址里的空格、反斜杠和中文应当转义: %s", pageURL)
	}
	content, err := os.ReadFile(filepath.Join(userDataDir, StartPageFileName))
	if err != nil {
		t.Fatalf("检测页文件没有生成: %v", err)
	}
	if extractStartPageData(t, content).ProfileName != "样例1" {
		t.Fatal("检测页文件内容不对")
	}
	if !SameFileURL(pageURL, FileURLFromPath(filepath.Join(userDataDir, StartPageFileName))) {
		t.Fatal("同一个文件生成的地址应当判定为相同")
	}
}

func TestFileURLFromPathHandlesWindowsPaths(t *testing.T) {
	t.Parallel()

	if goruntime.GOOS != "windows" {
		t.Skip("只在 Windows 上验证盘符和网络共享路径")
	}
	tests := []struct {
		path string
		want string
	}{
		{path: `D:\Program Files\WytaoF Browser\data\abc\wytaof-start.html`, want: "file:///D:/Program%20Files/WytaoF%20Browser/data/abc/wytaof-start.html"},
		{path: `\\server\share\数据\wytaof-start.html`, want: "file://server/share/%E6%95%B0%E6%8D%AE/wytaof-start.html"},
	}
	for _, tt := range tests {
		if got := FileURLFromPath(tt.path); got != tt.want {
			t.Fatalf("FileURLFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestSameFileURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a, b string
		want bool
	}{
		{a: "file:///D:/Program%20Files/a/wytaof-start.html", b: "file:///d:/Program Files/a/wytaof-start.html", want: true},
		{a: "file:///D:/a/wytaof-start.html", b: "file:///D:/b/wytaof-start.html", want: false},
		{a: "https://example.com/wytaof-start.html", b: "https://example.com/wytaof-start.html", want: false},
		{a: "", b: "", want: false},
	}
	for _, tt := range tests {
		if got := SameFileURL(tt.a, tt.b); got != tt.want {
			t.Fatalf("SameFileURL(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
