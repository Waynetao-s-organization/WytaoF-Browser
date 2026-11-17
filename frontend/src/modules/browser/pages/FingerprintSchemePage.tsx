import { useEffect, useMemo, useState } from 'react'
import { Edit2, Plus, Star, Trash2, Wand2 } from 'lucide-react'
import { Button, Card, ConfirmModal, FormItem, Input, Modal, Select, Table, Textarea, toast } from '../../../shared/components'
import type { TableColumn } from '../../../shared/components/Table'
import type { BrowserCore, FingerprintScheme, FingerprintSchemeInput, WeightedValue } from '../types'
import {
  applyFingerprintScheme,
  createFingerprintScheme,
  deleteFingerprintScheme,
  fetchBrowserCores,
  fetchBrowserProfiles,
  fetchFingerprintSchemes,
  setDefaultFingerprintScheme,
  updateFingerprintScheme,
} from '../api'

const PLATFORM_OPTIONS = [
  { value: 'windows', label: 'Windows' },
  { value: 'macos', label: 'macOS（本机是 Windows 时不建议）' },
  { value: 'linux', label: 'Linux' },
]
const BRAND_OPTIONS = ['Chrome', 'Edge', 'Opera', 'Vivaldi'].map(v => ({ value: v, label: v }))
const LANG_OPTIONS = ['en-US', 'en-GB', 'en-CA', 'en-AU', 'zh-CN', 'zh-TW', 'ja-JP', 'ko-KR', 'de-DE', 'fr-FR', 'es-ES', 'pt-BR'].map(v => ({ value: v, label: v }))
const TIMEZONE_OPTIONS = [
  'America/Los_Angeles', 'America/Denver', 'America/Phoenix', 'America/Chicago', 'America/New_York',
  'America/Vancouver', 'America/Toronto', 'Europe/London', 'Europe/Berlin', 'Asia/Tokyo', 'Asia/Singapore', 'Asia/Hong_Kong', 'Asia/Shanghai', 'Australia/Sydney', 'UTC',
].map(v => ({ value: v, label: v }))
const WEBRTC_OPTIONS = [
  { value: 'disable_non_proxied_udp', label: '禁用非代理 UDP（推荐，防真实 IP 泄露）' },
  { value: 'default_public_interface_only', label: '仅公网接口' },
  { value: 'default_public_and_private_interfaces', label: '公网 + 私网接口' },
]
const YES_NO = [
  { value: 'true', label: '开启' },
  { value: 'false', label: '关闭' },
]

function poolToText(pool: WeightedValue[] | undefined): string {
  return (pool || []).map(item => `${item.value}:${item.weight}`).join(', ')
}

function textToPool(text: string): WeightedValue[] {
  return text.split(/[,，\n]/).map(s => s.trim()).filter(Boolean).map(item => {
    const [value, weight] = item.split(':').map(s => s.trim())
    const w = Number.parseInt(weight || '1', 10)
    return { value, weight: Number.isFinite(w) && w > 0 ? w : 1 }
  })
}

function poolSummary(pool: WeightedValue[] | undefined): string {
  return (pool || []).map(item => item.value).join(' / ') || '—'
}

interface FormState {
  schemeName: string
  description: string
  platform: string
  brand: string
  lang: string
  timezone: string
  webrtcPolicy: string
  canvasNoise: boolean
  gpuSpoof: boolean
  hideWebGPU: boolean
  speechApi: string
  coreId: string
  maximized: boolean
  cores: string
  memory: string
  platformVersions: string
  windowSizes: string
}

function emptyForm(): FormState {
  return {
    schemeName: '', description: '', platform: 'windows', brand: 'Chrome', lang: 'en-US', timezone: 'America/Los_Angeles',
    webrtcPolicy: 'disable_non_proxied_udp', canvasNoise: true, gpuSpoof: true, hideWebGPU: true, speechApi: 'disable', coreId: '', maximized: true,
    cores: '4:15, 6:20, 8:35, 12:15, 16:10', memory: '8:100', platformVersions: '10.0.0:30, 15.0.0:30, 19.0.0:40', windowSizes: '1920,1080:40, 1536,864:20, 1366,768:15, 1440,900:10, 1600,900:10',
  }
}

function formFromScheme(s: FingerprintScheme): FormState {
  return {
    schemeName: s.schemeName, description: s.description || '',
    platform: s.fixed.platform || 'windows', brand: s.fixed.brand || 'Chrome', lang: s.fixed.lang || '', timezone: s.fixed.timezone || '',
    webrtcPolicy: s.fixed.webrtcPolicy || 'disable_non_proxied_udp', canvasNoise: s.fixed.canvasNoise, gpuSpoof: s.fixed.gpuSpoof, hideWebGPU: s.fixed.hideWebGPU,
    speechApi: s.fixed.speechApi || 'disable', coreId: s.fixed.coreId || '', maximized: s.fixed.maximized,
    cores: poolToText(s.device.cores), memory: poolToText(s.device.memory), platformVersions: poolToText(s.device.platformVersions), windowSizes: poolToText(s.device.windowSizes),
  }
}

function inputFromForm(f: FormState): FingerprintSchemeInput {
  return {
    schemeName: f.schemeName.trim(), description: f.description.trim(),
    fixed: {
      platform: f.platform, brand: f.brand, lang: f.lang, timezone: f.timezone, webrtcPolicy: f.webrtcPolicy,
      canvasNoise: f.canvasNoise, gpuSpoof: f.gpuSpoof, hideWebGPU: f.hideWebGPU, speechApi: f.speechApi, coreId: f.coreId, maximized: f.maximized,
    },
    device: { cores: textToPool(f.cores), memory: textToPool(f.memory), platformVersions: textToPool(f.platformVersions), windowSizes: textToPool(f.windowSizes) },
  }
}

export function FingerprintSchemePage() {
  const [schemes, setSchemes] = useState<FingerprintScheme[]>([])
  const [cores, setCores] = useState<BrowserCore[]>([])
  const [boundCount, setBoundCount] = useState<Record<string, number>>({})
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<FingerprintScheme | null>(null)
  const [form, setForm] = useState<FormState>(emptyForm())
  const [deleteTarget, setDeleteTarget] = useState<FingerprintScheme | null>(null)
  const [applyTarget, setApplyTarget] = useState<{ scheme: FingerprintScheme; reroll: boolean } | null>(null)

  const load = async () => {
    setLoading(true)
    try {
      const [list, coreList, profiles] = await Promise.all([fetchFingerprintSchemes(), fetchBrowserCores(), fetchBrowserProfiles()])
      setSchemes(list)
      setCores(coreList)
      const counts: Record<string, number> = {}
      for (const p of profiles) {
        const id = p.fingerprintSchemeId || ''
        counts[id] = (counts[id] || 0) + 1
      }
      setBoundCount(counts)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  const coreOptions = useMemo(() => [
    { value: '', label: '跟随系统默认内核' },
    ...cores.map(c => ({ value: c.coreId, label: c.coreName })),
  ], [cores])

  const openCreate = () => { setEditing(null); setForm(emptyForm()); setModalOpen(true) }
  const openEdit = (s: FingerprintScheme) => { setEditing(s); setForm(formFromScheme(s)); setModalOpen(true) }
  const set = (patch: Partial<FormState>) => setForm(prev => ({ ...prev, ...patch }))

  const handleSave = async () => {
    const payload = inputFromForm(form)
    if (!payload.schemeName) { toast.error('请输入方案名称'); return }
    setSaving(true)
    try {
      if (editing) {
        await updateFingerprintScheme(editing.schemeId, payload)
        toast.success('方案已更新；绑定它的窗口下次启动自动按新固定项生效')
      } else {
        await createFingerprintScheme(payload)
        toast.success('方案已创建')
      }
      setModalOpen(false)
      await load()
    } catch (error: any) {
      toast.error(typeof error === 'string' ? error : error?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const handleSetDefault = async (s: FingerprintScheme) => {
    try {
      await setDefaultFingerprintScheme(s.schemeId)
      toast.success(`已把「${s.schemeName}」设为默认方案`)
      await load()
    } catch (error: any) {
      toast.error(typeof error === 'string' ? error : error?.message || '设置失败')
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    try {
      await deleteFingerprintScheme(deleteTarget.schemeId)
      toast.success('方案已删除，原绑定窗口改回默认方案')
      setDeleteTarget(null)
      await load()
    } catch (error: any) {
      toast.error(typeof error === 'string' ? error : error?.message || '删除失败')
    }
  }

  const handleApply = async () => {
    if (!applyTarget) return
    try {
      const result = await applyFingerprintScheme(applyTarget.scheme.schemeId, [], applyTarget.reroll)
      toast.success(`已套用到 ${result.updated} 个窗口（跳过 ${result.skipped} 个，共 ${result.total} 个）`)
      if (result.errors?.length) toast.warning(result.errors.slice(0, 3).join('；'))
      setApplyTarget(null)
      await load()
    } catch (error: any) {
      toast.error(typeof error === 'string' ? error : error?.message || '套用失败')
    }
  }

  const columns: TableColumn<FingerprintScheme>[] = [
    {
      key: 'schemeName', title: '方案', render: (_value, s) => (
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span className="font-medium text-[var(--color-text-primary)]">{s.schemeName}</span>
            {s.isDefault && <span className="text-[11px] px-1.5 py-0.5 rounded bg-[var(--color-primary)] text-white">默认</span>}
            {s.builtin && <span className="text-[11px] px-1.5 py-0.5 rounded bg-[var(--color-bg-hover)] text-[var(--color-text-muted)]">内置</span>}
          </div>
          {s.description && <p className="text-xs text-[var(--color-text-muted)] mt-1 leading-relaxed">{s.description}</p>}
        </div>
      ),
    },
    {
      key: 'fixed', title: '固定项（同方案所有窗口一致）', render: (_value, s) => (
        <div className="text-xs text-[var(--color-text-secondary)] space-y-0.5">
          <div>{s.fixed.platform || '—'} · {s.fixed.brand || 'Chrome'} · {s.fixed.lang || '—'} · {s.fixed.timezone || '—'}</div>
          <div>画布噪声 {s.fixed.canvasNoise ? '开' : '关'} · 显卡伪装 {s.fixed.gpuSpoof ? '开' : '关'} · WebGPU {s.fixed.hideWebGPU ? '隐藏' : '可见'} · 语音 {s.fixed.speechApi === 'enable' ? '保留' : '屏蔽'}</div>
          <div>WebRTC {s.fixed.webrtcPolicy || '默认'} · 内核 {s.fixed.coreId || '系统默认'} · {s.fixed.maximized ? '窗口最大化' : '按尺寸池'}</div>
        </div>
      ),
    },
    {
      key: 'device', title: '设备池（每个窗口随机抽取）', render: (_value, s) => (
        <div className="text-xs text-[var(--color-text-secondary)] space-y-0.5">
          <div>核心数：{poolSummary(s.device.cores)}</div>
          <div>系统版本：{poolSummary(s.device.platformVersions)}</div>
          {!s.fixed.maximized && <div>窗口尺寸：{poolSummary(s.device.windowSizes)}</div>}
          <div>种子：每个窗口独立随机</div>
        </div>
      ),
    },
    { key: 'bound', title: '绑定窗口', render: (_value, s) => <span className="text-sm">{boundCount[s.schemeId] || 0}</span> },
    {
      key: 'actions', title: '操作', render: (_value, s) => (
        <div className="flex flex-wrap gap-1.5">
          <Button size="sm" variant="secondary" onClick={() => openEdit(s)}><Edit2 className="w-3.5 h-3.5" /> 编辑</Button>
          {!s.isDefault && <Button size="sm" variant="secondary" onClick={() => handleSetDefault(s)}><Star className="w-3.5 h-3.5" /> 设为默认</Button>}
          <Button size="sm" variant="secondary" onClick={() => setApplyTarget({ scheme: s, reroll: false })}><Wand2 className="w-3.5 h-3.5" /> 套用到全部窗口</Button>
          {!s.isDefault && <Button size="sm" variant="danger" onClick={() => setDeleteTarget(s)}><Trash2 className="w-3.5 h-3.5" /> 删除</Button>}
        </div>
      ),
    },
  ]

  return (
    <div className="space-y-4">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">指纹方案</h1>
          <p className="text-sm text-[var(--color-text-muted)] mt-1">
            方案只固定「必须一致」的部分（系统、品牌、语言、时区、WebRTC、画布与语音策略）；种子、显卡、核心数、系统版本等设备特征由每个窗口单独随机，同一方案下的不同窗口不会被当成同一台设备。新建窗口时不选方案就按默认方案生效，已有窗口在启动时也会按各自绑定的方案强制套用固定项。
          </p>
        </div>
        <Button onClick={openCreate}><Plus className="w-4 h-4" /> 新建方案</Button>
      </div>

      <Card>
        <Table columns={columns} data={schemes} rowKey={(s) => s.schemeId} loading={loading} emptyText="还没有指纹方案" />
      </Card>

      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title={editing ? `编辑方案：${editing.schemeName}` : '新建指纹方案'}>
        <div className="space-y-5">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <FormItem label="方案名称"><Input value={form.schemeName} onChange={e => set({ schemeName: e.target.value })} placeholder="如：美西洛杉矶 · Windows 台式机" /></FormItem>
            <FormItem label="说明"><Input value={form.description} onChange={e => set({ description: e.target.value })} placeholder="什么出口、什么场景用" /></FormItem>
          </div>
          <div>
            <p className="text-xs font-medium text-[var(--color-text-muted)] mb-2 uppercase tracking-wide">固定项（必须与代理出口一致）</p>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <FormItem label="操作系统"><Select value={form.platform} onChange={e => set({ platform: e.target.value })} options={PLATFORM_OPTIONS} /></FormItem>
              <FormItem label="浏览器品牌"><Select value={form.brand} onChange={e => set({ brand: e.target.value })} options={BRAND_OPTIONS} /></FormItem>
              <FormItem label="语言"><Select value={form.lang} onChange={e => set({ lang: e.target.value })} options={LANG_OPTIONS} /></FormItem>
              <FormItem label="时区（按出口 IP 所在地选）"><Select value={form.timezone} onChange={e => set({ timezone: e.target.value })} options={TIMEZONE_OPTIONS} /></FormItem>
              <FormItem label="WebRTC 策略"><Select value={form.webrtcPolicy} onChange={e => set({ webrtcPolicy: e.target.value })} options={WEBRTC_OPTIONS} /></FormItem>
              <FormItem label="指定内核"><Select value={form.coreId} onChange={e => set({ coreId: e.target.value })} options={coreOptions} /></FormItem>
              <FormItem label="画布按种子加噪声（多号防关联开、单号高风控关）"><Select value={String(form.canvasNoise)} onChange={e => set({ canvasNoise: e.target.value === 'true' })} options={YES_NO} /></FormItem>
              <FormItem label="显卡型号按种子伪装"><Select value={String(form.gpuSpoof)} onChange={e => set({ gpuSpoof: e.target.value === 'true' })} options={YES_NO} /></FormItem>
              <FormItem label="隐藏 WebGPU（避免暴露真实显卡）"><Select value={String(form.hideWebGPU)} onChange={e => set({ hideWebGPU: e.target.value === 'true' })} options={YES_NO} /></FormItem>
              <FormItem label="系统朗读语音"><Select value={form.speechApi} onChange={e => set({ speechApi: e.target.value })} options={[{ value: 'disable', label: '屏蔽（防暴露中文系统语音）' }, { value: 'enable', label: '保留' }]} /></FormItem>
              <FormItem label="窗口打开方式"><Select value={String(form.maximized)} onChange={e => set({ maximized: e.target.value === 'true' })} options={[{ value: 'true', label: '最大化（和真实用户一致）' }, { value: 'false', label: '按下方窗口尺寸池随机' }]} /></FormItem>
            </div>
          </div>
          <div>
            <p className="text-xs font-medium text-[var(--color-text-muted)] mb-2 uppercase tracking-wide">设备池（写法：值:权重，逗号分隔；每个新窗口按权重随机抽一个）</p>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <FormItem label="CPU 核心数池"><Input value={form.cores} onChange={e => set({ cores: e.target.value })} placeholder="4:15, 6:20, 8:35, 12:15, 16:10" /></FormItem>
              <FormItem label="设备内存池（最大 8，Chrom-150 起生效）"><Input value={form.memory} onChange={e => set({ memory: e.target.value })} placeholder="8:100" /></FormItem>
              <FormItem label="系统版本池（Windows：10.0.0=Win10，15.0.0=Win11 23H2，19.0.0=Win11 24H2）"><Input value={form.platformVersions} onChange={e => set({ platformVersions: e.target.value })} placeholder="10.0.0:30, 15.0.0:30, 19.0.0:40" /></FormItem>
              <FormItem label="窗口尺寸池（仅非最大化时用）"><Textarea rows={2} value={form.windowSizes} onChange={e => set({ windowSizes: e.target.value })} placeholder="1920,1080:40, 1536,864:20" /></FormItem>
            </div>
          </div>
          <div className="flex justify-end gap-2">
            <Button variant="secondary" onClick={() => setModalOpen(false)}>取消</Button>
            <Button onClick={handleSave} loading={saving}>{editing ? '保存修改' : '创建方案'}</Button>
          </div>
        </div>
      </Modal>

      <ConfirmModal
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDelete}
        title="删除指纹方案"
        content={`确定删除「${deleteTarget?.schemeName || ''}」？绑定它的 ${boundCount[deleteTarget?.schemeId || ''] || 0} 个窗口会改回默认方案（设备特征不变）。`}
        confirmText="删除"
        danger
      />

      <ConfirmModal
        open={!!applyTarget}
        onClose={() => setApplyTarget(null)}
        onConfirm={handleApply}
        title="把方案套用到全部窗口"
        content={`将把「${applyTarget?.scheme.schemeName || ''}」的固定项写入全部未在运行的窗口（时区、语言、品牌等统一；每个窗口自己的种子、核心数等设备特征保持不变，已登录账号不会变成新设备）。正在运行的窗口会跳过，关闭后再套用一次即可。`}
        confirmText="确定套用"
      />
    </div>
  )
}
