//go:build windows
// +build windows

package backend

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	goruntime "runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	windowTitlePollInterval   = 800 * time.Millisecond
	windowTitleSetTimeoutMs   = 500
	windowTitleBufferSize     = 1024
	windowTitleMaxWindowScan  = 4096
	chromeTopWindowClassName  = "Chrome_WidgetWin_1"
	win32MessageSetText       = 0x000C
	win32SendAbortIfHung      = 0x0002
	win32GetWindowOwnerOption = 4
)

var (
	user32DLL               = windows.NewLazySystemDLL("user32.dll")
	procFindWindowExW       = user32DLL.NewProc("FindWindowExW")
	procGetWindowTextW      = user32DLL.NewProc("GetWindowTextW")
	procGetWindow           = user32DLL.NewProc("GetWindow")
	procSendMessageTimeoutW = user32DLL.NewProc("SendMessageTimeoutW")
)

type instanceWindowTitler struct {
	states      map[windows.HWND]windowTitleState
	className   *uint16
	panicLogged bool
}

func (a *App) startInstanceWindowTitler() {
	className, err := windows.UTF16PtrFromString(chromeTopWindowClassName)
	if err != nil {
		return
	}
	a.windowTitleStop = make(chan struct{})
	titler := &instanceWindowTitler{states: make(map[windows.HWND]windowTitleState), className: className}
	go titler.run(a, a.windowTitleStop)
}

func (t *instanceWindowTitler) run(a *App, stop <-chan struct{}) {
	ticker := time.NewTicker(windowTitlePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			t.safeTick(a)
		}
	}
}

func (t *instanceWindowTitler) safeTick(a *App) {
	defer func() {
		if r := recover(); r != nil && !t.panicLogged {
			t.panicLogged = true
			logger.New("Browser").Error("窗口标题同步出错", logger.F("error", fmt.Sprint(r)))
		}
	}()
	t.tick(a.instanceWindowTitleTargets())
}

func (t *instanceWindowTitler) tick(targets map[uint32]string) {
	seen := make(map[windows.HWND]struct{}, len(t.states))
	if len(targets) > 0 {
		var cursor uintptr
		for i := 0; i < windowTitleMaxWindowScan; i++ {
			cursor, _, _ = procFindWindowExW.Call(0, cursor, uintptr(unsafe.Pointer(t.className)), 0)
			if cursor == 0 {
				break
			}
			hwnd := windows.HWND(cursor)
			var pid uint32
			if _, err := windows.GetWindowThreadProcessId(hwnd, &pid); err != nil {
				continue
			}
			prefix, ok := targets[pid]
			if !ok || !windows.IsWindowVisible(hwnd) {
				continue
			}
			if owner, _, _ := procGetWindow.Call(cursor, win32GetWindowOwnerOption); owner != 0 {
				continue
			}
			title, complete := readWindowTitle(cursor)
			if !complete || title == "" {
				continue
			}
			seen[hwnd] = struct{}{}

			next, state, changed := decideWindowTitle(title, prefix, t.states[hwnd])
			if changed && !writeWindowTitle(cursor, next) {
				continue
			}
			t.states[hwnd] = state
		}
	}
	for hwnd := range t.states {
		if _, ok := seen[hwnd]; !ok {
			delete(t.states, hwnd)
		}
	}
}

func readWindowTitle(hwnd uintptr) (string, bool) {
	buffer := make([]uint16, windowTitleBufferSize)
	length, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if length == 0 {
		return "", true
	}
	if int(length) >= len(buffer)-1 {
		return "", false
	}
	return windows.UTF16ToString(buffer[:length]), true
}

func writeWindowTitle(hwnd uintptr, title string) bool {
	text, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return false
	}
	var result uintptr
	ok, _, _ := procSendMessageTimeoutW.Call(
		hwnd,
		win32MessageSetText,
		0,
		uintptr(unsafe.Pointer(text)),
		win32SendAbortIfHung,
		windowTitleSetTimeoutMs,
		uintptr(unsafe.Pointer(&result)),
	)
	goruntime.KeepAlive(text)
	return ok != 0
}
