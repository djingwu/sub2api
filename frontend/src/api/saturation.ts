/**
 * Admin user saturation report API endpoints.
 */

import { apiClient } from './client'

export interface SaturationBaselineCombo {
  model: string
  effort: string
}

export interface SaturationConfig {
  baseline_combos: SaturationBaselineCombo[]
  threshold_percent: number
}

export interface SaturationSummary {
  user_count: number
  quota_usd: number
  used_usd: number
  lifetime_used_usd: number
  average_saturation: number
  average_lifetime_saturation: number
  dormant_users: number
  heavy_users: number
  reset_users: number
  never_used_users: number
  baseline_cost_usd: number
  baseline_cost_share: number
  baseline_token_share: number
  compliant_users: number
  compliant_rate: number
  earliest_window_start?: string | null
  latest_window_end?: string | null
}

export interface SaturationBandStat {
  key: string
  min_pct: number
  max_pct?: number | null
  users: number
  used_usd: number
}

export interface SaturationComboStat {
  model: string
  effort: string
  requests: number
  tokens: number
  cost_usd: number
  users: number
  usd_per_million_tokens: number
  baseline: boolean
  cost_share: number
  token_share: number
}

export interface SaturationSnapshot {
  generated_at: string
  config: SaturationConfig
  summary: SaturationSummary
  current_bands: SaturationBandStat[]
  lifetime_bands: SaturationBandStat[]
  combos: SaturationComboStat[]
}

export interface SaturationUser {
  user_id: number
  email: string
  username: string
  status: string
  primary_dept_id?: number | null
  dept_name?: string
  group_names?: string
  quota_usd: number
  used_usd: number
  used_percent: number
  daily_used_usd: number
  weekly_used_usd: number
  lifetime_used_usd: number
  lifetime_percent: number
  cycles: number
  window_started_at?: string | null
  expires_at?: string | null
  lifetime_requests: number
  window_requests: number
  window_tokens: number
  window_cost_usd: number
  first_used_at?: string | null
  last_used_at?: string | null
  baseline_cost_usd: number
  baseline_cost_share: number
  baseline_token_share: number
  compliant: boolean
  top_model?: string
  top_effort?: string
  top_combo_tokens: number
}

export interface SaturationUserList {
  items: SaturationUser[]
  total: number
  page: number
  page_size: number
}

export interface SaturationTrendPoint {
  date: string
  requests: number
  tokens: number
  cost_usd: number
  users: number
}

export interface SaturationUserQuery {
  page?: number
  page_size?: number
  metric?: 'current' | 'lifetime'
  band?: string
  compliance?: 'compliant' | 'non_compliant' | ''
  search?: string
  sort?: string
  order?: 'asc' | 'desc'
}

const saturationBase = '/admin/saturation'

export async function getSaturationSnapshot(signal?: AbortSignal): Promise<SaturationSnapshot> {
  const { data } = await apiClient.get<SaturationSnapshot>(saturationBase, { signal })
  return data
}

export async function getSaturationUsers(
  params: SaturationUserQuery,
  signal?: AbortSignal
): Promise<SaturationUserList> {
  const { data } = await apiClient.get<SaturationUserList>(`${saturationBase}/users`, {
    params,
    signal
  })
  return data
}

export async function getSaturationTrend(days = 30): Promise<SaturationTrendPoint[]> {
  const { data } = await apiClient.get<SaturationTrendPoint[]>('/admin/saturation/trend', {
    params: { days }
  })
  return data || []
}

export async function getSaturationConfig(): Promise<SaturationConfig> {
  const { data } = await apiClient.get<SaturationConfig>('/admin/saturation/config')
  return data
}

export async function updateSaturationConfig(config: SaturationConfig): Promise<SaturationConfig> {
  const { data } = await apiClient.put<SaturationConfig>('/admin/saturation/config', config)
  return data
}
