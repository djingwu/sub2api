import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import SaturationTrendChart from '../SaturationTrendChart.vue'

const { getSaturationTrend } = vi.hoisted(() => ({
  getSaturationTrend: vi.fn()
}))

vi.mock('@/api/saturation', () => ({
  getSaturationTrend
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (params && Object.keys(params).length > 0) {
          return `${key} ${JSON.stringify(params)}`
        }
        return key
      }
    })
  }
})

const trend = [
  { date: '2026-09-22', requests: 100, tokens: 1_000_000, cost_usd: 50, users: 3 },
  { date: '2026-09-23', requests: 200, tokens: 2_000_000, cost_usd: 150, users: 5 }
]

describe('SaturationTrendChart', () => {
  it('renders daily bars with totals and switches metrics', async () => {
    getSaturationTrend.mockResolvedValue(trend)
    const wrapper = mount(SaturationTrendChart, {
      global: { stubs: { Icon: true } }
    })
    await flushPromises()

    expect(getSaturationTrend).toHaveBeenCalledWith(30)
    const text = wrapper.text()
    expect(text).toContain('2026-09-22')
    expect(text).toContain('2026-09-23')
    expect(text).toContain('$200.00')
    expect(text).toContain('saturation.trend.dailyAverage')

    const metricButtons = wrapper.findAll('button').filter((button) => button.text() === 'saturation.trend.cost')
    expect(metricButtons).toHaveLength(1)
    await metricButtons[0].trigger('click')
    expect(wrapper.text()).toContain('$150.00')
  })
})
