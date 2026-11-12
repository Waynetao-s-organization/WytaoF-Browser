package browser

import (
	"strings"
	"testing"

	"ant-chrome/backend/internal/config"
)

func newRenameTestManager(t *testing.T) *Manager {
	t.Helper()

	mgr := NewManager(config.DefaultConfig(), t.TempDir())
	mgr.ProfileDAO = NewSQLiteProfileDAO(openMigratedTestDB(t))
	profile := &Profile{
		ProfileId:   "profile-1",
		ProfileName: "旧名字",
		UserDataDir: "profile-1",
		Tags:        []string{"Team"},
		CreatedAt:   "2026-01-01T00:00:00+08:00",
		UpdatedAt:   "2026-01-01T00:00:00+08:00",
	}
	if err := mgr.ProfileDAO.Upsert(profile); err != nil {
		t.Fatalf("预置实例失败: %v", err)
	}
	mgr.InitData()
	return mgr
}

func TestRenameUpdatesOnlyTheName(t *testing.T) {
	t.Parallel()

	mgr := newRenameTestManager(t)
	renamed, err := mgr.Rename("profile-1", "  样例1  ")
	if err != nil {
		t.Fatalf("Rename 返回错误: %v", err)
	}
	if renamed.ProfileName != "样例1" {
		t.Fatalf("名称应当去掉首尾空格后保存，实际 %q", renamed.ProfileName)
	}

	stored, err := mgr.ProfileDAO.GetById("profile-1")
	if err != nil {
		t.Fatalf("读取实例失败: %v", err)
	}
	if stored.ProfileName != "样例1" || stored.UserDataDir != "profile-1" || len(stored.Tags) != 1 || stored.Tags[0] != "Team" {
		t.Fatalf("改名只应改名称，实际 %+v", stored)
	}
	if stored.UpdatedAt == "2026-01-01T00:00:00+08:00" {
		t.Fatalf("改名后更新时间应当变化")
	}
}

func TestRenameRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	mgr := newRenameTestManager(t)
	if _, err := mgr.Rename("profile-1", "   "); err == nil {
		t.Fatal("空名称应当被拒绝")
	}
	if _, err := mgr.Rename("profile-1", strings.Repeat("名", maxProfileNameRunes+1)); err == nil {
		t.Fatal("超长名称应当被拒绝")
	}
	if _, err := mgr.Rename("missing", "新名字"); err == nil {
		t.Fatal("不存在的实例应当报错")
	}

	stored, err := mgr.ProfileDAO.GetById("profile-1")
	if err != nil {
		t.Fatalf("读取实例失败: %v", err)
	}
	if stored.ProfileName != "旧名字" {
		t.Fatalf("被拒绝的改名不应改动数据，实际 %q", stored.ProfileName)
	}
}

func TestCopyPreservesFingerprintConfigWithoutOldSeed(t *testing.T) {
	t.Parallel()

	mgr := newRenameTestManager(t)
	mgr.Profiles["profile-1"].FingerprintArgs = []string{
		"--fingerprint=999999",
		"--lang=en-US",
		"--timezone=America/Los_Angeles",
		"--webrtc-ip-handling-policy=disable_non_proxied_udp",
	}
	_ = mgr.ProfileDAO.Upsert(mgr.Profiles["profile-1"])

	copied, err := mgr.Copy("profile-1", "样例副本")
	if err != nil {
		t.Fatalf("Copy 返回错误: %v", err)
	}
	for _, arg := range copied.FingerprintArgs {
		if strings.HasPrefix(arg, "--fingerprint=") {
			t.Fatalf("克隆实例不应沿用旧指纹种子: %v", copied.FingerprintArgs)
		}
	}
	want := []string{"--lang=en-US", "--timezone=America/Los_Angeles", "--webrtc-ip-handling-policy=disable_non_proxied_udp"}
	if strings.Join(copied.FingerprintArgs, "|") != strings.Join(want, "|") {
		t.Fatalf("克隆实例应保留时区、语言等指纹设置:\n got=%v\nwant=%v", copied.FingerprintArgs, want)
	}
}
