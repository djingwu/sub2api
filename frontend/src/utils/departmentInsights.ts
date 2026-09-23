import { formatCompactNumber } from '@/utils/format'
import type {
  ClientSoftwareStat,
  DepartmentUsageStat,
  DepartmentUsageSummary,
  UnusedDepartment
} from '@/api/usage'

export type DepartmentInsightTone = 'positive' | 'warning' | 'neutral'

export type DepartmentInsightIcon =
  | 'trophy'
  | 'chart'
  | 'lightbulb'
  | 'exclamationTriangle'
  | 'grid'
  | 'checkCircle'
  | 'users'

export interface DepartmentInsight {
  id: string
  tone: DepartmentInsightTone
  icon: DepartmentInsightIcon
  text: string
}

export type TranslateFn = (key: string, named?: Record<string, unknown>) => string

export interface DepartmentInsightInput {
  summary: DepartmentUsageSummary | null
  departments: DepartmentUsageStat[]
  previousDepartments: DepartmentUsageStat[]
  clients: ClientSoftwareStat[]
  unusedDepartments: UnusedDepartment[]
  anonymousLabels: Record<number, string>
  translate: TranslateFn
}

const GROWTH_THRESHOLD = 0.1
const INTENSITY_THRESHOLD = 1.5
const CACHE_MIN_EFFECTIVE_INPUT = 1_000_000
const CACHE_GAP = 0.05
const CONCENTRATION_THRESHOLD = 0.6
const CLIENT_DOMINANCE_THRESHOLD = 0.5
const CLIENT_FRAGMENTED_MIN = 4

function percent(value: number, digits = 1): string {
  return `${(value * 100).toFixed(digits)}%`
}

function effectiveInputTokens(department: DepartmentUsageStat): number {
  return (
    (department.input_tokens || 0) +
    (department.cache_creation_tokens || 0) +
    (department.cache_read_tokens || 0)
  )
}

function cacheReadRate(department: DepartmentUsageStat): number | null {
  const effective = effectiveInputTokens(department)
  if (effective <= 0) return null
  return (department.cache_read_tokens || 0) / effective
}

function tokenDelta(
  current: DepartmentUsageStat,
  previous: DepartmentUsageStat | undefined
): number | null {
  if (!previous || previous.total_tokens <= 0 || current.total_tokens <= 0) return null
  return (current.total_tokens - previous.total_tokens) / previous.total_tokens
}

function departmentLabel(input: DepartmentInsightInput, department: DepartmentUsageStat): string {
  return input.anonymousLabels[department.group_id] || input.translate('departmentUsage.unassigned')
}

// Data-only conclusions for the team report. Everything is derived from the
// payload the page already has, so the panel never triggers extra requests.
export function buildDepartmentInsights(input: DepartmentInsightInput): DepartmentInsight[] {
  if (input.departments.length === 0) return []

  const insights: DepartmentInsight[] = []
  pushCoverageInsight(insights, input)
  pushGrowthInsights(insights, input)
  pushIntensityInsight(insights, input)
  pushCacheInsight(insights, input)
  pushConcentrationInsight(insights, input)
  pushClientInsight(insights, input)
  return insights
}

function pushCoverageInsight(insights: DepartmentInsight[], input: DepartmentInsightInput) {
  const total = input.summary?.total_departments || 0
  if (total <= 0) return
  const unused = Math.min(input.unusedDepartments.length, total)
  const used = total - unused
  if (unused === 0) {
    insights.push({
      id: 'coverage',
      tone: 'positive',
      icon: 'checkCircle',
      text: input.translate('departmentUsage.insightCoverageFull', { total })
    })
    return
  }
  const rate = total > 0 ? used / total : 0
  insights.push({
    id: 'coverage',
    tone: rate < 0.5 ? 'warning' : 'neutral',
    icon: 'grid',
    text: input.translate('departmentUsage.insightCoverage', {
      used,
      total,
      rate: percent(rate, 0),
      unused
    })
  })
}

function pushGrowthInsights(insights: DepartmentInsight[], input: DepartmentInsightInput) {
  const previousById = new Map(
    input.previousDepartments.map((department) => [department.group_id, department])
  )
  let best: { department: DepartmentUsageStat; delta: number } | null = null
  let worst: { department: DepartmentUsageStat; delta: number } | null = null
  for (const department of input.departments) {
    const delta = tokenDelta(department, previousById.get(department.group_id))
    if (delta === null) continue
    if (delta >= GROWTH_THRESHOLD && (!best || delta > best.delta)) {
      best = { department, delta }
    }
    if (delta <= -GROWTH_THRESHOLD && (!worst || delta < worst.delta)) {
      worst = { department, delta }
    }
  }
  if (best) {
    insights.push({
      id: 'growth',
      tone: 'neutral',
      icon: 'chart',
      text: input.translate('departmentUsage.insightGrowth', {
        name: departmentLabel(input, best.department),
        percent: `+${percent(best.delta)}`,
        tokens: formatCompactNumber(best.department.total_tokens)
      })
    })
  }
  if (worst) {
    insights.push({
      id: 'decline',
      tone: 'neutral',
      icon: 'chart',
      text: input.translate('departmentUsage.insightDecline', {
        name: departmentLabel(input, worst.department),
        percent: percent(worst.delta),
        tokens: formatCompactNumber(worst.department.total_tokens)
      })
    })
  }
}

function pushIntensityInsight(insights: DepartmentInsight[], input: DepartmentInsightInput) {
  const summary = input.summary
  if (!summary || summary.active_users <= 0 || summary.total_tokens <= 0) return
  const teamAverage = summary.total_tokens / summary.active_users
  let top: { department: DepartmentUsageStat; value: number; ratio: number } | null = null
  for (const department of input.departments) {
    if (department.active_user_count <= 0 || department.total_tokens <= 0) continue
    const value = department.total_tokens / department.active_user_count
    const ratio = value / teamAverage
    if (ratio >= INTENSITY_THRESHOLD && (!top || value > top.value)) {
      top = { department, value, ratio }
    }
  }
  if (!top) return
  insights.push({
    id: 'intensity',
    tone: 'neutral',
    icon: 'trophy',
    text: input.translate('departmentUsage.insightIntensity', {
      name: departmentLabel(input, top.department),
      perCapita: formatCompactNumber(top.value),
      ratio: top.ratio.toFixed(1)
    })
  })
}

function pushCacheInsight(insights: DepartmentInsight[], input: DepartmentInsightInput) {
  let totalRead = 0
  let totalEffective = 0
  for (const department of input.departments) {
    totalRead += department.cache_read_tokens || 0
    totalEffective += effectiveInputTokens(department)
  }
  if (totalEffective <= 0) return
  const teamRate = totalRead / totalEffective

  let worst: { department: DepartmentUsageStat; rate: number; effective: number } | null = null
  for (const department of input.departments) {
    const effective = effectiveInputTokens(department)
    if (effective < CACHE_MIN_EFFECTIVE_INPUT) continue
    const rate = cacheReadRate(department)
    if (rate === null || rate > teamRate - CACHE_GAP) continue
    if (!worst || rate < worst.rate) {
      worst = { department, rate, effective }
    }
  }
  if (!worst) return
  insights.push({
    id: 'cache',
    tone: 'warning',
    icon: 'lightbulb',
    text: input.translate('departmentUsage.insightCacheLow', {
      name: departmentLabel(input, worst.department),
      rate: percent(worst.rate),
      average: percent(teamRate),
      input: formatCompactNumber(worst.effective)
    })
  })
}

function pushConcentrationInsight(insights: DepartmentInsight[], input: DepartmentInsightInput) {
  let top:
    | { department: DepartmentUsageStat; model: string; share: number }
    | null = null
  for (const department of input.departments) {
    if (department.total_tokens <= 0) continue
    const leader = department.top_models?.[0]
    if (!leader || leader.total_tokens <= 0) continue
    const share = leader.total_tokens / department.total_tokens
    if (share < CONCENTRATION_THRESHOLD) continue
    if (!top || share > top.share) {
      top = { department, model: leader.model, share }
    }
  }
  if (!top) return
  insights.push({
    id: 'concentration',
    tone: 'neutral',
    icon: 'grid',
    text: input.translate('departmentUsage.insightConcentration', {
      name: departmentLabel(input, top.department),
      model: top.model || input.translate('departmentUsage.unassigned'),
      share: percent(top.share, 0)
    })
  })
}

function pushClientInsight(insights: DepartmentInsight[], input: DepartmentInsightInput) {
  const clients = input.clients.filter((client) => client.total_tokens > 0)
  if (clients.length === 0) return
  const total = clients.reduce((sum, client) => sum + client.total_tokens, 0)
  if (total <= 0) return
  const sorted = [...clients].sort((a, b) => b.total_tokens - a.total_tokens)
  const top = sorted[0]
  const share = top.total_tokens / total
  if (share >= CLIENT_DOMINANCE_THRESHOLD) {
    insights.push({
      id: 'clients',
      tone: 'neutral',
      icon: 'lightbulb',
      text: input.translate('departmentUsage.insightClientTop', {
        client: top.client_software || input.translate('departmentUsage.unassigned'),
        share: percent(share, 0),
        departments: top.department_count
      })
    })
    return
  }
  if (sorted.length >= CLIENT_FRAGMENTED_MIN) {
    insights.push({
      id: 'clients',
      tone: 'neutral',
      icon: 'users',
      text: input.translate('departmentUsage.insightClientFragmented', {
        count: sorted.length,
        share: percent(share, 0)
      })
    })
  }
}
