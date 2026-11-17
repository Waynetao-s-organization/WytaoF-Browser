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
  { value: '', label: '默认（自动保底 Chrome）' },
  { value: 'Chrome', label: 'Google Chrome' },
  { value: 'Edge', label: 'Microsoft Edge' },
  { value: 'Opera', label: 'Opera' },
  { value: 'Vivaldi', label: 'Vivaldi' },
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

const SCALE_FACTOR_OPTIONS = [
  { value: '', label: '自动（设分辨率时固定 1x）' },
  { value: '1', label: '1x（标准桌面显示器）' },
  { value: '1.25', label: '1.25x（125% 缩放）' },
  { value: '1.5', label: '1.5x（150% 缩放）' },
  { value: '2', label: '2x（高分屏 / Retina）' },
]

const HARDWARE_CONCURRENCY_OPTIONS = [
  { value: '', label: '不设置（跟随本机）' },
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

const WEBRTC_OPTIONS = [
  { value: '', label: '不设置' },
  { value: 'disable_non_proxied_udp', label: '禁用非代理 UDP（推荐防泄露）' },
  { value: 'default_public_interface_only', label: '仅公网接口' },
  { value: 'default_public_and_private_interfaces', label: '公网+私网接口' },
]

const CANVAS_SPOOFING_OPTIONS = [
  { value: 'noise', label: '开启种子噪声（默认，同站点多号防关联）' },
  { value: 'real', label: '关闭像素篡改（恢复纯色与字宽，单号高风控 / Chrom-150 推荐）' },
]

const SPEECH_API_OPTIONS = [
  { value: 'disable', label: '屏蔽系统语音包（默认，防暴露中文系统语音）' },
  { value: 'enable', label: '保留系统朗读语音（网页需语音合成功能时选）' },
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
  const [confirmSeedOpen, setConfirmSeedOpen] = useState(false)
  const lastEmittedRef = useRef<string>(value.join('\n'))

  useEffect(() => {
    const joined = value.join('\n')
    if (joined === lastEmittedRef.current) {
      return
    }
    lastEmittedRef.current = joined
    setConfig(parseFingerprintValue(value))
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
    const next: FingerprintConfig = {
      ...preset.config,
      seed: randomFingerprintSeed(),
      disableCanvasSpoofing: config.disableCanvasSpoofing,
      speechApiPolicy: config.speechApiPolicy,
      unknownArgs: config.unknownArgs,
    }
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

  const advancedText = serialize(config).join('\n')

  return (
    <div className="space-y-4">
      {/* 指纹种子 */}
      <div className="p-3 rounded-lg bg-[var(--color-bg-hover)] border border-[var(--color-border)] space-y-2">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium text-[var(--color-text-muted)] uppercase tracking-wide">指纹种子（Fingerprint Seed）</span>
          <span className="text-xs text-[var(--color-text-muted)]">决定底层内核派生的硬件与噪声特征，不同种子 = 不同指纹</span>
        </div>
        <div className="flex items-center gap-2">
          <Input
            value={config.seed ?? ''}
            onChange={e => update({ seed: e.target.value || undefined })}
            placeholder="留空则由系统按 ProfileId 自动派生固定种子"
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
        content="重新生成后，当前指纹将完全改变，浏览器的 WebGL、Audio、Canvas 等派生特征都会随之变化。确定继续？"
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
        <span className="text-xs text-[var(--color-text-muted)] shrink-0">选择后覆盖基础配置</span>
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
          <FormItem label="屏幕缩放比">
            <Select value={config.scaleFactor ?? ''} onChange={e => update({ scaleFactor: e.target.value || undefined })} options={SCALE_FACTOR_OPTIONS} />
          </FormItem>
          <FormItem label="CPU 核心数">
            <Select value={config.hardwareConcurrency ?? ''} onChange={e => update({ hardwareConcurrency: e.target.value || undefined })} options={HARDWARE_CONCURRENCY_OPTIONS} />
          </FormItem>
          <FormItem label="设备内存（Chrom-150+ 内核生效）">
            <Select value={config.deviceMemory ?? ''} onChange={e => update({ deviceMemory: e.target.value || undefined })} options={DEVICE_MEMORY_OPTIONS} />
          </FormItem>
        </div>
      </div>

      {/* 渲染与隐私 */}
      <div>
        <p className="text-xs font-medium text-[var(--color-text-muted)] mb-2 uppercase tracking-wide">渲染与隐私</p>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <FormItem label="WebRTC 策略">
            <Select value={config.webrtcPolicy ?? ''} onChange={e => update({ webrtcPolicy: e.target.value || undefined })} options={WEBRTC_OPTIONS} />
          </FormItem>
          <FormItem label="系统朗读语音">
            <Select
              value={config.speechApiPolicy === 'enable' ? 'enable' : 'disable'}
              onChange={e => update({ speechApiPolicy: e.target.value === 'enable' ? 'enable' : undefined })}
              options={SPEECH_API_OPTIONS}
            />
          </FormItem>
          <div className="md:col-span-2">
            <FormItem label="Canvas 指纹模式">
              <Select
                value={config.disableCanvasSpoofing ? 'real' : 'noise'}
                onChange={e => update({ disableCanvasSpoofing: e.target.value === 'real' ? true : undefined })}
                options={CANVAS_SPOOFING_OPTIONS}
              />
            </FormItem>
          </div>
        </div>
        <p className="mt-2 text-xs text-[var(--color-text-muted)] leading-relaxed">
          说明：WebGL 显卡由内核按种子自动派生，WebGPU、系统已安装字体与触控点数跟随本机硬件；高风控平台（如 PayPal、TikTok、Amazon）建议关闭 Canvas 像素篡改，并在 Windows 本机避免选用跨系统的 macOS 预设。
        </p>
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
