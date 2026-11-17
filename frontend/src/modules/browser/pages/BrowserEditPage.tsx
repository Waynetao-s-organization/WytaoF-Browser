import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { FolderOpen, Layers, RotateCcw } from 'lucide-react'
import { Button, Card, ConfirmModal, FormItem, Input, Modal, Select, Textarea, toast } from '../../../shared/components'
import type { BrowserCore, BrowserProfileInput, BrowserProxy, BrowserGroup, BrowserProfileTemplate, FingerprintScheme } from '../types'
import { FINGERPRINT_SCHEME_NONE, createBrowserProfile, fetchAllTags, fetchBrowserCores, fetchBrowserProfileTemplates, fetchBrowserProfiles, fetchBrowserProxies, fetchBrowserSettings, fetchFingerprintSchemes, fetchGroups, generateFingerprintSchemeArgs, openUserDataDir, restartBrowserInstance, updateBrowserProfile } from '../api'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import { FingerprintPanel } from '../components/FingerprintPanel'
import { TagInput } from '../components/TagInput'
import { GroupSelector } from '../components/GroupSelector'
import { ProxyPickerModal } from '../components/ProxyPickerModal'

const fallbackLowLaunchArgs = ['--disable-sync', '--no-first-run', '--disable-speech-api']
const fallbackDefaultFingerprintArgs = ['--fingerprint-brand=Chrome', '--fingerprint-platform=windows']
const directProxyID = '__direct__'

function normalizeLaunchArgs(args: string[]): string[] {
  return (args || []).map(item => item.trim()).filter(Boolean)
}

function resolveDefaultLaunchArgs(args: string[]): string[] {
  const normalized = normalizeLaunchArgs(args)
  return normalized.length > 0 ? normalized : fallbackLowLaunchArgs
}

function resolveDefaultFingerprintArgs(args: string[]): string[] {
  const normalized = normalizeLaunchArgs(args)
  return normalized.length > 0 ? normalized : fallbackDefaultFingerprintArgs
}

function resolvePoolProxySelection(
  proxyId: string,
  proxyConfig: string,
  proxies: BrowserProxy[],
): { proxyId: string; proxyConfig: string } {
  const normalizedProxyId = proxyId.trim()
  if (normalizedProxyId) {
    const matchedByID = proxies.find((proxy) => proxy.proxyId.trim() === normalizedProxyId)
    if (matchedByID?.proxyId) {
      return { proxyId: matchedByID.proxyId, proxyConfig: '' }
    }
  }

  const rawProxyConfig = proxyConfig.trim()
  const normalizedConfig = rawProxyConfig.toLowerCase()
  if (normalizedConfig) {
    const matchedByConfig = proxies.find((proxy) => (proxy.proxyConfig || '').trim().toLowerCase() === normalizedConfig)
    if (matchedByConfig?.proxyId) {
      return { proxyId: matchedByConfig.proxyId, proxyConfig: '' }
    }
    return { proxyId: '', proxyConfig: rawProxyConfig }
  }

  const directProxy = proxies.find((proxy) => proxy.proxyId === directProxyID)
  return { proxyId: directProxy?.proxyId || '', proxyConfig: '' }
}

export function BrowserEditPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const isCreate = id === 'new'
  const [formData, setFormData] = useState<BrowserProfileInput>({
    profileName: '',
    userDataDir: '',
    coreId: '',
    fingerprintArgs: [],
    proxyId: directProxyID,
    proxyConfig: '',
    launchArgs: [],
    tags: [],
    keywords: [],
    groupId: '',
  })
  const [cores, setCores] = useState<BrowserCore[]>([])
  const [proxies, setProxies] = useState<BrowserProxy[]>([])
  const [groups, setGroups] = useState<BrowserGroup[]>([])
  const [templates, setTemplates] = useState<BrowserProfileTemplate[]>([])
  const [selectedTemplateId, setSelectedTemplateId] = useState('')
  const [launchArgsText, setLaunchArgsText] = useState('')
  const [allTags, setAllTags] = useState<string[]>([])
  const [saving, setSaving] = useState(false)
  const [restartingAfterSave, setRestartingAfterSave] = useState(false)
  const [isInstanceRunning, setIsInstanceRunning] = useState(false)
  const [proxyPickerOpen, setProxyPickerOpen] = useState(false)
  const [isDirty, setIsDirty] = useState(false)
  const [leaveConfirm, setLeaveConfirm] = useState(false)
  const [saveError, setSaveError] = useState('')
  const [schemes, setSchemes] = useState<FingerprintScheme[]>([])
  const [schemeId, setSchemeId] = useState<string>('')

  const rerollDevice = async (targetSchemeId: string) => {
    if (!targetSchemeId || targetSchemeId === FINGERPRINT_SCHEME_NONE) return
    try {
      const args = await generateFingerprintSchemeArgs(targetSchemeId)
      if (args.length > 0) {
        setIsDirty(true)
        setFormData(prev => ({ ...prev, fingerprintArgs: args, fingerprintSchemeId: targetSchemeId }))
      }
    } catch (error: any) {
      toast.error(typeof error === 'string' ? error : error?.message || '生成设备指纹失败')
    }
  }

  const handleSchemeChange = (next: string) => {
    setSchemeId(next)
    setIsDirty(true)
    setFormData(prev => ({ ...prev, fingerprintSchemeId: next }))
    if (next !== FINGERPRINT_SCHEME_NONE) rerollDevice(next)
  }

  useEffect(() => {
    const loadData = async () => {
      const [coreList, proxyList, tagList, groupList, settings, templateList, schemeList] = await Promise.all([
        fetchBrowserCores(),
        fetchBrowserProxies(),
        fetchAllTags(),
        fetchGroups(),
        fetchBrowserSettings(),
        fetchBrowserProfileTemplates(),
        fetchFingerprintSchemes(),
      ])
      setSchemes(schemeList)
      const defaultScheme = schemeList.find(s => s.isDefault) || schemeList[0]
      const resolvedDefaultLaunchArgs = resolveDefaultLaunchArgs(settings.defaultLaunchArgs || [])
      setCores(coreList)
      setProxies(proxyList)
      setAllTags(tagList)
      setGroups(groupList)
      setTemplates(templateList)

      if (isCreate) {
        const resolved = resolvePoolProxySelection('', '', proxyList)
        const resolvedDefaultFingerprintArgs = resolveDefaultFingerprintArgs(settings.defaultFingerprintArgs || [])
        let generated: string[] = []
        if (defaultScheme) {
          try { generated = await generateFingerprintSchemeArgs(defaultScheme.schemeId) } catch { generated = [] }
        }
        setSchemeId(defaultScheme ? defaultScheme.schemeId : FINGERPRINT_SCHEME_NONE)
        setFormData((prev) => ({
          ...prev,
          fingerprintArgs: generated.length > 0 ? generated : (prev.fingerprintArgs.length > 0 ? prev.fingerprintArgs : resolvedDefaultFingerprintArgs),
          fingerprintSchemeId: defaultScheme ? defaultScheme.schemeId : FINGERPRINT_SCHEME_NONE,
          proxyId: resolved.proxyId || directProxyID,
          proxyConfig: '',
        }))
        setLaunchArgsText(resolvedDefaultLaunchArgs.join('\n'))
        return
      }
      const list = await fetchBrowserProfiles()
      const current = list.find(item => item.profileId === id)
      if (!current) return
      setIsInstanceRunning(Boolean(current.running))
      const currentLaunchArgs = normalizeLaunchArgs(current.launchArgs)
      const normalizedCoreId = !current.coreId || current.coreId.toLowerCase() === 'default'
        ? ''
        : current.coreId
      const resolvedProxy = resolvePoolProxySelection(current.proxyId || '', current.proxyConfig || '', proxyList)
      const currentSchemeId = current.fingerprintSchemeId || (defaultScheme ? defaultScheme.schemeId : FINGERPRINT_SCHEME_NONE)
      setSchemeId(currentSchemeId)
      setFormData({
        profileName: current.profileName,
        userDataDir: current.userDataDir,
        coreId: normalizedCoreId,
        fingerprintArgs: current.fingerprintArgs,
        fingerprintSchemeId: currentSchemeId,
        proxyId: resolvedProxy.proxyId,
        proxyConfig: resolvedProxy.proxyConfig,
        launchArgs: currentLaunchArgs,
        tags: current.tags,
        keywords: current.keywords || [],
        groupId: current.groupId || '',
      })
      setLaunchArgsText(currentLaunchArgs.join('\n'))
    }
    loadData()
  }, [id, isCreate])

  useEffect(() => {
    if (isCreate || !id) return
    const syncRunning = async (payload: any) => {
      const targetId = typeof payload === 'string' ? payload : payload?.profileId
      if (targetId && targetId !== id) return
      const list = await fetchBrowserProfiles()
      const current = list.find(item => item.profileId === id)
      if (current) setIsInstanceRunning(Boolean(current.running))
    }
    const offStarted = EventsOn('browser:instance:started', syncRunning)
    const offStopped = EventsOn('browser:instance:stopped', syncRunning)
    const offCrashed = EventsOn('browser:instance:crashed', syncRunning)
    return () => {
      offStarted?.()
      offStopped?.()
      offCrashed?.()
    }
  }, [id, isCreate])

  const handleChange = (field: keyof BrowserProfileInput, value: string | string[]) => {
    setIsDirty(true)
    setFormData(prev => {
      if (field === 'proxyId') {
        return { ...prev, proxyId: typeof value === 'string' ? value : '', proxyConfig: '' }
      }
      return { ...prev, [field]: value }
    })
  }

  const handleTemplateChange = (templateId: string) => {
    setSelectedTemplateId(templateId)
    const template = templates.find(item => item.templateId === templateId)
    if (!template) return
    setIsDirty(true)
    setFormData({
      profileName: template.profileName || template.templateName,
      userDataDir: template.userDataDir || '',
      coreId: template.coreId || '',
      fingerprintArgs: template.fingerprintArgs || [],
      proxyId: template.proxyId || directProxyID,
      proxyConfig: template.proxyConfig || '',
      launchArgs: template.launchArgs || [],
      tags: template.tags || [],
      keywords: template.keywords || [],
      groupId: template.groupId || '',
    })
    setLaunchArgsText(normalizeLaunchArgs(template.launchArgs || []).join('\n'))
  }

  const performSave = async (restartAfterSave: boolean) => {
    if (restartAfterSave) {
      setRestartingAfterSave(true)
    } else {
      setSaving(true)
    }
    const resolvedProxyId = (formData.proxyId || '').trim()
    const resolvedProxyConfig = (formData.proxyConfig || '').trim()
    const payload: BrowserProfileInput = {
      ...formData,
      proxyId: resolvedProxyId,
      proxyConfig: '',
      launchArgs: normalizeLaunchArgs(launchArgsText.split('\n')),
    }
    if (!resolvedProxyId) {
      if (resolvedProxyConfig) {
        payload.proxyConfig = resolvedProxyConfig
      } else {
        payload.proxyId = directProxyID
      }
    }
    try {
      if (isCreate) {
        await createBrowserProfile(payload)
        toast.success('配置已创建')
      } else if (id) {
        await updateBrowserProfile(id, payload)
        if (restartAfterSave) {
          await restartBrowserInstance(id)
          toast.success('配置已更新并重启实例，新配置已生效')
        } else if (isInstanceRunning) {
          toast.warning('配置已保存；当前实例仍在运行，需重启实例后新配置才会生效')
        } else {
          toast.success('配置已更新')
        }
      }
      setIsDirty(false)
      navigate('/browser/list')
    } catch (error: any) {
      setSaveError(typeof error === 'string' ? error : error?.message || '保存失败')
    } finally {
      setSaving(false)
      setRestartingAfterSave(false)
    }
  }

  const handleSave = () => performSave(false)
  const handleSaveAndRestart = () => performSave(true)

  const handleBack = () => {
    if (isDirty) { setLeaveConfirm(true) } else { navigate('/browser/list') }
  }

  const defaultCore = cores.find(c => c.isDefault)

  const handleOpenUserDataDir = async () => {
    if (!formData.userDataDir.trim()) {
      toast.error('请先输入用户数据目录')
      return
    }
    try {
      await openUserDataDir(formData.userDataDir)
    } catch (error: unknown) {
      toast.error((error as Error)?.message || '打开目录失败')
    }
  }

  const handleProxyListUpdated = (nextProxies: BrowserProxy[]) => {
    setProxies(nextProxies)
  }

  const handleProxyDeleted = (deletedProxyId: string, nextProxies: BrowserProxy[]) => {
    setProxies(nextProxies)
    if (formData.proxyId !== deletedProxyId) {
      return
    }

    const fallbackProxy = nextProxies.find((proxy) => proxy.proxyId === directProxyID)
    if (fallbackProxy) {
      handleChange('proxyId', fallbackProxy.proxyId)
      return
    }

    handleChange('proxyId', '')
  }

  return (
    <div className="space-y-5 animate-fade-in">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">{isCreate ? '新建配置' : '编辑配置'}</h1>
          <p className="text-sm text-[var(--color-text-muted)] mt-1">完善指纹与启动参数</p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={handleBack} disabled={saving || restartingAfterSave}>返回列表</Button>
          {!isCreate && isInstanceRunning ? (
            <>
              <Button variant="secondary" size="sm" onClick={handleSave} loading={saving} disabled={restartingAfterSave}>仅保存</Button>
              <Button size="sm" onClick={handleSaveAndRestart} loading={restartingAfterSave} disabled={saving}>
                <RotateCcw className="w-4 h-4 mr-1.5" />
                保存并重启生效
              </Button>
            </>
          ) : (
            <Button size="sm" onClick={handleSave} loading={saving}>保存配置</Button>
          )}
        </div>
      </div>

      {!isCreate && isInstanceRunning && (
        <div className="rounded-xl border border-amber-300/60 bg-amber-500/10 px-4 py-3 text-sm text-[var(--color-text-primary)]">
          当前实例正在运行中。修改时区、语言、代理或其它指纹设置后，点击右上角「保存并重启生效」即可关闭旧窗口并用新设置重新打开。
        </div>
      )}

      {isCreate && (
        <Card title="实例模板" subtitle="选择模板后会自动填充配置，保存前仍可继续修改">
          <FormItem label="选择模板">
            <Select
              value={selectedTemplateId}
              onChange={e => handleTemplateChange(e.target.value)}
              options={[
                { value: '', label: '不使用模板' },
                ...templates.map(item => ({ value: item.templateId, label: item.templateName })),
              ]}
            />
          </FormItem>
        </Card>
      )}

      <Card title="基础信息" subtitle="实例与配置名称">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormItem label="配置名称" required>
            <Input value={formData.profileName} onChange={e => handleChange('profileName', e.target.value)} placeholder="请输入配置名称" />
          </FormItem>
          <FormItem label="用户数据目录（留空自动生成）">
            <div className="flex gap-2">
              <Input
                value={formData.userDataDir}
                onChange={e => handleChange('userDataDir', e.target.value)}
                placeholder="留空自动生成"
                className="flex-1"
              />
              <Button variant="secondary" size="sm" onClick={handleOpenUserDataDir} title="在资源管理器中打开">
                <FolderOpen className="w-4 h-4" />
              </Button>
            </div>
          </FormItem>
          <FormItem label="内核">
            <Select
              value={formData.coreId}
              onChange={e => handleChange('coreId', e.target.value)}
              options={
                cores.length > 0 ? [
                  { value: '', label: defaultCore ? `使用默认 (${defaultCore.coreName})` : '使用默认内核' },
                  ...cores.map(c => ({ value: c.coreId, label: c.coreName })),
                ] : [
                  { value: '', label: '暂无内核，请添加内核' }
                ]
              }
            />
          </FormItem>
          <FormItem label="标签">
            <TagInput
              value={formData.tags}
              onChange={tags => handleChange('tags', tags)}
              suggestions={allTags}
              placeholder="输入标签后按回车，支持从已有标签选择"
            />
          </FormItem>
          <FormItem label="分组">
            <GroupSelector
              groups={groups}
              value={formData.groupId || ''}
              onChange={groupId => handleChange('groupId', groupId)}
              placeholder="未分组"
              className="w-full"
            />
          </FormItem>
        </div>
      </Card>

      <Card title="代理配置" subtitle="仅支持从代理池选择（包含直连节点）">
        <div className="grid grid-cols-1 gap-4">
          <FormItem label="代理池选择">
            <div className="flex gap-2">
              <Select
                value={formData.proxyId}
                onChange={e => handleChange('proxyId', e.target.value)}
                options={
                  proxies.length > 0 ? [
                    ...(formData.proxyId === '' && formData.proxyConfig
                      ? [{ value: '', label: '接口自定义代理（保持原值）' }]
                      : []),
                    ...proxies.map(p => ({ value: p.proxyId, label: p.proxyName || p.proxyId })),
                  ] : [{ value: '', label: '暂无代理，请先到代理池创建' }]
                }
                className="flex-1"
              />
              <Button variant="secondary" size="sm" onClick={() => setProxyPickerOpen(true)} title="按分组选择代理">
                <Layers className="w-4 h-4" />
              </Button>
            </div>
          </FormItem>
        </div>
        <p className="text-xs text-[var(--color-text-muted)] mt-2">
          已移除手动代理输入，实例默认按代理池节点生效。
          {formData.proxyId === '' && formData.proxyConfig ? ' 当前实例为接口自定义代理，未改动代理选择时会保持原值。' : ''}
        </p>
      </Card>

      <ProxyPickerModal
        open={proxyPickerOpen}
        currentProxyId={formData.proxyId}
        onSelect={proxy => handleChange('proxyId', proxy.proxyId)}
        onProxyListUpdated={handleProxyListUpdated}
        onProxyDeleted={handleProxyDeleted}
        onClose={() => setProxyPickerOpen(false)}
      />

      <Card title="指纹配置" subtitle="先选指纹方案，再按需微调设备项">
        <FormItem label="指纹方案" hint="方案固定时区、语言、品牌等必须一致的项；种子、显卡、核心数等设备特征每个窗口单独随机">
          <div className="flex gap-2">
            <div className="flex-1 min-w-0">
              <Select
                value={schemeId}
                onChange={e => handleSchemeChange(e.target.value)}
                options={[
                  ...schemes.map(s => ({ value: s.schemeId, label: s.isDefault ? `${s.schemeName}（默认）` : s.schemeName })),
                  { value: FINGERPRINT_SCHEME_NONE, label: '不使用方案（手动配置）' },
                ]}
              />
            </div>
            {schemeId !== FINGERPRINT_SCHEME_NONE && (
              <Button type="button" variant="secondary" onClick={() => rerollDevice(schemeId)}>重新随机设备</Button>
            )}
          </div>
        </FormItem>
        <FingerprintPanel
          value={formData.fingerprintArgs}
          onChange={args => handleChange('fingerprintArgs', args)}
        />
      </Card>

      <Card title="启动参数" subtitle={isCreate ? '新建时默认填入轻量参数模板，直接改这里即可' : '每行一个参数'}>
        <div className="space-y-2">
          <Textarea
            value={launchArgsText}
            onChange={e => { setLaunchArgsText(e.target.value); setIsDirty(true) }}
            rows={6}
            placeholder="--disable-sync"
          />
          {isCreate && (
            <p className="text-xs text-[var(--color-text-muted)]">这里默认就是轻量参数模板；需要更复杂的参数，直接在此基础上修改。</p>
          )}
        </div>
      </Card>

      <ConfirmModal
        open={leaveConfirm}
        onClose={() => setLeaveConfirm(false)}
        onConfirm={() => navigate('/browser/list')}
        title="放弃未保存的更改？"
        content="当前页面有未保存的修改，离开后将丢失这些更改。"
        confirmText="放弃并离开"
        cancelText="继续编辑"
        danger
      />

      <Modal
        open={!!saveError}
        onClose={() => setSaveError('')}
        title="保存失败"
        width="420px"
        footer={<Button onClick={() => setSaveError('')}>知道了</Button>}
      >
        <div className="text-[var(--color-text-secondary)]">{saveError}</div>
      </Modal>
    </div>
  )
}
