import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import SaturationReport from '../SaturationReport.vue'
import type { SaturationSnapshot, SaturationUserList } from '@/api/saturation'

const { getSaturationSnapshot, getSaturationUsers } = vi.hoisted(() => ({
  getSaturationSnapshot: vi.fn(),
  getSaturationUsers: vi.fn()
}))

vi.mock('@/api/saturation', () => ({
  getSaturationSnapshot,
  getSaturationUsers
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const snapshot: SaturationSnapshot = {
  generated_at: '2026-09-23T00:00:00Z',
  config: {
    baseline_combos: [
      { model: 'gpt-5.6-luna', effort: 'max' },
      { model: 'gpt-6-astra', effort: 'medium' }
    ],
    threshold_percent: 50
  },
  summary: {
    user_count: 3,
    quota_usd: 1800,
    used_usd: 780,
    lifetime_used_usd: 1500,
    average_saturation: 43.3,
    average_lifetime_saturation: 55,
    dormant_users: 1,
    heavy_users: 1,
    reset_users: 1,
    never_used_users: 0,
    baseline_cost_usd: 250,
    baseline_cost_share: 69.4,
    baseline_token_share: 80.4,
    compliant_users: 1,
    compliant_rate: 50,
    earliest_window_start: null,
    latest_window_end: null
  },
  current_bands: [
    { key: 'dormant', min_pct: 0, max_pct: 20, users: 1, used_usd: 0 },
    { key: 'light', min_pct: 20, max_pct: 50, users: 0, used_usd: 0 },
    { key: 'active', min_pct: 50, max_pct: 80, users: 1, used_usd: 300 },
    { key: 'saturated', min_pct: 80, max_pct: null, users: 1, used_usd: 480 }
  ],
  lifetime_bands: [
    { key: 'dormant', min_pct: 0, max_pct: 20, users: 1, used_usd: 120 },
    { key: 'light', min_pct: 20, max_pct: 50, users: 0, used_usd: 0 },
    { key: 'active', min_pct: 50, max_pct: 80, users: 1, used_usd: 900 },
    { key: 'saturated', min_pct: 80, max_pct: null, users: 1, used_usd: 480 }
  ],
  combos: [
    {
      model: 'gpt-5.6-luna',
      effort: 'max',
      requests: 10,
      tokens: 4_000_000,
      cost_usd: 200,
      users: 1,
      usd_per_million_tokens: 50,
      baseline: true,
      cost_share: 55.6,
      token_share: 71.4
    }
  ]
}

const users: SaturationUserList = {
  items: [
    {
      user_id: 1,
      email: 'u1@example.com',
      username: 'u1',
      status: 'active',
      dept_name: '研发部',
      group_names: '研发部组',
      quota_usd: 600,
      used_usd: 300,
      used_percent: 50,
      daily_used_usd: 10,
      weekly_used_usd: 80,
      lifetime_used_usd: 900,
      lifetime_percent: 75,
      cycles: 2,
      window_started_at: '2026-09-13T00:00:00Z',
      expires_at: '2026-10-13T00:00:00Z',
      lifetime_requests: 17,
      window_requests: 17,
      window_tokens: 4_600_000,
      window_cost_usd: 300,
      first_used_at: '2026-08-09T00:00:00Z',
      last_used_at: '2026-09-23T00:00:00Z',
      baseline_cost_usd: 250,
      baseline_cost_share: 83.3,
      baseline_token_share: 90,
      compliant: true,
      top_model: 'gpt-5.6-luna',
      top_effort: 'max',
      top_combo_tokens: 4_000_000
    }
  ],
  total: 1,
  page: 1,
  page_size: 20
}

const stubs = {
  Icon: true,
  Pagination: true,
  SaturationConfigDialog: true
}

describe('SaturationReport', () => {
  beforeEach(() => {
    getSaturationSnapshot.mockReset()
    getSaturationUsers.mockReset()
    getSaturationSnapshot.mockResolvedValue(snapshot)
    getSaturationUsers.mockResolvedValue(users)
  })

  it('renders summary KPIs and the user rows', async () => {
    const wrapper = mount(SaturationReport, {
      global: { stubs }
    })
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('43.3%')
    expect(text).toContain('$780.00 / $1800.00')
    expect(text).toContain('u1@example.com')
    expect(text).toContain('研发部')
    expect(text).toContain('83.3%')
    expect(text).toContain('saturation.combos.unitCost')
    expect(getSaturationSnapshot).toHaveBeenCalled()
    expect(getSaturationUsers).toHaveBeenCalledWith(expect.objectContaining({ page: 1 }))
  })
})
