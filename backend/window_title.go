package backend

import (
	"strings"
	"unicode"
)

const maxWindowTitleNameRunes = 60

type windowTitleState struct {
	raw     string
	applied string
}

func instanceWindowTitlePrefix(profileName string) string {
	fields := strings.FieldsFunc(profileName, func(r rune) bool {
		return unicode.IsControl(r) || unicode.IsSpace(r)
	})
	name := []rune(strings.Join(fields, " "))
	if len(name) == 0 {
		return ""
	}
	if len(name) > maxWindowTitleNameRunes {
		name = append(name[:maxWindowTitleNameRunes], '…')
	}
	return "【" + string(name) + "】"
}

// decideWindowTitle 根据窗口当前标题算出要不要改写；浏览器自己换了标题就重新加前缀，实例改名就换前缀。
func decideWindowTitle(current string, prefix string, state windowTitleState) (string, windowTitleState, bool) {
	if current == "" || prefix == "" {
		return "", state, false
	}
	if state.applied == current {
		want := prefix + state.raw
		if want == current {
			return "", state, false
		}
		return want, windowTitleState{raw: state.raw, applied: want}, true
	}
	if strings.HasPrefix(current, prefix) {
		return "", windowTitleState{raw: strings.TrimPrefix(current, prefix), applied: current}, false
	}
	next := prefix + current
	return next, windowTitleState{raw: current, applied: next}, true
}

func (a *App) instanceWindowTitleTargets() map[uint32]string {
	if a == nil || a.browserMgr == nil || !a.instanceNameOnWindowEnabled() {
		return nil
	}
	a.browserMgr.Mutex.Lock()
	defer a.browserMgr.Mutex.Unlock()

	targets := make(map[uint32]string)
	for _, profile := range a.browserMgr.Profiles {
		if profile == nil || !profile.Running || profile.Pid <= 0 {
			continue
		}
		if prefix := instanceWindowTitlePrefix(profile.ProfileName); prefix != "" {
			targets[uint32(profile.Pid)] = prefix
		}
	}
	return targets
}

func (a *App) stopInstanceWindowTitler() {
	a.windowTitleStopOnce.Do(func() {
		if a.windowTitleStop != nil {
			close(a.windowTitleStop)
		}
	})
}
