//go:build windows
// +build windows

package backend

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWindowTitlerPrefixesRealBrowserWindow(t *testing.T) {
	chromeExe := strings.TrimSpace(os.Getenv("WYTAOF_KERNEL_TEST_EXE"))
	if chromeExe == "" {
		t.Skip("真机测试：把环境变量 WYTAOF_KERNEL_TEST_EXE 设成某个内核的 chrome.exe 路径后才会跑")
	}

	cmd := exec.Command(chromeExe,
		"--user-data-dir="+t.TempDir(),
		"--no-first-run",
		"--no-default-browser-check",
		"--window-size=900,600",
		"about:blank",
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动内核失败: %v", err)
	}
	defer func() {
		_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
		_, _ = cmd.Process.Wait()
	}()

	className, err := windows.UTF16PtrFromString(chromeTopWindowClassName)
	if err != nil {
		t.Fatalf("转换窗口类名失败: %v", err)
	}
	titler := &instanceWindowTitler{states: make(map[windows.HWND]windowTitleState), className: className}
	pid := uint32(cmd.Process.Pid)

	readTitles := func() []string {
		var titles []string
		var cursor uintptr
		for i := 0; i < windowTitleMaxWindowScan; i++ {
			cursor, _, _ = procFindWindowExW.Call(0, cursor, uintptr(unsafe.Pointer(className)), 0)
			if cursor == 0 {
				break
			}
			var owner uint32
			if _, err := windows.GetWindowThreadProcessId(windows.HWND(cursor), &owner); err != nil || owner != pid {
				continue
			}
			if title, complete := readWindowTitle(cursor); complete && title != "" {
				titles = append(titles, title)
			}
		}
		return titles
	}
	waitForTitle := func(prefix string, timeout time.Duration) []string {
		deadline := time.Now().Add(timeout)
		for {
			titler.tick(map[uint32]string{pid: prefix})
			titles := readTitles()
			for _, title := range titles {
				if strings.HasPrefix(title, prefix) {
					return titles
				}
			}
			if time.Now().After(deadline) {
				return titles
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	titles := waitForTitle("【样例1】", 40*time.Second)
	t.Logf("加前缀后的窗口标题: %q", titles)
	if len(titles) == 0 || !strings.HasPrefix(titles[0], "【样例1】") {
		t.Fatalf("窗口标题没有加上实例名称: %q", titles)
	}

	titles = waitForTitle("【样例2】", 10*time.Second)
	t.Logf("改名后的窗口标题: %q", titles)
	if len(titles) == 0 || !strings.HasPrefix(titles[0], "【样例2】") || strings.Contains(titles[0], "【样例1】") {
		t.Fatalf("实例改名后窗口标题应当换成新名称: %q", titles)
	}
}
