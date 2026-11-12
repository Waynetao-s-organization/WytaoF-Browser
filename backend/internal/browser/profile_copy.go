package browser

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Copy 复制实例配置（保留源实例的时区、语言、防泄露等指纹项，仅去掉旧指纹种子由新实例重新生成）
func (m *Manager) Copy(profileId string, newName string) (*Profile, error) {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	src, exists := m.Profiles[profileId]
	if !exists {
		log.Error("源实例不存在", logger.F("profile_id", profileId))
		return nil, fmt.Errorf("profile not found")
	}

	now := time.Now().Format(time.RFC3339)
	newId := uuid.NewString()

	profileName := strings.TrimSpace(newName)
	if profileName == "" {
		profileName = src.ProfileName + " (副本)"
	}

	copiedFingerprintArgs := cloneFingerprintArgsWithoutSeed(src.FingerprintArgs)
	if len(copiedFingerprintArgs) == 0 {
		copiedFingerprintArgs = cloneFingerprintArgsWithoutSeed(m.Config.Browser.DefaultFingerprintArgs)
	}

	profile := &Profile{
		ProfileId:          newId,
		ProfileName:        profileName,
		UserDataDir:        newId,
		CoreId:             normalizeProfileCoreID(src.CoreId),
		FingerprintArgs:    copiedFingerprintArgs,
		ProxyId:            src.ProxyId,
		ProxyConfig:        src.ProxyConfig,
		ProxyBindSourceID:  src.ProxyBindSourceID,
		ProxyBindSourceURL: src.ProxyBindSourceURL,
		ProxyBindName:      src.ProxyBindName,
		ProxyBindUpdatedAt: src.ProxyBindUpdatedAt,
		LaunchArgs:         append([]string{}, src.LaunchArgs...),
		Tags:               append([]string{}, src.Tags...),
		Keywords:           append([]string{}, src.Keywords...),
		GroupId:            src.GroupId,
		Running:            false,
		DebugPort:          0,
		Pid:                0,
		LastError:          "",
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	m.Profiles[newId] = profile
	log.Info("实例复制成功", logger.F("src_id", profileId), logger.F("new_id", newId), logger.F("new_name", profileName))

	if m.ProfileDAO != nil {
		if err := m.ProfileDAO.Upsert(profile); err != nil {
			delete(m.Profiles, newId)
			return nil, err
		}
	} else if err := m.SaveProfiles(); err != nil {
		delete(m.Profiles, newId)
		return nil, err
	}

	m.ensureProfileLaunchCode(profile)
	return profile, nil
}

func cloneFingerprintArgsWithoutSeed(args []string) []string {
	out := make([]string, 0, len(args))
	for _, raw := range args {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(strings.ToLower(trimmed), "--fingerprint=") {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}
