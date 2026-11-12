import { useEffect, useMemo, useRef, useState } from 'react'
import { ChevronDown, ChevronUp, RefreshCw, Wand2 } from 'lucide-react'
import { ConfirmModal, FormItem, Input, Select, Textarea } from '../../../shared/components'
import {
  type FingerprintConfig,
  FINGERPRINT_PRESETS,
  PRESET_RESOLUTIONS,
  deserialize,
  getSystemTimezone,
  randomFingerprintSeed,
  serialize,
} from '../utils/fingerprintSerializer'

interface FingerprintPanelProps {
  value: string[]
  onChange: (args: string[]) => void
}

const BRAND_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'Chrome', label: 'Chrome' },
  { value: 'Edge', label: 'Edge' },
  { value: 'Firefox', label: 'Firefox' },
  { value: 'Safari', label: 'Safari' },
]

const PLATFORM_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'windows', label: 'Windows' },
  { value: 'macos', label: 'macOS' },
  { value: 'linux', label: 'Linux' },
]

const LANG_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'zh-CN', label: '简体中文 (zh-CN)' },
  { value: 'zh-TW', label: '繁體中文 (zh-TW)' },
  { value: 'en-US', label: 'English (en-US)' },
  { value: 'en-GB', label: 'English (en-GB)' },
  { value: 'ja-JP', label: '日本語 (ja-JP)' },
  { value: 'ko-KR', label: '한국어 (ko-KR)' },
  { value: 'fr-FR', label: 'Français (fr-FR)' },
  { value: 'de-DE', label: 'Deutsch (de-DE)' },
  { value: 'es-ES', label: 'Español (es-ES)' },
  { value: 'pt-BR', label: 'Português (pt-BR)' },
  { value: 'ru-RU', label: 'Русский (ru-RU)' },
  { value: 'vi-VN', label: 'Tiếng Việt (vi-VN)' },
  { value: 'th-TH', label: 'ไทย (th-TH)' },
  { value: 'id-ID', label: 'Bahasa Indonesia (id-ID)' },
]

const TIMEZONE_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'system', label: '跟随系统时区' },
  { value: 'UTC', label: 'UTC (UTC+0)' },
  // 亚洲
  { value: 'Asia/Shanghai', label: 'Asia/Shanghai (中国标准时间 UTC+8)' },
  { value: 'Asia/Hong_Kong', label: 'Asia/Hong_Kong (香港 UTC+8)' },
  { value: 'Asia/Taipei', label: 'Asia/Taipei (台湾 UTC+8)' },
  { value: 'Asia/Singapore', label: 'Asia/Singapore (新加坡 UTC+8)' },
  { value: 'Asia/Tokyo', label: 'Asia/Tokyo (东京 UTC+9)' },
  { value: 'Asia/Seoul', label: 'Asia/Seoul (首尔 UTC+9)' },
  { value: 'Asia/Bangkok', label: 'Asia/Bangkok (曼谷 UTC+7)' },
  { value: 'Asia/Ho_Chi_Minh', label: 'Asia/Ho_Chi_Minh (胡志明 UTC+7)' },
  { value: 'Asia/Jakarta', label: 'Asia/Jakarta (雅加达 UTC+7)' },
  { value: 'Asia/Kuala_Lumpur', label: 'Asia/Kuala_Lumpur (吉隆坡 UTC+8)' },
  { value: 'Asia/Manila', label: 'Asia/Manila (马尼拉 UTC+8)' },
  { value: 'Asia/Kolkata', label: 'Asia/Kolkata (印度 UTC+5:30)' },
  { value: 'Asia/Dubai', label: 'Asia/Dubai (迪拜 UTC+4)' },
  // 美洲
  { value: 'America/Los_Angeles', label: 'America/Los_Angeles (美西洛杉矶)' },
  { value: 'America/Denver', label: 'America/Denver (美国山地丹佛)' },
  { value: 'America/Phoenix', label: 'America/Phoenix (美国亚利桑那)' },
  { value: 'America/Chicago', label: 'America/Chicago (美中芝加哥)' },
  { value: 'America/New_York', label: 'America/New_York (美东纽约)' },
  { value: 'America/Vancouver', label: 'America/Vancouver (加拿大温哥华)' },
  { value: 'America/Toronto', label: 'America/Toronto (加拿大多伦多)' },
  { value: 'America/Mexico_City', label: 'America/Mexico_City (墨西哥城)' },
  { value: 'America/Sao_Paulo', label: 'America/Sao_Paulo (巴西圣保罗)' },
  // 欧洲
  { value: 'Europe/London', label: 'Europe/London (英国伦敦)' },
  { value: 'Europe/Paris', label: 'Europe/Paris (法国巴黎)' },
  { value: 'Europe/Berlin', label: 'Europe/Berlin (德国柏林/法兰克福)' },
  { value: 'Europe/Amsterdam', label: 'Europe/Amsterdam (荷兰阿姆斯特丹)' },
  { value: 'Europe/Zurich', label: 'Europe/Zurich (瑞士苏黎世)' },
  { value: 'Europe/Madrid', label: 'Europe/Madrid (西班牙马德里)' },
  { value: 'Europe/Rome', label: 'Europe/Rome (意大利罗马)' },
  { value: 'Europe/Warsaw', label: 'Europe/Warsaw (波兰华沙)' },
  { value: 'Europe/Istanbul', label: 'Europe/Istanbul (土耳其伊斯坦布尔)' },
  { value: 'Europe/Moscow', label: 'Europe/Moscow (莫斯科)' },
  // 大洋洲
  { value: 'Australia/Sydney', label: 'Australia/Sydney (澳大利亚悉尼)' },
  { value: 'Pacific/Auckland', label: 'Pacific/Auckland (新西兰奥克兰)' },
  { value: 'custom', label: '自定义时区...' },
]

const RESOLUTION_OPTIONS = [
  { value: '', label: '不设置' },
  ...PRESET_RESOLUTIONS.map(r => ({ value: r, label: r })),
  { value: 'custom', label: '自定义...' },
]

const WEBGL_VENDOR_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'Intel', label: 'Intel' },
  { value: 'NVIDIA', label: 'NVIDIA' },
  { value: 'AMD', label: 'AMD' },
  { value: 'Apple', label: 'Apple' },
]

const WEBGL_RENDERER_OPTIONS: Record<string, { value: string; label: string }[]> = {
  Intel: [
    { value: '', label: '不设置' },
    { value: 'Intel(R) UHD Graphics 630', label: 'UHD Graphics 630' },
    { value: 'Intel(R) UHD Graphics 620', label: 'UHD Graphics 620' },
    { value: 'Intel(R) HD Graphics 520', label: 'HD Graphics 520' },
    { value: 'Intel(R) Iris(R) Xe Graphics', label: 'Iris Xe Graphics' },
    { value: 'custom', label: '自定义...' },
  ],
  NVIDIA: [
    { value: '', label: '不设置' },
    { value: 'NVIDIA GeForce RTX 3080', label: 'GeForce RTX 3080' },
    { value: 'NVIDIA GeForce RTX 3060', label: 'GeForce RTX 3060' },
    { value: 'NVIDIA GeForce GTX 1660', label: 'GeForce GTX 1660' },
    { value: 'NVIDIA GeForce GTX 1080 Ti', label: 'GeForce GTX 1080 Ti' },
    { value: 'custom', label: '自定义...' },
  ],
  AMD: [
    { value: '', label: '不设置' },
    { value: 'AMD Radeon RX 6600', label: 'Radeon RX 6600' },
    { value: 'AMD Radeon RX 580', label: 'Radeon RX 580' },
    { value: 'AMD Radeon Vega 8', label: 'Radeon Vega 8' },
    { value: 'custom', label: '自定义...' },
  ],
  Apple: [
    { value: '', label: '不设置' },
    { value: 'Apple M1', label: 'Apple M1' },
    { value: 'Apple M2', label: 'Apple M2' },
    { value: 'Apple M3', label: 'Apple M3' },
    { value: 'custom', label: '自定义...' },
  ],
}

const BOOL_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'true', label: '启用' },
  { value: 'false', label: '禁用' },
]

const HARDWARE_CONCURRENCY_OPTIONS = [
  { value: '', label: '不设置' },
  { value: '2', label: '2 核' },
  { value: '4', label: '4 核' },
  { value: '6', label: '6 核' },
  { value: '8', label: '8 核' },
  { value: '10', label: '10 核' },
  { value: '12', label: '12 核' },
  { value: '16', label: '16 核' },
]

const DEVICE_MEMORY_OPTIONS = [
  { value: '', label: '不设置' },
  { value: '2', label: '2 GB' },
  { value: '4', label: '4 GB' },
  { value: '8', label: '8 GB' },
  { value: '16', label: '16 GB' },
  { value: '32', label: '32 GB' },
]

const COLOR_DEPTH_OPTIONS = [
  { value: '', label: '不设置' },
  { value: '24', label: '24 位（标准）' },
  { value: '30', label: '30 位（HDR）' },
  { value: '32', label: '32 位' },
]

const WEBRTC_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'disable_non_proxied_udp', label: '禁用非代理 UDP（推荐）' },
  { value: 'default_public_interface_only', label: '仅公网接口' },
  { value: 'default_public_and_private_interfaces', label: '公网+私网接口' },
]

const TOUCH_POINTS_OPTIONS = [
  { value: '', label: '不设置' },
  { value: '0', label: '0（无触摸）' },
  { value: '1', label: '1 点触摸' },
  { value: '5', label: '5 点触摸' },
  { value: '10', label: '10 点触摸' },
]

const PRESET_OPTIONS = [
  { value: '', label: '选择预设...' },
  ...FINGERPRINT_PRESETS.map(p => ({ value: p.id, label: p.name })),
]

function parseFingerprintValue(args: string[]): FingerprintConfig {
  const parsed = deserialize(args)
  if (parsed.timezone && !TIMEZONE_OPTIONS.some(o => o.value === parsed.timezone && o.value !== 'custom')) {
    parsed.customTimezone = parsed.timezone
    parsed.timezone = 'custom'
  }
  return parsed
}

export function FingerprintPanel({ value, onChange }: FingerprintPanelProps) {
  const [config, setConfig] = useState<FingerprintConfig>(() => parseFingerprintValue(value))
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const [customRendererMode, setCustomRendererMode] = useState(false)
  const [confirmSeedOpen, setConfirmSeedOpen] = useState(false)
  const lastEmittedRef = useRef<string>(value.join('\n'))

  useEffect(() => {
    const joined = value.join('\n')
    if (joined === lastEmittedRef.current) {
      return
    }
    lastEmittedRef.current = joined
    setConfig(parseFingerprintValue(value))
    setCustomRendererMode(false)
  }, [value.join('\n')])

  const emitConfig = (next: FingerprintConfig) => {
    setConfig(next)
    const serialized = serialize(next)
    lastEmittedRef.current = serialized.join('\n')
    onChange(serialized)
  }

  const update = (patch: Partial<FingerprintConfig>) => {
    emitConfig({ ...config, ...patch })
  }

  const handlePresetChange = (presetId: string) => {
    if (!presetId) return
    const preset = FINGERPRINT_PRESETS.find(p => p.id === presetId)
    if (!preset) return
    // 应用预设时自动生成新种子，保留未知参数
    const next: FingerprintConfig = {
      ...preset.config,
      seed: randomFingerprintSeed(),
      unknownArgs: config.unknownArgs,
    }
    setCustomRendererMode(false)
    emitConfig(next)
  }

  const handleAdvancedChange = (text: string) => {
    const args = text.split('\n').map(s => s.trim()).filter(Boolean)
    const parsed = deserialize(args)
    if (parsed.timezone && !TIMEZONE_OPTIONS.some(o => o.value === parsed.timezone && o.value !== 'custom')) {
      parsed.customTimezone = parsed.timezone
      parsed.timezone = 'custom'
    }
    emitConfig(parsed)
  }

  const langOptions = useMemo(() => {
    if (config.lang && !LANG_OPTIONS.some(o => o.value === config.lang)) {
      return [...LANG_OPTIONS, { value: config.lang, label: `${config.lang} (当前设置)` }]
    }
    return LANG_OPTIONS
  }, [config.lang])

  const timezoneSelectValue = useMemo(() => {
    if (!config.timezone) return ''
    if (config.timezone === 'custom') return 'custom'
    if (TIMEZONE_OPTIONS.some(o => o.value === config.timezone)) return config.timezone
    return 'custom'
  }, [config.timezone])

  const customTimezoneValue = config.timezone === 'custom'
    ? (config.customTimezone ?? '')
    : (timezoneSelectValue === 'custom' ? (config.timezone ?? '') : '')

  const rendererOptions = config.webglVendor
    ? (WEBGL_RENDERER_OPTIONS[config.webglVendor] ?? [{ value: '', label: '不设置' }, { value: 'custom', label: '自定义...' }])
    : [{ value: '', label: '不设置' }]

  const isCustomRenderer = customRendererMode || (config.webglRenderer
    ? !rendererOptions.some(o => o.value === config.webglRenderer && o.value !== 'custom')
    : false)

  const advancedText = serialize(config).join('\n')

  return (
    <div className="space-y-4">
      {/* 指纹种子 */}
      <div className="p-3 rounded-lg bg-[var(--color-bg-hover)] border border-[var(--color-border)] space-y-2">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium text-[var(--color-text-muted)] uppercase tracking-wide">指纹种子（Fingerprint Seed）</span>
          <span className="text-xs text-[var(--color-text-muted)]">决定所有随机噪声的根值，不同种子 = 不同指纹</span>
        </div>
        <div className="flex items-center gap-2">
          <Input
            value={config.seed ?? ''}
            onChange={e => update({ seed: e.target.value || undefined })}
            placeholder="留空则由系统按 ProfileId 自动生成"
            className="flex-1 font-mono text-sm"
          />
          <button
            type="button"
            title="随机生成新种子"
            onClick={() => {
              if (config.seed) {
                setConfirmSeedOpen(true)
              } else {
                update({ seed: randomFingerprintSeed() })
              }
            }}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs bg-[var(--color-primary)] text-white hover:opacity-90 transition-opacity shrink-0"
          >
            <RefreshCw className="w-3.5 h-3.5" />
            随机
          </button>
        </div>
      </div>

      <ConfirmModal
        open={confirmSeedOpen}
        onClose={() => setConfirmSeedOpen(false)}
        onConfirm={() => update({ seed: randomFingerprintSeed() })}
        title="重新生成指纹种子"
        content="重新生成后，当前指纹将完全改变，浏览器的 Canvas、WebGL、Audio 等所有噪声特征都会随之变化。确定继续？"
        confirmText="确定重新生成"
        danger
      />

      {/* 预设选择 */}
      <div className="flex items-center gap-3 p-3 rounded-lg bg-[var(--color-bg-hover)] border border-[var(--color-border)]">
        <Wand2 className="w-4 h-4 text-[var(--color-text-muted)] shrink-0" />
        <div className="flex-1 min-w-0">
          <Select
            value=""
            onChange={e => handlePresetChange(e.target.value)}
            options={PRESET_OPTIONS}
          />
        </div>
        <span className="text-xs text-[var(--color-text-muted)] shrink-0">选择后覆盖当前配置</span>
      </div>

      {/* 基础身份 */}
      <div>
        <p className="text-xs font-medium text-[var(--color-text-muted)] mb-2 uppercase tracking-wide">基础身份</p>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormItem label="浏览器品牌">
            <Select value={config.brand ?? ''} onChange={e => update({ brand: e.target.value || undefined })} options={BRAND_OPTIONS} />
          </FormItem>
          <FormItem label="操作系统">
            <Select value={config.platform ?? ''} onChange={e => update({ platform: e.target.value || undefined })} options={PLATFORM_OPTIONS} />
          </FormItem>
          <FormItem label="语言">
            <Select value={config.lang ?? ''} onChange={e => update({ lang: e.target.value || undefined })} options={langOptions} />
          </FormItem>
          <FormItem label="时区">
            <Select
              value={timezoneSelectValue}
              onChange={e => {
                const val = e.target.value
                if (val === 'custom') {
                  update({ timezone: 'custom', customTimezone: customTimezoneValue })
                } else {
                  update({ timezone: val || undefined, customTimezone: undefined })
                }
              }}
              options={TIMEZONE_OPTIONS.map(opt =>
                opt.value === 'system'
                  ? { ...opt, label: `跟随系统时区 (当前: ${getSystemTimezone()})` }
                  : opt
              )}
            />
          </FormItem>
          {timezoneSelectValue === 'custom' && (
            <FormItem label="自定义时区（IANA 名称）">
              <Input
                value={customTimezoneValue}
                onChange={e => update({ timezone: 'custom', customTimezone: e.target.value || undefined })}
                placeholder="如 America/Phoenix 或 Europe/Amsterdam"
              />
            </FormItem>
          )}
        </div>
      </div>

      {/* 屏幕与硬件 */}
      <div>
        <p className="text-xs font-medium text-[var(--color-text-muted)] mb-2 uppercase tracking-wide">屏幕与硬件</p>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormItem label="分辨率">
            <Select
              value={config.resolution ?? ''}
              onChange={e => update({ resolution: e.target.value || undefined, customResolution: undefined })}
              options={RESOLUTION_OPTIONS}
            />
          </FormItem>
          {config.resolution === 'custom' && (
            <FormItem label="自定义分辨率">
              <Input value={config.customResolution ?? ''} onChange={e => update({ customResolution: e.target.value || undefined })} placeholder="1600,900" />
            </FormItem>
          )}
          <FormItem label="色深">
            <Select value={config.colorDepth ?? ''} onChange={e => update({ colorDepth: e.target.value || undefined })} options={COLOR_DEPTH_OPTIONS} />
          </FormItem>
          <FormItem label="CPU 核心数">
            <Select value={config.hardwareConcurrency ?? ''} onChange={e => update({ hardwareConcurrency: e.target.value || undefined })} options={HARDWARE_CONCURRENCY_OPTIONS} />
          </FormItem>
          <FormItem label="设备内存">
            <Select value={config.deviceMemory ?? ''} onChange={e => update({ deviceMemory: e.target.value || undefined })} options={DEVICE_MEMORY_OPTIONS} />
          </FormItem>
          <FormItem label="触摸点数">
            <Select value={config.touchPoints ?? ''} onChange={e => update({ touchPoints: e.target.value || undefined })} options={TOUCH_POINTS_OPTIONS} />
          </FormItem>
        </div>
      </div>

      {/* 渲染指纹 */}
      <div>
        <p className="text-xs font-medium text-[var(--color-text-muted)] mb-2 uppercase tracking-wide">渲染指纹</p>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormItem label="WebGL 供应商">
            <Select
              value={config.webglVendor ?? ''}
              onChange={e => {
                setCustomRendererMode(false)
                update({ webglVendor: e.target.value || undefined, webglRenderer: undefined })
              }}
              options={WEBGL_VENDOR_OPTIONS}
            />
          </FormItem>
          <FormItem label="WebGL 渲染器">
            {isCustomRenderer ? (
              <div className="flex gap-2">
                <Input
                  value={config.webglRenderer ?? ''}
                  onChange={e => update({ webglRenderer: e.target.value || undefined })}
                  placeholder="自定义渲染器名称"
                  className="flex-1"
                />
                <button
                  type="button"
                  onClick={() => {
                    setCustomRendererMode(false)
                    update({ webglRenderer: undefined })
                  }}
                  className="px-2.5 text-xs rounded-lg border border-[var(--color-border-default)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-subtle)] shrink-0"
                >
                  切回列表
                </button>
              </div>
            ) : (
              <Select
                value={config.webglRenderer ?? ''}
                onChange={e => {
                  if (e.target.value === 'custom') {
                    setCustomRendererMode(true)
                    update({ webglRenderer: undefined })
                  } else {
                    setCustomRendererMode(false)
                    update({ webglRenderer: e.target.value || undefined })
                  }
                }}
                options={rendererOptions}
                disabled={!config.webglVendor}
              />
            )}
          </FormItem>
          <FormItem label="Canvas 噪声">
            <Select
              value={config.canvasNoise === undefined ? '' : String(config.canvasNoise)}
              onChange={e => { const v = e.target.value; update({ canvasNoise: v === '' ? undefined : v === 'true' }) }}
              options={BOOL_OPTIONS}
            />
          </FormItem>
          <FormItem label="Audio 噪声">
            <Select
              value={config.audioNoise === undefined ? '' : String(config.audioNoise)}
              onChange={e => { const v = e.target.value; update({ audioNoise: v === '' ? undefined : v === 'true' }) }}
              options={BOOL_OPTIONS}
            />
          </FormItem>
        </div>
      </div>

      {/* 网络与隐私 */}
      <div>
        <p className="text-xs font-medium text-[var(--color-text-muted)] mb-2 uppercase tracking-wide">网络与隐私</p>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormItem label="WebRTC 策略">
            <Select value={config.webrtcPolicy ?? ''} onChange={e => update({ webrtcPolicy: e.target.value || undefined })} options={WEBRTC_OPTIONS} />
          </FormItem>
          <FormItem label="Do Not Track">
            <Select
              value={config.doNotTrack === undefined ? '' : String(config.doNotTrack)}
              onChange={e => { const v = e.target.value; update({ doNotTrack: v === '' ? undefined : v === 'true' }) }}
              options={BOOL_OPTIONS}
            />
          </FormItem>
          <FormItem label="媒体设备 (摄像头,麦克风,扬声器)">
            <Input
              value={config.mediaDevices ?? ''}
              onChange={e => update({ mediaDevices: e.target.value || undefined })}
              placeholder="2,1,1"
            />
          </FormItem>
        </div>
      </div>

      {/* 字体 */}
      <div>
        <p className="text-xs font-medium text-[var(--color-text-muted)] mb-2 uppercase tracking-wide">字体</p>
        <FormItem label="字体列表">
          <Input
            value={config.fonts ?? ''}
            onChange={e => update({ fonts: e.target.value || undefined })}
            placeholder="Arial,Helvetica,Times New Roman（逗号分隔）"
          />
        </FormItem>
      </div>

      {/* 高级模式 */}
      <div className="border border-[var(--color-border)] rounded-lg overflow-hidden">
        <button
          type="button"
          className="w-full flex items-center justify-between px-4 py-2.5 text-sm text-[var(--color-text-muted)] hover:bg-[var(--color-bg-hover)] transition-colors"
          onClick={() => setAdvancedOpen(v => !v)}
        >
          <span>高级模式（原始参数）</span>
          {advancedOpen ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
        </button>
        {advancedOpen && (
          <div className="px-4 pb-4 pt-2 border-t border-[var(--color-border)]">
            <p className="text-xs text-[var(--color-text-muted)] mb-2">每行一个参数，修改后自动同步到上方控件</p>
            <Textarea
              value={advancedText}
              onChange={e => handleAdvancedChange(e.target.value)}
              rows={6}
              placeholder="--fingerprint-brand=Chrome"
            />
          </div>
        )}
      </div>
    </div>
  )
}
