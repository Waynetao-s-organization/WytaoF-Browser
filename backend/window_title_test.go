package backend

import (
	internalbrowser "ant-chrome/backend/internal/browser"
	"ant-chrome/backend/internal/config"
	"strings"
	"testing"
)

func TestInstanceWindowTitlePrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{name: "样例1", want: "【样例1】"},
		{name: "  Team\t01\n", want: "【Team 01】"},
		{name: "   ", want: ""},
		{name: strings.Repeat("长", maxWindowTitleNameRunes+5), want: "【" + strings.Repeat("长", maxWindowTitleNameRunes) + "…】"},
	}
	for _, tt := range tests {
		if got := instanceWindowTitlePrefix(tt.name); got != tt.want {
			t.Fatalf("instanceWindowTitlePrefix(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestDecideWindowTitle(t *testing.T) {
	t.Parallel()

	prefix := "【样例1】"

	next, state, changed := decideWindowTitle("ChatGPT - Chromium", prefix, windowTitleState{})
	if !changed || next != "【样例1】ChatGPT - Chromium" || state.raw != "ChatGPT - Chromium" || state.applied != next {
		t.Fatalf("第一次看到窗口应当加前缀: next=%q state=%+v changed=%v", next, state, changed)
	}

	if _, same, changed := decideWindowTitle(next, prefix, state); changed || same != state {
		t.Fatalf("标题没变时不应重复写: state=%+v changed=%v", same, changed)
	}

	next2, state2, changed := decideWindowTitle("Gmail - Chromium", prefix, state)
	if !changed || next2 != "【样例1】Gmail - Chromium" || state2.raw != "Gmail - Chromium" {
		t.Fatalf("浏览器换了标题后应当重新加前缀: next=%q state=%+v changed=%v", next2, state2, changed)
	}

	renamed, state3, changed := decideWindowTitle(next2, "【样例2】", state2)
	if !changed || renamed != "【样例2】Gmail - Chromium" || state3.raw != "Gmail - Chromium" {
		t.Fatalf("实例改名后应当换前缀而不是叠加: next=%q state=%+v changed=%v", renamed, state3, changed)
	}

	_, adopted, changed := decideWindowTitle("【样例1】Claude - Chromium", prefix, windowTitleState{})
	if changed || adopted.raw != "Claude - Chromium" || adopted.applied != "【样例1】Claude - Chromium" {
		t.Fatalf("已经带前缀的窗口不应再加一层: state=%+v changed=%v", adopted, changed)
	}

	if _, _, changed := decideWindowTitle("", prefix, windowTitleState{}); changed {
		t.Fatal("空标题不应改写")
	}
	if _, _, changed := decideWindowTitle("ChatGPT", "", windowTitleState{}); changed {
		t.Fatal("没有前缀时不应改写")
	}
}

func TestInstanceWindowTitleTargetsOnlyIncludesRunningProfiles(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	app := NewApp(t.TempDir())
	app.config = cfg
	app.browserMgr = internalbrowser.NewManager(cfg, app.appRoot)
	app.browserMgr.Profiles = map[string]*BrowserProfile{
		"running": {ProfileId: "running", ProfileName: "样例1", Running: true, Pid: 4321},
		"stopped": {ProfileId: "stopped", ProfileName: "样例2", Running: false, Pid: 0},
		"no-pid":  {ProfileId: "no-pid", ProfileName: "样例3", Running: true, Pid: 0},
	}

	targets := app.instanceWindowTitleTargets()
	if len(targets) != 1 || targets[4321] != "【样例1】" {
		t.Fatalf("只应包含运行中且有进程号的实例: %+v", targets)
	}

	cfg.Browser.WindowTitleDisabled = true
	if targets := app.instanceWindowTitleTargets(); len(targets) != 0 {
		t.Fatalf("设置里关掉以后不应再改窗口标题: %+v", targets)
	}
}
