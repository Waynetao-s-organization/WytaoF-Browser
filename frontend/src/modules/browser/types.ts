export interface BrowserProfile {
  profileId: string
  profileName: string
  userDataDir: string
  coreId: string
  fingerprintArgs: string[]
  fingerprintSchemeId?: string
  proxyId: string
  proxyConfig: string
  proxyBindSourceId?: string
  proxyBindSourceUrl?: string
  proxyBindName?: string
  proxyBindUpdatedAt?: string
  launchArgs: string[]
  tags: string[]
  keywords: string[]
  groupId?: string
  running: boolean
  debugPort: number
  debugReady: boolean
  pid: number
  runtimeWarning: string
  lastError: string
  createdAt: string
  updatedAt: string
  lastStartAt?: string
  lastStopAt?: string
  launchCode?: string
}

export interface BrowserProfileInput {
  profileName: string
  userDataDir: string
  coreId: string
  fingerprintArgs: string[]
  fingerprintSchemeId?: string
  proxyId: string
  proxyConfig: string
  launchArgs: string[]
  tags: string[]
  keywords: string[]
  groupId?: string
}

export interface BrowserProfileTemplate {
  templateId: string
  templateName: string
  profileName: string
  userDataDir: string
  coreId: string
  fingerprintArgs: string[]
  proxyId: string
  proxyConfig: string
  launchArgs: string[]
  tags: string[]
  keywords: string[]
  groupId?: string
  createdAt: string
  updatedAt: string
}

export interface BrowserProfileTemplateInput {
  templateName: string
  profileName: string
  userDataDir: string
  coreId: string
  fingerprintArgs: string[]
  proxyId: string
  proxyConfig: string
  launchArgs: string[]
  tags: string[]
  keywords: string[]
  groupId?: string
}

export interface BrowserProfileBatchCreateInput {
  count: number
  base: BrowserProfileInput
  randomizeFingerprintPerProfile?: boolean
}

export interface BrowserProfileBatchCreateItemResult {
  index: number
  profileName: string
  profile?: BrowserProfile
  error?: string
}

export interface BrowserProfileBatchCreateResult {
  total: number
  succeeded: number
  failed: number
  items: BrowserProfileBatchCreateItemResult[]
}

export interface BrowserTab {
  tabId: string
  title: string
  url: string
  active: boolean
}

export interface BrowserSettings {
  userDataRoot: string
  defaultFingerprintArgs: string[]
  defaultLaunchArgs: string[]
  defaultStartUrls: string[]
  restoreLastSession: boolean
  startPageDisabled?: boolean
  windowTitleDisabled?: boolean
  startReadyTimeoutMs: number
  startStableWindowMs: number
}

export interface ProxyCheckTarget {
  id: string
  name: string
  type: string
  url: string
  parser?: string
  timeoutMs?: number
  expectedStatus?: number[]
}

export interface ProxyCheckSettings {
  bridgeStartTimeoutMs: number
  speedTargetId: string
  ipHealthTargetId: string
  targets: ProxyCheckTarget[]
}

export interface BrowserCore {
  coreId: string
  coreName: string
  corePath: string
  isDefault: boolean
}

export interface BrowserCoreInput {
  coreId: string
  coreName: string
  corePath: string
  isDefault: boolean
}

export interface BrowserCoreValidateResult {
  valid: boolean
  message: string
}

export interface BrowserProxy {
  proxyId: string
  proxyName: string
  proxyConfig: string
  dnsServers?: string
  groupName?: string
  sourceId?: string
  sourceUrl?: string
  sourceNamePrefix?: string
  sourceAutoRefresh?: boolean
  sourceRefreshIntervalM?: number
  sourceLastRefreshAt?: string
  lastLatencyMs?: number
  lastTestOk?: boolean
  lastTestedAt?: string
  lastIPHealthJson?: string
}

export interface ProxyIPHealthResult {
  proxyId: string
  ok: boolean
  source: string
  error: string
  ip: string
  fraudScore: number
  isResidential: boolean
  isBroadcast: boolean
  country: string
  region: string
  city: string
  asOrganization: string
  rawData: Record<string, any>
  updatedAt: string
}

export interface BrowserCoreExtended {
  coreId: string
  chromeVersion: string
  instanceCount: number
}

export interface CookieInfo {
  name: string
  value: string
  domain: string
  path: string
  expires: number
  httpOnly: boolean
  secure: boolean
  sameSite: string
}

export interface SnapshotInfo {
  snapshotId: string
  profileId: string
  name: string
  sizeMB: number
  createdAt: string
}

export interface BrowserBookmark {
  name: string
  url: string
  openOnStart?: boolean
  openTags?: string[]
  openGroupIds?: string[]
}

export interface BookmarkSyncResult {
  total: number
  synced: number
  skipped: number
  failed: number
  skippedList: string[]
  failedList: string[]
}


// 分组相关类型
export interface BrowserGroup {
  groupId: string
  groupName: string
  parentId: string
  sortOrder: number
  createdAt: string
  updatedAt: string
}

export interface BrowserGroupInput {
  groupName: string
  parentId: string
  sortOrder: number
}

export interface BrowserGroupWithCount extends BrowserGroup {
  instanceCount: number
}

export interface WeightedValue {
  value: string
  weight: number
}

export interface FingerprintSchemeFixed {
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
}

export interface FingerprintSchemeDevice {
  cores: WeightedValue[]
  memory: WeightedValue[]
  platformVersions: WeightedValue[]
  windowSizes: WeightedValue[]
}

export interface FingerprintScheme {
  schemeId: string
  schemeName: string
  description: string
  builtin: boolean
  isDefault: boolean
  sortOrder: number
  fixed: FingerprintSchemeFixed
  device: FingerprintSchemeDevice
  createdAt: string
  updatedAt: string
}

export interface FingerprintSchemeInput {
  schemeName: string
  description: string
  fixed: FingerprintSchemeFixed
  device: FingerprintSchemeDevice
}

export interface FingerprintSchemeApplyResult {
  total: number
  updated: number
  skipped: number
  errors: string[]
}
