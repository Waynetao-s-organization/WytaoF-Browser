//go:build windows

package browser

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestEnsureProfileDisplayNameSkipsRunningBrowser(t *testing.T) {
	t.Parallel()

	userDataDir := t.TempDir()
	localStatePath := filepath.Join(userDataDir, "Local State")
	if err := os.WriteFile(localStatePath, []byte(sampleLocalState), 0o644); err != nil {
		t.Fatalf("写入 Local State 失败: %v", err)
	}

	lockPath, err := syscall.UTF16PtrFromString(filepath.Join(userDataDir, "lockfile"))
	if err != nil {
		t.Fatalf("转换路径失败: %v", err)
	}
	handle, err := syscall.CreateFile(lockPath, syscall.GENERIC_WRITE, syscall.FILE_SHARE_READ, nil, syscall.CREATE_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("模拟浏览器占用 lockfile 失败: %v", err)
	}

	changed, err := EnsureProfileDisplayName(userDataDir, "样例1")
	if err != nil || changed {
		t.Fatalf("浏览器在运行时不应改文件: changed=%v err=%v", changed, err)
	}
	data, _ := os.ReadFile(localStatePath)
	if string(data) != sampleLocalState {
		t.Fatal("浏览器在运行时 Local State 被改动了")
	}

	_ = syscall.CloseHandle(handle)
	_ = os.Remove(filepath.Join(userDataDir, "lockfile"))
	changed, err = EnsureProfileDisplayName(userDataDir, "样例1")
	if err != nil || !changed {
		t.Fatalf("浏览器退出后应当可以改: changed=%v err=%v", changed, err)
	}
}
