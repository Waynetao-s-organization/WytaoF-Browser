// 指纹参数序列化/反序列化工具

/**
 * 获取系统当前时区
 * @returns IANA 时区标识符，如 "Asia/Shanghai"
 */
export function getSystemTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone
  } catch {
    return 'UTC'
  }
}

export interface FingerprintConfig {
  // 指纹种子（核心）
  seed?: string                  // --fingerprint=<seed>  控制所有随机噪声的根种子

  // 基础身份
  brand?: string                 // --fingerprint-brand=（Chrome / Edge / Opera / Vivaldi）
  platform?: string              // --fingerprint-platform=（windows / macos / linux）
  lang?: string                  // --lang=
  timezone?: string              // --timezone=（具体时区、'system' 或 'custom'）
  customTimezone?: string        // 当 timezone === 'custom' 时使用

  // 屏幕与硬件（底层内核真实生效项）
  resolution?: string            // --window-size=（预设值或 'custom'）
  customResolution?: string      // 当 resolution === 'custom' 时使用
  scaleFactor?: string           // --fingerprint-device-scale-factor=（1 / 1.25 / 1.5 / 2）
  hardwareConcurrency?: string   // --fingerprint-hardware-concurrency=
  deviceMemory?: string          // --fingerprint-memory=（Chrom-150+ 内核生效）

  // 渲染与隐私（真实内核控制开关）
  webrtcPolicy?: string          // --webrtc-ip-handling-policy=
  disableCanvasSpoofing?: boolean // --disable-spoofing=canvas（关闭纯色像素篡改并恢复正常字宽）
  speechApiPolicy?: 'disable' | 'enable' // 默认屏蔽，'enable' 时输出 --enable-speech-api

  unknownArgs?: string[]         // 无法识别的原始参数，原样保留
}

export const PRESET_RESOLUTIONS = ['1920,1080', '1440,900', '1366,768', '2560,1440', '1280,800', '1600,900']

export function normalizeBrand(raw?: string): string | undefined {
  if (!raw) return undefined
  const lower = raw.trim().toLowerCase()
  if (!lower) return undefined
  if (lower === 'edge') return 'Edge'
  if (lower === 'opera') return 'Opera'
  if (lower === 'vivaldi') return 'Vivaldi'
  // 历史不支持的 Firefox / Safari / Google Chrome 统一修正为内核支持的 Chrome，避免拼出畸形 UA
  return 'Chrome'
}

// CLI 参数前缀 → FingerprintConfig 字段映射（仅保留底层内核真实识别的参数）
export const KEY_MAP: Record<string, keyof FingerprintConfig> = {
  '--fingerprint': 'seed',
  '--fingerprint-brand': 'brand',
  '--fingerprint-platform': 'platform',
  '--lang': 'lang',
  '--timezone': 'timezone',
  '--window-size': 'resolution',
  '--fingerprint-device-scale-factor': 'scaleFactor',
  '--fingerprint-hardware-concurrency': 'hardwareConcurrency',
  '--fingerprint-memory': 'deviceMemory',
  '--fingerprint-device-memory': 'deviceMemory',
  '--webrtc-ip-handling-policy': 'webrtcPolicy',
}

// 由后端启动时自动派生的配套参数，反序列化时不混入原始未知参数
const TRANSIENT_DERIVED_KEYS = new Set([
  '--accept-lang',
  '--accept-language',
  '--fingerprint-screen-width',
  '--fingerprint-screen-height',
])

// 经二进制核实底层 Chrom-144 / Chrom-150 内核并不识别的历史废弃假开关，静默忽略以免误导
const DEPRECATED_UNSUPPORTED_KEYS = new Set([
  '--fingerprint-color-depth',
  '--fingerprint-canvas-noise',
  '--fingerprint-audio-noise',
  '--fingerprint-webgl-vendor',
  '--fingerprint-gpu-vendor',
  '--fingerprint-webgl-renderer',
  '--fingerprint-gpu-renderer',
  '--fingerprint-fonts',
  '--fingerprint-do-not-track',
  '--fingerprint-media-devices',
  '--fingerprint-touch-points',
])

// FingerprintConfig → string[]
export function serialize(config: FingerprintConfig): string[] {
  const args: string[] = []
  if (config.seed) args.push(`--fingerprint=${config.seed}`)
  const normalizedBrand = normalizeBrand(config.brand)
  if (normalizedBrand) args.push(`--fingerprint-brand=${normalizedBrand}`)
  if (config.platform) {
    const plat = config.platform === 'mac' ? 'macos' : config.platform
    args.push(`--fingerprint-platform=${plat}`)
  }
  if (config.lang) args.push(`--lang=${config.lang}`)
  const rawTz = config.timezone === 'custom' ? config.customTimezone?.trim() : config.timezone
  if (rawTz) {
    const tz = rawTz === 'system' ? getSystemTimezone() : rawTz
    if (tz) args.push(`--timezone=${tz}`)
  }

  const res = config.resolution === 'custom' ? config.customResolution?.trim() : config.resolution
  if (res) args.push(`--window-size=${res}`)
  if (config.scaleFactor) args.push(`--fingerprint-device-scale-factor=${config.scaleFactor}`)

  if (config.hardwareConcurrency) args.push(`--fingerprint-hardware-concurrency=${config.hardwareConcurrency}`)
  if (config.deviceMemory) args.push(`--fingerprint-memory=${config.deviceMemory}`)

  if (config.webrtcPolicy) args.push(`--webrtc-ip-handling-policy=${config.webrtcPolicy}`)

  const remainingUnknown: string[] = []
  const extraDisabledSpoofings: string[] = []
  for (const raw of config.unknownArgs ?? []) {
    const trimmed = raw.trim()
    if (!trimmed) continue
    if (trimmed.toLowerCase().startsWith('--disable-spoofing=')) {
      const modules = trimmed
        .slice('--disable-spoofing='.length)
        .split(',')
        .map(s => s.trim())
        .filter(s => s && s.toLowerCase() !== 'canvas')
      extraDisabledSpoofings.push(...modules)
      continue
    }
    remainingUnknown.push(trimmed)
  }

  const spoofingList = [
    ...(config.disableCanvasSpoofing ? ['canvas'] : []),
    ...extraDisabledSpoofings,
  ]
  if (spoofingList.length > 0) {
    args.push(`--disable-spoofing=${Array.from(new Set(spoofingList)).join(',')}`)
  }

  if (config.speechApiPolicy === 'enable') {
    args.push('--enable-speech-api')
  }

  return [...args, ...remainingUnknown]
}

// string[] → FingerprintConfig
export function deserialize(args: string[]): FingerprintConfig {
  const config: FingerprintConfig = { unknownArgs: [] }

  for (const rawArg of args) {
    const arg = rawArg.trim()
    if (!arg) continue
    const lowerArg = arg.toLowerCase()
    if (lowerArg === '--enable-speech-api') {
      config.speechApiPolicy = 'enable'
      continue
    }
    if (lowerArg === '--disable-speech-api') {
      if (!config.speechApiPolicy) {
        config.speechApiPolicy = 'disable'
      }
      continue
    }

    const eqIdx = arg.indexOf('=')
    if (eqIdx === -1) {
      config.unknownArgs!.push(arg)
      continue
    }
    const key = arg.slice(0, eqIdx)
    const lowerKey = key.toLowerCase()
    const val = arg.slice(eqIdx + 1)

    if (TRANSIENT_DERIVED_KEYS.has(lowerKey) || DEPRECATED_UNSUPPORTED_KEYS.has(lowerKey)) {
      continue
    }

    if (lowerKey === '--disable-spoofing') {
      const modules = val.split(',').map(s => s.trim()).filter(Boolean)
      const hasCanvas = modules.some(m => m.toLowerCase() === 'canvas')
      if (hasCanvas) {
        config.disableCanvasSpoofing = true
      }
      const others = modules.filter(m => m.toLowerCase() !== 'canvas')
      if (others.length > 0) {
        config.unknownArgs!.push(`--disable-spoofing=${others.join(',')}`)
      }
      continue
    }

    const field = KEY_MAP[lowerKey] ?? KEY_MAP[key]
    if (!field) {
      config.unknownArgs!.push(arg)
      continue
    }

    if (field === 'brand') {
      config.brand = normalizeBrand(val)
    } else if (field === 'platform') {
      config.platform = val === 'mac' ? 'macos' : val
    } else if (field === 'resolution') {
      if (PRESET_RESOLUTIONS.includes(val)) {
        config.resolution = val
      } else {
        config.resolution = 'custom'
        config.customResolution = val
      }
    } else {
      (config as Record<string, unknown>)[field] = val
    }
  }

  return config
}

// 生成随机指纹种子（32位正整数）
export function randomFingerprintSeed(): string {
  return String(Math.floor(Math.random() * 2147483647) + 1)
}

// ─── 预设指纹配置（全部使用底层内核真实生效的参数） ─────────────────────────────

export interface FingerprintPreset {
  id: string
  name: string
  description: string
  config: Partial<FingerprintConfig>
}

export const FINGERPRINT_PRESETS: FingerprintPreset[] = [
  {
    id: 'win-chrome-office',
    name: 'Windows / Chrome / 国内办公',
    description: '模拟国内办公 Windows 用户，中文环境，1920x1080 标准缩放',
    config: {
      brand: 'Chrome',
      platform: 'windows',
      lang: 'zh-CN',
      timezone: 'Asia/Shanghai',
      resolution: '1920,1080',
      scaleFactor: '1',
      hardwareConcurrency: '8',
      deviceMemory: '8',
      webrtcPolicy: 'disable_non_proxied_udp',
    },
  },
  {
    id: 'win-chrome-gaming',
    name: 'Windows / Chrome / 美东高配桌面',
    description: '模拟美东高配 Windows PC，英文环境，2560x1440',
    config: {
      brand: 'Chrome',
      platform: 'windows',
      lang: 'en-US',
      timezone: 'America/New_York',
      resolution: '2560,1440',
      scaleFactor: '1',
      hardwareConcurrency: '16',
      deviceMemory: '16',
      webrtcPolicy: 'disable_non_proxied_udp',
    },
  },
  {
    id: 'win-chrome-us-user',
    name: 'Windows / Chrome / 美西日常用户',
    description: '模拟美西洛杉矶普通 Windows 用户，英文环境，1920x1080',
    config: {
      brand: 'Chrome',
      platform: 'windows',
      lang: 'en-US',
      timezone: 'America/Los_Angeles',
      resolution: '1920,1080',
      scaleFactor: '1',
      hardwareConcurrency: '8',
      deviceMemory: '8',
      webrtcPolicy: 'disable_non_proxied_udp',
    },
  },
  {
    id: 'win-edge-enterprise',
    name: 'Windows / Edge / 商务笔记本',
    description: '模拟 Windows Edge 用户，1440x900',
    config: {
      brand: 'Edge',
      platform: 'windows',
      lang: 'en-US',
      timezone: 'America/Chicago',
      resolution: '1440,900',
      scaleFactor: '1',
      hardwareConcurrency: '8',
      deviceMemory: '8',
      webrtcPolicy: 'disable_non_proxied_udp',
    },
  },
  {
    id: 'win-chrome-uk-office',
    name: 'Windows / Chrome / 英国办公',
    description: '模拟英国伦敦 Windows 用户，英文环境 (en-GB)',
    config: {
      brand: 'Chrome',
      platform: 'windows',
      lang: 'en-GB',
      timezone: 'Europe/London',
      resolution: '1920,1080',
      scaleFactor: '1',
      hardwareConcurrency: '8',
      deviceMemory: '8',
      webrtcPolicy: 'disable_non_proxied_udp',
    },
  },
  {
    id: 'mac-chrome-designer',
    name: 'macOS / Chrome / 国内设计师',
    description: '模拟 Mac 用户（2x Retina）；强风控平台在 Windows 本机上慎选跨系统预设',
    config: {
      brand: 'Chrome',
      platform: 'macos',
      lang: 'zh-CN',
      timezone: 'Asia/Shanghai',
      resolution: '1440,900',
      scaleFactor: '2',
      hardwareConcurrency: '10',
      deviceMemory: '16',
      webrtcPolicy: 'disable_non_proxied_udp',
    },
  },
  {
    id: 'mac-chrome-jp',
    name: 'macOS / Chrome / 日本用户',
    description: '模拟日本东京 Mac 用户，日语环境（2x Retina）',
    config: {
      brand: 'Chrome',
      platform: 'macos',
      lang: 'ja-JP',
      timezone: 'Asia/Tokyo',
      resolution: '1440,900',
      scaleFactor: '2',
      hardwareConcurrency: '8',
      deviceMemory: '8',
      webrtcPolicy: 'disable_non_proxied_udp',
    },
  },
  {
    id: 'mac-chrome-us-edu',
    name: 'macOS / Chrome / 美东教育网',
    description: '模拟美国大学 Mac 用户，英文环境（2x Retina）',
    config: {
      brand: 'Chrome',
      platform: 'macos',
      lang: 'en-US',
      timezone: 'America/New_York',
      resolution: '1440,900',
      scaleFactor: '2',
      hardwareConcurrency: '8',
      deviceMemory: '8',
      webrtcPolicy: 'disable_non_proxied_udp',
    },
  },
]

export function randomFingerprintArgs(currentArgs: string[] = []): string[] {
  const current = deserialize(currentArgs)
  // 在 Windows 电脑上随机生成默认优先选用 Windows 预设，避免 WebGPU 与跨系统预设冲突
  const winPresets = FINGERPRINT_PRESETS.filter(p => p.config.platform === 'windows')
  const pool = winPresets.length > 0 ? winPresets : FINGERPRINT_PRESETS
  const preset = pool[Math.floor(Math.random() * pool.length)]
  return serialize({
    ...preset.config,
    seed: randomFingerprintSeed(),
    disableCanvasSpoofing: current.disableCanvasSpoofing,
    speechApiPolicy: current.speechApiPolicy,
    unknownArgs: current.unknownArgs,
  })
}
