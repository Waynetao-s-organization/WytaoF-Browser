import type { FingerprintScheme, FingerprintSchemeApplyResult, FingerprintSchemeInput } from '../types'
import { getBindings } from './runtime'

export const FINGERPRINT_SCHEME_NONE = '__none__'

export async function fetchFingerprintSchemes(): Promise<FingerprintScheme[]> {
  const bindings: any = await getBindings()
  if (bindings?.BrowserFingerprintSchemeList) {
    return (await bindings.BrowserFingerprintSchemeList()) || []
  }
  return []
}

export async function createFingerprintScheme(input: FingerprintSchemeInput): Promise<FingerprintScheme> {
  const bindings: any = await getBindings()
  if (!bindings?.BrowserFingerprintSchemeCreate) throw new Error('当前运行环境不支持指纹方案')
  return await bindings.BrowserFingerprintSchemeCreate(input)
}

export async function updateFingerprintScheme(schemeId: string, input: FingerprintSchemeInput): Promise<FingerprintScheme> {
  const bindings: any = await getBindings()
  if (!bindings?.BrowserFingerprintSchemeUpdate) throw new Error('当前运行环境不支持指纹方案')
  return await bindings.BrowserFingerprintSchemeUpdate(schemeId, input)
}

export async function deleteFingerprintScheme(schemeId: string): Promise<void> {
  const bindings: any = await getBindings()
  if (!bindings?.BrowserFingerprintSchemeDelete) throw new Error('当前运行环境不支持指纹方案')
  await bindings.BrowserFingerprintSchemeDelete(schemeId)
}

export async function setDefaultFingerprintScheme(schemeId: string): Promise<void> {
  const bindings: any = await getBindings()
  if (!bindings?.BrowserFingerprintSchemeSetDefault) throw new Error('当前运行环境不支持指纹方案')
  await bindings.BrowserFingerprintSchemeSetDefault(schemeId)
}

export async function generateFingerprintSchemeArgs(schemeId: string): Promise<string[]> {
  const bindings: any = await getBindings()
  if (!bindings?.BrowserFingerprintSchemeGenerateArgs) throw new Error('当前运行环境不支持指纹方案')
  return (await bindings.BrowserFingerprintSchemeGenerateArgs(schemeId)) || []
}

export async function applyFingerprintScheme(schemeId: string, profileIds: string[], rerollDevice: boolean): Promise<FingerprintSchemeApplyResult> {
  const bindings: any = await getBindings()
  if (!bindings?.BrowserFingerprintSchemeApply) throw new Error('当前运行环境不支持指纹方案')
  return await bindings.BrowserFingerprintSchemeApply(schemeId, profileIds, rerollDevice)
}
