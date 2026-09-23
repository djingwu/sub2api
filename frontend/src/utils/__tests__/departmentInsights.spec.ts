import { describe, expect, it } from 'vitest'

import type { DepartmentUsageStat } from '@/api/usage'
import {
  buildDepartmentInsights,
  type DepartmentInsightInput,
  type TranslateFn
} from '../departmentInsights'

const translate: TranslateFn = (key, named) => {
  if (!named) return key
  const params = Object.entries(named)
    .map(([name, value]) => `${name}=${String(value)}`)
    .join(',')
  return `${key}(${params})`
}

const labels = { 1: '部门 A', 2: '部门 B', 3: '部门 C' }

function department(overrides: Partial<DepartmentUsageStat> = {}): DepartmentUsageStat {
  return {
    group_id: 1,
    group_name: 'A',
    requests: 0,
    total_tokens: 0,
    input_tokens: 0,
    output_tokens: 0,
    cache_creation_tokens: 0,
    cache_read_tokens: 0,
    model_count: 0,
    active_user_count: 0,
    image_count: 0,
    video_count: 0,
    stream_requests: 0,
    avg_duration_ms: 0,
    avg_first_token_ms: 0,
    top_models: [],
    ...overrides
  }
}

function build(overrides: Partial<DepartmentInsightInput>) {
  return buildDepartmentInsights({
    summary: null,
    departments: [],
    previousDepartments: [],
    clients: [],
    unusedDepartments: [],
    anonymousLabels: labels,
    translate,
    ...overrides
  })
}

describe('buildDepartmentInsights', () => {
  it('returns nothing when there is no department data', () => {
    expect(build({})).toEqual([])
  })

  it('reports the fastest growing and fastest declining departments', () => {
    const insights = build({
      departments: [
        department({ group_id: 1, total_tokens: 22_000 }),
        department({ group_id: 2, total_tokens: 5_000 })
      ],
      previousDepartments: [
        department({ group_id: 1, total_tokens: 10_000 }),
        department({ group_id: 2, total_tokens: 10_000 })
      ]
    })

    const growth = insights.find((insight) => insight.id === 'growth')
    expect(growth?.text).toContain('departmentUsage.insightGrowth')
    expect(growth?.text).toContain('name=部门 A')
    expect(growth?.text).toContain('percent=+120.0%')
    expect(growth?.text).toContain('tokens=22.0K')

    const decline = insights.find((insight) => insight.id === 'decline')
    expect(decline?.text).toContain('departmentUsage.insightDecline')
    expect(decline?.text).toContain('name=部门 B')
    expect(decline?.text).toContain('percent=-50.0%')
  })

  it('flags the department with the highest per-capita intensity', () => {
    const insights = build({
      summary: {
        total_requests: 0,
        total_tokens: 100_000,
        active_departments: 2,
        active_users: 10,
        total_departments: 2
      },
      departments: [
        department({ group_id: 1, total_tokens: 40_000, active_user_count: 2 }),
        department({ group_id: 2, total_tokens: 18_000, active_user_count: 1 })
      ]
    })

    const intensity = insights.find((insight) => insight.id === 'intensity')
    expect(intensity?.text).toContain('name=部门 A')
    expect(intensity?.text).toContain('perCapita=20.0K')
    expect(intensity?.text).toContain('ratio=2.0')
  })

  it('suggests prompt caching for the lowest cache-read rate at scale', () => {
    const insights = build({
      departments: [
        department({
          group_id: 1,
          total_tokens: 2_000_000,
          input_tokens: 1_800_000,
          cache_read_tokens: 200_000
        }),
        department({
          group_id: 2,
          total_tokens: 1_000_000,
          input_tokens: 200_000,
          cache_read_tokens: 800_000
        })
      ]
    })

    const cache = insights.find((insight) => insight.id === 'cache')
    expect(cache?.tone).toBe('warning')
    expect(cache?.text).toContain('name=部门 A')
    expect(cache?.text).toContain('rate=10.0%')
    expect(cache?.text).toContain('average=33.3%')
    expect(cache?.text).toContain('input=2.0M')
  })

  it('ignores low-volume cache rates', () => {
    const insights = build({
      departments: [
        department({
          group_id: 1,
          total_tokens: 100_000,
          input_tokens: 90_000,
          cache_read_tokens: 10_000
        })
      ]
    })

    expect(insights.find((insight) => insight.id === 'cache')).toBeUndefined()
  })

  it('reports model concentration above the threshold', () => {
    const insights = build({
      departments: [
        department({
          group_id: 1,
          total_tokens: 10_000,
          top_models: [{ model: 'gpt-5', total_tokens: 8_000 }]
        })
      ]
    })

    const concentration = insights.find((insight) => insight.id === 'concentration')
    expect(concentration?.text).toContain('model=gpt-5')
    expect(concentration?.text).toContain('share=80%')
  })

  it('describes a dominant client software product', () => {
    const insights = build({
      departments: [department({ group_id: 1, total_tokens: 1 })],
      clients: [
        { client_software: 'claude-cli', requests: 1, total_tokens: 8_000, user_count: 2, department_count: 3 },
        { client_software: 'other', requests: 1, total_tokens: 2_000, user_count: 1, department_count: 1 }
      ]
    })

    const clients = insights.find((insight) => insight.id === 'clients')
    expect(clients?.text).toContain('departmentUsage.insightClientTop')
    expect(clients?.text).toContain('client=claude-cli')
    expect(clients?.text).toContain('share=80%')
    expect(clients?.text).toContain('departments=3')
  })

  it('reports adoption coverage and marks low coverage as a warning', () => {
    const base = {
      departments: [department({ group_id: 1, total_tokens: 1 })]
    }
    const healthy = build({
      ...base,
      summary: {
        total_requests: 0,
        total_tokens: 0,
        active_departments: 1,
        active_users: 1,
        total_departments: 4
      },
      unusedDepartments: [
        { group_id: 2, group_name: 'B' },
        { group_id: 3, group_name: 'C' }
      ]
    }).find((insight) => insight.id === 'coverage')
    expect(healthy?.tone).toBe('neutral')
    expect(healthy?.text).toContain('used=2')
    expect(healthy?.text).toContain('total=4')
    expect(healthy?.text).toContain('rate=50%')

    const low = build({
      ...base,
      summary: {
        total_requests: 0,
        total_tokens: 0,
        active_departments: 1,
        active_users: 1,
        total_departments: 4
      },
      unusedDepartments: [
        { group_id: 2, group_name: 'B' },
        { group_id: 3, group_name: 'C' },
        { group_id: 4, group_name: 'D' }
      ]
    }).find((insight) => insight.id === 'coverage')
    expect(low?.tone).toBe('warning')
    expect(low?.text).toContain('rate=25%')
  })

  it('reports full coverage when every department had usage', () => {
    const coverage = build({
      departments: [department({ group_id: 1, total_tokens: 1 })],
      summary: {
        total_requests: 0,
        total_tokens: 0,
        active_departments: 1,
        active_users: 1,
        total_departments: 1
      }
    }).find((insight) => insight.id === 'coverage')
    expect(coverage?.tone).toBe('positive')
    expect(coverage?.text).toContain('total=1')
  })
})
