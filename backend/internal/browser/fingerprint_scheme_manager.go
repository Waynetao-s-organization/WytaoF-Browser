package browser

import (
	"ant-chrome/backend/internal/logger"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// FingerprintSchemeApplyResult 批量把方案套用到实例的结果
type FingerprintSchemeApplyResult struct {
	Total   int      `json:"total"`
	Updated int      `json:"updated"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors"`
}

func (m *Manager) ensureSchemeCache() {
	if m.schemes != nil {
		return
	}
	m.schemes = map[string]*FingerprintScheme{}
	if m.SchemeDAO == nil {
		return
	}
	list, err := m.SchemeDAO.List()
	if err != nil {
		logger.New("Browser").Error("加载指纹方案失败", logger.F("error", err.Error()))
		return
	}
	for _, s := range list {
		m.schemes[s.SchemeId] = s
	}
}

// EnsureBuiltinSchemes 表为空时写入内置方案；已有数据时只补缺失的内置项，不覆盖用户改过的内容
func (m *Manager) EnsureBuiltinSchemes() {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	m.ensureSchemeCache()
	if m.SchemeDAO == nil {
		return
	}
	hasDefault := false
	for _, s := range m.schemes {
		if s.IsDefault {
			hasDefault = true
		}
	}
	for _, builtin := range BuiltinFingerprintSchemes() {
		if _, exists := m.schemes[builtin.SchemeId]; exists {
			continue
		}
		scheme := builtin
		if hasDefault {
			scheme.IsDefault = false
		} else if scheme.IsDefault {
			hasDefault = true
		}
		if err := m.SchemeDAO.Upsert(&scheme); err != nil {
			logger.New("Browser").Error("写入内置指纹方案失败", logger.F("scheme", scheme.SchemeName), logger.F("error", err.Error()))
			continue
		}
		copied := scheme
		m.schemes[copied.SchemeId] = &copied
	}
	if !hasDefault {
		if first, ok := m.schemes["builtin-us-west-desktop"]; ok {
			first.IsDefault = true
			_ = m.SchemeDAO.Upsert(first)
		}
	}
}

func (m *Manager) listSchemesLocked() []*FingerprintScheme {
	m.ensureSchemeCache()
	list := make([]*FingerprintScheme, 0, len(m.schemes))
	for _, s := range m.schemes {
		list = append(list, s)
	}
	sortSchemes(list)
	return list
}

// ListFingerprintSchemes 全部方案（默认方案排最前）
func (m *Manager) ListFingerprintSchemes() []FingerprintScheme {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	list := m.listSchemesLocked()
	out := make([]FingerprintScheme, 0, len(list))
	for _, s := range list {
		out = append(out, *s)
	}
	return out
}

func (m *Manager) defaultSchemeLocked() *FingerprintScheme {
	m.ensureSchemeCache()
	for _, s := range m.schemes {
		if s.IsDefault {
			return s
		}
	}
	list := m.listSchemesLocked()
	if len(list) > 0 {
		return list[0]
	}
	return nil
}

func (m *Manager) schemeByIDLocked(schemeId string) *FingerprintScheme {
	m.ensureSchemeCache()
	schemeId = strings.TrimSpace(schemeId)
	if schemeId == "" || schemeId == FingerprintSchemeNone {
		return nil
	}
	return m.schemes[schemeId]
}

// ResolveProfileScheme 实例实际生效的方案：绑定了就用绑定的；绑定失效或为空时用默认方案；明确填 __none__ 则不用
func (m *Manager) ResolveProfileScheme(profile *Profile) *FingerprintScheme {
	if profile == nil {
		return nil
	}
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	return m.resolveProfileSchemeLocked(profile)
}

func (m *Manager) resolveProfileSchemeLocked(profile *Profile) *FingerprintScheme {
	id := strings.TrimSpace(profile.FingerprintSchemeId)
	if id == FingerprintSchemeNone {
		return nil
	}
	if s := m.schemeByIDLocked(id); s != nil {
		return s
	}
	return m.defaultSchemeLocked()
}

// EffectiveFingerprintArgs 启动时真正下发的指纹参数（方案固定项强制生效，设备项保留实例自己的）
func (m *Manager) EffectiveFingerprintArgs(profile *Profile) []string {
	if profile == nil {
		return nil
	}
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	return m.EffectiveFingerprintArgsLocked(profile)
}

// EffectiveFingerprintArgsLocked 同 EffectiveFingerprintArgs，调用方必须已持有 Manager.Mutex
// （实例启动流程 browserInstanceStartWithInput 全程持锁，这里不能再加锁，否则死锁、界面一直转圈）
func (m *Manager) EffectiveFingerprintArgsLocked(profile *Profile) []string {
	if profile == nil {
		return nil
	}
	scheme := m.resolveProfileSchemeLocked(profile)
	if scheme == nil {
		return append([]string{}, profile.FingerprintArgs...)
	}
	return ApplySchemeToArgs(scheme, profile.ProfileId, profile.FingerprintArgs)
}

// ResolveProfileCoreID 方案指定了内核时优先用方案的
func (m *Manager) ResolveProfileCoreID(profile *Profile) string {
	if profile == nil {
		return ""
	}
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	return m.ResolveProfileCoreIDLocked(profile)
}

// ResolveProfileCoreIDLocked 同 ResolveProfileCoreID，调用方必须已持有 Manager.Mutex
func (m *Manager) ResolveProfileCoreIDLocked(profile *Profile) string {
	if profile == nil {
		return ""
	}
	if scheme := m.resolveProfileSchemeLocked(profile); scheme != nil && strings.TrimSpace(scheme.Fixed.CoreId) != "" {
		if _, ok := m.GetCore(scheme.Fixed.CoreId); ok {
			return scheme.Fixed.CoreId
		}
	}
	return profile.CoreId
}

// GenerateFingerprintArgsForScheme 为新窗口生成一套“固定项 + 随机设备”的参数
func (m *Manager) GenerateFingerprintArgsForScheme(schemeId string) ([]string, error) {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	scheme := m.schemeByIDLocked(schemeId)
	if scheme == nil {
		return nil, fmt.Errorf("指纹方案不存在：%s", schemeId)
	}
	return NewSchemeProfileArgs(scheme), nil
}

// CreateFingerprintScheme 新建方案
func (m *Manager) CreateFingerprintScheme(input FingerprintSchemeInput) (*FingerprintScheme, error) {
	normalized, err := NormalizeSchemeInput(input)
	if err != nil {
		return nil, err
	}
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	m.ensureSchemeCache()
	if m.SchemeDAO == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	now := time.Now().Format(time.RFC3339)
	scheme := &FingerprintScheme{
		SchemeId: uuid.NewString(), SchemeName: normalized.SchemeName, Description: normalized.Description,
		Fixed: normalized.Fixed, Device: normalized.Device, SortOrder: 100 + len(m.schemes), CreatedAt: now, UpdatedAt: now,
	}
	if len(m.schemes) == 0 {
		scheme.IsDefault = true
	}
	if err := m.SchemeDAO.Upsert(scheme); err != nil {
		return nil, err
	}
	m.schemes[scheme.SchemeId] = scheme
	copied := *scheme
	return &copied, nil
}

// UpdateFingerprintScheme 修改方案（内置方案也允许改固定项和设备池）
func (m *Manager) UpdateFingerprintScheme(schemeId string, input FingerprintSchemeInput) (*FingerprintScheme, error) {
	normalized, err := NormalizeSchemeInput(input)
	if err != nil {
		return nil, err
	}
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	scheme := m.schemeByIDLocked(schemeId)
	if scheme == nil {
		return nil, fmt.Errorf("指纹方案不存在：%s", schemeId)
	}
	scheme.SchemeName = normalized.SchemeName
	scheme.Description = normalized.Description
	scheme.Fixed = normalized.Fixed
	scheme.Device = normalized.Device
	if m.SchemeDAO != nil {
		if err := m.SchemeDAO.Upsert(scheme); err != nil {
			return nil, err
		}
	}
	copied := *scheme
	return &copied, nil
}

// DeleteFingerprintScheme 删除方案；绑定在它上面的实例改回默认方案
func (m *Manager) DeleteFingerprintScheme(schemeId string) error {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	scheme := m.schemeByIDLocked(schemeId)
	if scheme == nil {
		return fmt.Errorf("指纹方案不存在：%s", schemeId)
	}
	if scheme.IsDefault {
		return fmt.Errorf("默认方案不能删除，请先把别的方案设为默认")
	}
	if m.SchemeDAO != nil {
		if err := m.SchemeDAO.Delete(schemeId); err != nil {
			return err
		}
	}
	delete(m.schemes, schemeId)
	changed := false
	for _, p := range m.Profiles {
		if strings.TrimSpace(p.FingerprintSchemeId) == schemeId {
			p.FingerprintSchemeId = ""
			changed = true
		}
	}
	if changed {
		_ = m.SaveProfiles()
	}
	return nil
}

// SetDefaultFingerprintScheme 设为默认方案
func (m *Manager) SetDefaultFingerprintScheme(schemeId string) error {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	target := m.schemeByIDLocked(schemeId)
	if target == nil {
		return fmt.Errorf("指纹方案不存在：%s", schemeId)
	}
	for _, s := range m.schemes {
		if s.IsDefault != (s.SchemeId == schemeId) {
			s.IsDefault = s.SchemeId == schemeId
			if m.SchemeDAO != nil {
				if err := m.SchemeDAO.Upsert(s); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// applySchemeToProfileLocked 把方案写回实例的存储参数（固定项以方案为准，设备项保留；rerollDevice=true 时整套设备重抽）
func (m *Manager) applySchemeToProfileLocked(profile *Profile, scheme *FingerprintScheme, rerollDevice bool) bool {
	if profile == nil || scheme == nil {
		return false
	}
	var next []string
	if rerollDevice {
		next = NewSchemeProfileArgs(scheme)
	} else {
		next = ApplySchemeToArgs(scheme, profile.ProfileId, profile.FingerprintArgs)
	}
	changed := profile.FingerprintSchemeId != scheme.SchemeId || !equalStringSlices(profile.FingerprintArgs, next)
	profile.FingerprintSchemeId = scheme.SchemeId
	profile.FingerprintArgs = next
	if coreId := strings.TrimSpace(scheme.Fixed.CoreId); coreId != "" {
		if _, ok := m.GetCore(coreId); ok && profile.CoreId != coreId {
			profile.CoreId = coreId
			changed = true
		}
	}
	if changed {
		profile.UpdatedAt = time.Now().Format(time.RFC3339)
	}
	return changed
}

// ApplyFingerprintSchemeToProfiles 把方案套到指定实例（profileIds 为空 = 全部实例）
func (m *Manager) ApplyFingerprintSchemeToProfiles(schemeId string, profileIds []string, rerollDevice bool) (FingerprintSchemeApplyResult, error) {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	scheme := m.schemeByIDLocked(schemeId)
	if scheme == nil {
		return FingerprintSchemeApplyResult{}, fmt.Errorf("指纹方案不存在：%s", schemeId)
	}
	targets := make([]*Profile, 0)
	if len(profileIds) == 0 {
		for _, p := range m.Profiles {
			targets = append(targets, p)
		}
	} else {
		for _, id := range profileIds {
			if p, ok := m.Profiles[strings.TrimSpace(id)]; ok {
				targets = append(targets, p)
			}
		}
	}
	result := FingerprintSchemeApplyResult{Total: len(targets), Errors: []string{}}
	for _, p := range targets {
		if p.Running {
			result.Skipped++
			result.Errors = append(result.Errors, fmt.Sprintf("%s 正在运行，已跳过（重启后生效的改动请先关闭实例）", p.ProfileName))
			continue
		}
		if m.applySchemeToProfileLocked(p, scheme, rerollDevice) {
			result.Updated++
		} else {
			result.Skipped++
		}
	}
	if result.Updated > 0 {
		if err := m.SaveProfiles(); err != nil {
			return result, err
		}
	}
	return result, nil
}

// MigrateLegacyProfilesToDefaultScheme 第一次升级：把所有还没绑定方案的实例绑到默认方案，固定项统一、设备项保留
func (m *Manager) MigrateLegacyProfilesToDefaultScheme() (int, error) {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	scheme := m.defaultSchemeLocked()
	if scheme == nil {
		return 0, fmt.Errorf("没有可用的默认指纹方案")
	}
	updated := 0
	for _, p := range m.Profiles {
		if strings.TrimSpace(p.FingerprintSchemeId) != "" {
			continue
		}
		if m.applySchemeToProfileLocked(p, scheme, false) {
			updated++
		} else {
			p.FingerprintSchemeId = scheme.SchemeId
		}
	}
	if err := m.SaveProfiles(); err != nil {
		return updated, err
	}
	return updated, nil
}

// prepareSchemeForNewProfile 新建实例时套用方案：未指定方案用默认方案，__none__ 表示手动配置
func (m *Manager) prepareSchemeForNewProfile(input *ProfileInput, forceNewDevice bool) {
	if input == nil {
		return
	}
	id := strings.TrimSpace(input.FingerprintSchemeId)
	if id == FingerprintSchemeNone {
		return
	}
	var scheme *FingerprintScheme
	if id != "" {
		scheme = m.schemeByIDLocked(id)
	}
	if scheme == nil {
		scheme = m.defaultSchemeLocked()
	}
	if scheme == nil {
		return
	}
	input.FingerprintSchemeId = scheme.SchemeId
	if forceNewDevice || ExtractFingerprintSeed(input.FingerprintArgs) == "" {
		input.FingerprintArgs = NewSchemeProfileArgs(scheme)
	} else {
		input.FingerprintArgs = ApplySchemeToArgs(scheme, uuid.NewString(), input.FingerprintArgs)
	}
	if coreId := strings.TrimSpace(scheme.Fixed.CoreId); coreId != "" {
		if _, ok := m.GetCore(coreId); ok {
			input.CoreId = coreId
		}
	}
}

// ensureUniqueSeedLocked 新实例的种子不能和已有实例重复，否则两个窗口会是同一台“设备”
func (m *Manager) ensureUniqueSeedLocked(args []string, selfId string) []string {
	seed := ExtractFingerprintSeed(args)
	if seed == "" {
		return args
	}
	for _, p := range m.Profiles {
		if p.ProfileId == selfId {
			continue
		}
		if ExtractFingerprintSeed(p.FingerprintArgs) == seed {
			return ReplaceFingerprintSeed(args, randomFingerprintSeed())
		}
	}
	return args
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
