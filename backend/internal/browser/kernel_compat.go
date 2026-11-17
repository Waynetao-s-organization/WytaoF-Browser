package browser

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// canvasSpoofCrashMajors 带种子读 Canvas 会让标签页崩溃的内核大版本（2026-11-17 实测：Chrom-150 上 Cloudflare Turnstile 小部件直接崩成哭脸、拿不到 token，加 --disable-spoofing=canvas 后正常）
var canvasSpoofCrashMajors = map[int]bool{150: true}

var kernelVersionPattern = regexp.MustCompile(`(\d{2,3})\.\d+\.\d+\.\d+`)

// KernelMajorVersion 从内核可执行文件所在目录推断大版本号（manifest 文件名 / 版本号子目录 / 目录名里的 Chrom-150），推断不出返回 0
func KernelMajorVersion(binaryPath string) int {
	dir := filepath.Dir(strings.TrimSpace(binaryPath))
	if dir == "" || dir == "." {
		return 0
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, "*.manifest")); len(matches) > 0 {
		if m := kernelVersionPattern.FindStringSubmatch(filepath.Base(matches[0])); len(m) > 1 {
			if v, err := strconv.Atoi(m[1]); err == nil {
				return v
			}
		}
	}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				if m := kernelVersionPattern.FindStringSubmatch(entry.Name()); len(m) > 1 && m[0] == entry.Name() {
					if v, err := strconv.Atoi(m[1]); err == nil {
						return v
					}
				}
			}
		}
	}
	if m := regexp.MustCompile(`(?i)chrom(?:e|ium)?[-_ ]?(\d{2,3})`).FindStringSubmatch(filepath.Base(dir)); len(m) > 1 {
		if v, err := strconv.Atoi(m[1]); err == nil {
			return v
		}
	}
	return 0
}

// KernelCanvasSpoofCrashes 该内核带种子篡改 Canvas 是否会崩溃
func KernelCanvasSpoofCrashes(binaryPath string) bool {
	return canvasSpoofCrashMajors[KernelMajorVersion(binaryPath)]
}

// EnsureRealCanvasForCrashingKernel 对已知会崩的内核，带种子时强制补上 --disable-spoofing=canvas（已有 disable-spoofing 就并进去）。
// 返回整理后的参数和是否做了改动。
func EnsureRealCanvasForCrashingKernel(args []string, binaryPath string) ([]string, bool) {
	if !KernelCanvasSpoofCrashes(binaryPath) || ExtractFingerprintSeed(args) == "" {
		return args, false
	}
	out := make([]string, 0, len(args)+1)
	merged := false
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		if strings.HasPrefix(strings.ToLower(trimmed), "--disable-spoofing=") {
			items := strings.Split(trimmed[len("--disable-spoofing="):], ",")
			has := false
			for _, item := range items {
				if strings.EqualFold(strings.TrimSpace(item), "canvas") {
					has = true
				}
			}
			if has {
				return args, false
			}
			out = append(out, "--disable-spoofing="+strings.Join(append(items, "canvas"), ","))
			merged = true
			continue
		}
		out = append(out, arg)
	}
	if !merged {
		out = append(out, "--disable-spoofing=canvas")
	}
	return out, true
}