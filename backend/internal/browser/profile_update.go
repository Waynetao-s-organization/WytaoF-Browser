package browser

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	"strings"
	"time"
)

// Update 更新配置
func (m *Manager) Update(profileId string, input ProfileInput) (*Profile, error) {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	profile, exists := m.Profiles[profileId]
	if !exists {
		log.Error("浏览器配置不存在", logger.F("profile_id", profileId))
		return nil, fmt.Errorf("profile not found")
	}
	resolvedProxy, err := m.resolveProfileProxyInput(input.ProxyId, input.ProxyConfig)
	if err != nil {
		log.Error("代理绑定失败", logger.F("profile_id", profileId), logger.F("proxy_id", strings.TrimSpace(input.ProxyId)), logger.F("error", err.Error()))
		return nil, err
	}

	profile.ProfileName = input.ProfileName
	profile.UserDataDir = input.UserDataDir
	profile.CoreId = normalizeProfileCoreID(input.CoreId)
	profile.FingerprintArgs = input.FingerprintArgs
	if resolvedProxy.HasSelectedProxy {
		_ = BindProfileToProxy(profile, resolvedProxy.SelectedProxy, true)
	} else if resolvedProxy.FallbackToDirect {
		_ = m.bindProfileToDirectProxy(profile)
	} else {
		profile.ProxyId = resolvedProxy.ProxyId
		profile.ProxyConfig = resolvedProxy.ProxyConfig
		_ = ClearProfileProxyBinding(profile)
	}
	if resolvedProxy.UsedConfigFallback {
		log.Warn("代理ID未命中，已改为使用输入的代理配置",
			logger.F("profile_id", profileId),
			logger.F("proxy_id", strings.TrimSpace(input.ProxyId)),
		)
	}
	profile.LaunchArgs = input.LaunchArgs
	profile.Tags = input.Tags
	profile.Keywords = append([]string{}, input.Keywords...)
	profile.GroupId = buildProfileGroupID(input.GroupId)
	profile.UpdatedAt = time.Now().Format(time.RFC3339)

	log.Info("浏览器配置更新", logger.F("profile_id", profileId), logger.F("profile_name", input.ProfileName))
	if m.ProfileDAO != nil {
		profile.CoreId = normalizeProfileCoreID(profile.CoreId)
		if err := m.ProfileDAO.Upsert(profile); err != nil {
			log.Error("实例配置更新持久化失败", logger.F("profile_id", profileId), logger.F("error", err))
			return nil, err
		}
		return profile, nil
	}
	if err := m.SaveProfiles(); err != nil {
		return nil, err
	}
	return profile, nil
}

const maxProfileNameRunes = 100

// Rename 只修改实例名称，不影响其他字段
func (m *Manager) Rename(profileId string, profileName string) (*Profile, error) {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	profile, exists := m.Profiles[profileId]
	if !exists {
		return nil, fmt.Errorf("profile not found")
	}
	name := strings.TrimSpace(profileName)
	if name == "" {
		return nil, fmt.Errorf("实例名称不能为空")
	}
	if len([]rune(name)) > maxProfileNameRunes {
		return nil, fmt.Errorf("实例名称不能超过 %d 个字", maxProfileNameRunes)
	}
	if name == profile.ProfileName {
		return profile, nil
	}

	previous, previousUpdatedAt := profile.ProfileName, profile.UpdatedAt
	profile.ProfileName = name
	profile.UpdatedAt = time.Now().Format(time.RFC3339)

	var err error
	if m.ProfileDAO != nil {
		err = m.ProfileDAO.Upsert(profile)
	} else {
		err = m.SaveProfiles()
	}
	if err != nil {
		profile.ProfileName, profile.UpdatedAt = previous, previousUpdatedAt
		log.Error("实例改名保存失败", logger.F("profile_id", profileId), logger.F("error", err))
		return nil, err
	}
	log.Info("实例改名", logger.F("profile_id", profileId), logger.F("profile_name", name))
	return profile, nil
}

// SetKeywords 设置实例关键字（独立接口，不影响其他字段）
func (m *Manager) SetKeywords(profileId string, keywords []string) (*Profile, error) {
	log := logger.New("Browser")
	m.InitData()
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	profile, exists := m.Profiles[profileId]
	if !exists {
		return nil, fmt.Errorf("profile not found")
	}
	profile.Keywords = append([]string{}, keywords...)
	profile.UpdatedAt = time.Now().Format(time.RFC3339)

	log.Info("关键字更新", logger.F("profile_id", profileId))
	if err := m.SaveProfiles(); err != nil {
		return nil, err
	}
	return profile, nil
}
