package browser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKernelMajorVersionFromManifestAndDirName(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "Chrom-150")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "150.0.7871.186.manifest"), []byte("{}"), 0o644)
	if v := KernelMajorVersion(filepath.Join(dir, "chrome.exe")); v != 150 {
		t.Fatalf("manifest 推断大版本错误：%d", v)
	}
	bare := filepath.Join(t.TempDir(), "Chrom-144")
	_ = os.MkdirAll(bare, 0o755)
	if v := KernelMajorVersion(filepath.Join(bare, "chrome.exe")); v != 144 {
		t.Fatalf("目录名推断大版本错误：%d", v)
	}
}

func TestEnsureRealCanvasForCrashingKernel(t *testing.T) {
	t.Parallel()
	crash := filepath.Join(t.TempDir(), "Chrom-150")
	_ = os.MkdirAll(crash, 0o755)
	ok := filepath.Join(t.TempDir(), "Chrom-144")
	_ = os.MkdirAll(ok, 0o755)
	seeded := []string{"--fingerprint=2031806230", "--fingerprint-brand=Chrome"}
	got, changed := EnsureRealCanvasForCrashingKernel(seeded, filepath.Join(crash, "chrome.exe"))
	if !changed || !strings.Contains(strings.Join(got, " "), "--disable-spoofing=canvas") {
		t.Fatalf("150 带种子应补 --disable-spoofing=canvas：%v", got)
	}
	got, changed = EnsureRealCanvasForCrashingKernel([]string{"--fingerprint=1", "--disable-spoofing=gpu"}, filepath.Join(crash, "chrome.exe"))
	if !changed || !strings.Contains(strings.Join(got, " "), "--disable-spoofing=gpu,canvas") || strings.Count(strings.Join(got, " "), "--disable-spoofing=") != 1 {
		t.Fatalf("应并入已有 disable-spoofing：%v", got)
	}
	if _, changed = EnsureRealCanvasForCrashingKernel([]string{"--fingerprint=1", "--disable-spoofing=canvas"}, filepath.Join(crash, "chrome.exe")); changed {
		t.Fatalf("已经关了画布篡改不应再改")
	}
	if _, changed = EnsureRealCanvasForCrashingKernel(seeded, filepath.Join(ok, "chrome.exe")); changed {
		t.Fatalf("144 不应改动")
	}
	if _, changed = EnsureRealCanvasForCrashingKernel([]string{"--fingerprint-brand=Chrome"}, filepath.Join(crash, "chrome.exe")); changed {
		t.Fatalf("没有种子不应改动")
	}
}