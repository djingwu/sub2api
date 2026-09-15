<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <section class="relative overflow-hidden rounded-3xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-700 dark:bg-dark-900 md:p-8">
        <div class="pointer-events-none absolute -right-16 -top-20 h-56 w-56 rounded-full bg-primary-500/10 blur-3xl"></div>
        <div class="relative max-w-2xl">
          <div class="mb-3 inline-flex items-center gap-2 rounded-full bg-primary-500/10 px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em] text-primary-600 dark:text-primary-400">
            <Icon name="chart" size="sm" />
            {{ t('departmentUsage.badge') }}
          </div>
          <h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-white md:text-3xl">
            {{ t('departmentUsage.title') }}
          </h1>
          <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-dark-400">
            {{ t('departmentUsage.description') }}
          </p>
        </div>
      </section>

      <!-- Filters live outside the overflow-hidden header so the date picker
           popover is never clipped by the decorative blur wrapper. -->
      <div class="flex flex-wrap items-center gap-3">
        <span class="text-sm font-medium text-gray-600 dark:text-dark-300">
          {{ t('departmentUsage.timeRange') }}
        </span>
        <DateRangePicker
          v-model:start-date="startDate"
          v-model:end-date="endDate"
          @change="loadUsage"
        />
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="loading || trendLoading || heatmapLoading"
          :title="t('common.refresh')"
          @click="loadUsage"
        >
          <Icon name="refresh" size="sm" :class="loading || trendLoading || heatmapLoading ? 'animate-spin' : ''" />
          <span class="hidden sm:inline">{{ t('common.refresh') }}</span>
        </button>
      </div>

      <section class="card overflow-hidden">
        <div class="flex flex-col gap-2 border-b border-gray-200 px-5 py-5 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between sm:px-6">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('departmentUsage.tableTitle') }}</h2>
            <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ rangeLabel }}</p>
          </div>
          <span class="text-xs text-gray-400 dark:text-dark-500">{{ t('departmentUsage.tokenOnlyNote') }}</span>
        </div>

        <div v-if="loading" class="space-y-4 p-6">
          <div v-for="index in 5" :key="index" class="flex items-center justify-between gap-4">
            <div class="skeleton h-4 w-40"></div>
            <div class="skeleton h-4 w-28"></div>
          </div>
        </div>
        <div v-else-if="departments.length === 0" class="px-6 py-16 text-center">
          <div class="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-gray-100 text-gray-400 dark:bg-dark-800 dark:text-dark-500">
            <Icon name="chart" size="md" />
          </div>
          <p class="mt-4 text-sm font-medium text-gray-700 dark:text-dark-200">{{ t('departmentUsage.noData') }}</p>
          <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ t('departmentUsage.noDataHint') }}</p>
        </div>
        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[56rem]">
            <thead class="bg-gray-50 dark:bg-dark-950/60">
              <tr>
                <th class="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400 sm:px-6">
                  <button
                    type="button"
                    class="inline-flex items-center gap-1 uppercase tracking-wider transition-colors hover:text-gray-700 dark:hover:text-dark-200"
                    :aria-label="t('departmentUsage.sortByName')"
                    :title="t('departmentUsage.sortLabel')"
                    @click="toggleSort('name')"
                  >
                    {{ t('departmentUsage.department') }}
                    <Icon v-if="sortKey === 'name'" :name="sortDirection === 'asc' ? 'arrowUp' : 'arrowDown'" size="sm" />
                  </button>
                </th>
                <th class="px-5 py-3 text-right text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                  <button
                    type="button"
                    class="ml-auto inline-flex items-center gap-1 uppercase tracking-wider transition-colors hover:text-gray-700 dark:hover:text-dark-200"
                    :aria-label="t('departmentUsage.sortByRequests')"
                    :title="t('departmentUsage.sortLabel')"
                    @click="toggleSort('requests')"
                  >
                    {{ t('departmentUsage.requests') }}
                    <Icon v-if="sortKey === 'requests'" :name="sortDirection === 'asc' ? 'arrowUp' : 'arrowDown'" size="sm" />
                  </button>
                </th>
                <th class="px-5 py-3 text-right text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                  <button
                    type="button"
                    class="ml-auto inline-flex items-center gap-1 uppercase tracking-wider transition-colors hover:text-gray-700 dark:hover:text-dark-200"
                    :aria-label="t('departmentUsage.sortByTokens')"
                    :title="t('departmentUsage.sortLabel')"
                    @click="toggleSort('tokens')"
                  >
                    {{ t('departmentUsage.tokens') }}
                    <Icon v-if="sortKey === 'tokens'" :name="sortDirection === 'asc' ? 'arrowUp' : 'arrowDown'" size="sm" />
                  </button>
                </th>
                <th class="px-5 py-3 text-right text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                  {{ t('departmentUsage.cacheHitRate') }}
                </th>
                <th class="px-5 py-3 text-right text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                  {{ t('departmentUsage.modelCount') }}
                </th>
                <th class="px-5 py-3 text-right text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                  {{ t('departmentUsage.activeUsers') }}
                </th>
                <th class="px-5 py-3 text-right text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400 sm:px-6">
                  <button
                    type="button"
                    class="ml-auto inline-flex items-center gap-1 uppercase tracking-wider transition-colors hover:text-gray-700 dark:hover:text-dark-200"
                    :aria-label="t('departmentUsage.sortByLatency')"
                    :title="t('departmentUsage.sortLabel')"
                    @click="toggleSort('duration')"
                  >
                    {{ t('departmentUsage.avgLatency') }}
                    <Icon v-if="sortKey === 'duration'" :name="sortDirection === 'asc' ? 'arrowUp' : 'arrowDown'" size="sm" />
                  </button>
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
              <template v-for="department in sortedDepartments" :key="department.group_id">
                <tr class="transition-colors hover:bg-gray-50 dark:hover:bg-dark-800/50">
                  <td class="px-5 py-4 text-sm font-medium text-gray-800 dark:text-dark-100 sm:px-6">
                    <div class="flex items-center gap-2">
                      <button
                        type="button"
                        class="inline-flex h-6 w-6 flex-none items-center justify-center rounded-md text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-dark-200"
                        :aria-expanded="isExpanded(department.group_id)"
                        :aria-label="isExpanded(department.group_id) ? t('departmentUsage.collapseTopModels') : t('departmentUsage.expandTopModels')"
                        :title="isExpanded(department.group_id) ? t('departmentUsage.collapseTopModels') : t('departmentUsage.expandTopModels')"
                        @click="toggleGroup(department.group_id)"
                      >
                        <Icon :name="isExpanded(department.group_id) ? 'chevronDown' : 'chevronRight'" size="sm" />
                      </button>
                      <span>{{ department.group_name || t('departmentUsage.unassigned') }}</span>
                    </div>
                  </td>
                  <td class="px-5 py-4 text-right text-sm tabular-nums text-gray-700 dark:text-dark-200">
                    {{ formatTokens(department.requests) }}
                  </td>
                  <td class="px-5 py-4 text-right text-sm sm:px-6">
                    <div class="font-semibold tabular-nums text-primary-600 dark:text-primary-400">
                      {{ formatTokens(department.total_tokens) }}
                    </div>
                    <div
                      v-if="department.delta"
                      class="mt-0.5 text-[11px] tabular-nums"
                      :class="deltaClass(department.delta)"
                      :title="`${t('departmentUsage.vsPrevPeriod')} · ${previousRangeLabel}`"
                    >
                      {{ department.delta.text }}
                    </div>
                  </td>
                  <td class="px-5 py-4 text-right text-sm tabular-nums text-gray-700 dark:text-dark-200">
                    {{ cacheHitRate(department) }}
                  </td>
                  <td class="px-5 py-4 text-right text-sm tabular-nums text-gray-700 dark:text-dark-200">
                    {{ formatTokens(department.model_count) }}
                  </td>
                  <td class="px-5 py-4 text-right text-sm tabular-nums text-gray-700 dark:text-dark-200">
                    {{ formatTokens(department.active_user_count) }}
                  </td>
                  <td class="px-5 py-4 text-right text-sm sm:px-6">
                    <div class="tabular-nums text-gray-700 dark:text-dark-200">
                      {{ formatLatency(department.avg_duration_ms) }}
                    </div>
                    <div class="mt-0.5 text-[11px] tabular-nums text-gray-400 dark:text-dark-500">
                      {{ t('departmentUsage.firstToken') }} {{ formatLatency(department.avg_first_token_ms) }}
                    </div>
                  </td>
                </tr>
                <tr v-if="isExpanded(department.group_id)" class="bg-gray-50/70 dark:bg-dark-950/40">
                  <td colspan="7" class="px-5 py-4 sm:px-6">
                    <div class="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
                      <div>
                        <p class="mb-2 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">
                          {{ t('departmentUsage.tokenBreakdown') }}
                        </p>
                        <dl class="space-y-1.5 text-sm">
                          <div class="flex items-center justify-between gap-3">
                            <dt class="text-gray-500 dark:text-dark-400">{{ t('departmentUsage.inputTokens') }}</dt>
                            <dd class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ formatTokens(department.input_tokens) }}</dd>
                          </div>
                          <div class="flex items-center justify-between gap-3">
                            <dt class="text-gray-500 dark:text-dark-400">{{ t('departmentUsage.outputTokens') }}</dt>
                            <dd class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ formatTokens(department.output_tokens) }}</dd>
                          </div>
                          <div class="flex items-center justify-between gap-3">
                            <dt class="text-gray-500 dark:text-dark-400">{{ t('departmentUsage.cacheCreation') }}</dt>
                            <dd class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ formatTokens(department.cache_creation_tokens) }}</dd>
                          </div>
                          <div class="flex items-center justify-between gap-3">
                            <dt class="text-gray-500 dark:text-dark-400">{{ t('departmentUsage.cacheRead') }}</dt>
                            <dd class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ formatTokens(department.cache_read_tokens) }}</dd>
                          </div>
                        </dl>
                      </div>

                      <div>
                        <p class="mb-2 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">
                          {{ t('departmentUsage.usageProfile') }}
                        </p>
                        <dl class="space-y-1.5 text-sm">
                          <div class="flex items-center justify-between gap-3">
                            <dt class="text-gray-500 dark:text-dark-400">{{ t('departmentUsage.streamShare') }}</dt>
                            <dd class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ streamShare(department) }}</dd>
                          </div>
                          <div class="flex items-center justify-between gap-3">
                            <dt class="text-gray-500 dark:text-dark-400">{{ t('departmentUsage.avgLatency') }}</dt>
                            <dd class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ formatLatency(department.avg_duration_ms) }}</dd>
                          </div>
                          <div class="flex items-center justify-between gap-3">
                            <dt class="text-gray-500 dark:text-dark-400">{{ t('departmentUsage.images') }}</dt>
                            <dd class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ formatTokens(department.image_count) }}</dd>
                          </div>
                          <div class="flex items-center justify-between gap-3">
                            <dt class="text-gray-500 dark:text-dark-400">{{ t('departmentUsage.videos') }}</dt>
                            <dd class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ formatTokens(department.video_count) }}</dd>
                          </div>
                        </dl>
                      </div>

                      <div>
                        <p class="mb-2 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">
                          {{ t('departmentUsage.concentration') }}
                        </p>
                        <div class="mb-2 flex items-center gap-4 text-xs text-gray-500 dark:text-dark-400">
                          <span>{{ t('departmentUsage.topOneShare') }} <span class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ topShare(department, 1) }}</span></span>
                          <span>{{ t('departmentUsage.topNShare', { n: department.top_models.length }) }} <span class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ topShare(department, department.top_models.length) }}</span></span>
                        </div>
                        <p class="mb-2 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">
                          {{ t('departmentUsage.topModels') }}
                        </p>
                        <p v-if="department.top_models.length === 0" class="text-sm text-gray-400 dark:text-dark-500">
                          {{ t('departmentUsage.noModelData') }}
                        </p>
                        <div v-else class="space-y-2">
                          <div
                            v-for="model in department.top_models"
                            :key="model.model"
                            class="flex items-center justify-between gap-4 text-sm"
                          >
                            <span class="truncate text-gray-600 dark:text-dark-300">{{ model.model || t('departmentUsage.unassigned') }}</span>
                            <span class="flex-none tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ formatTokens(model.total_tokens) }}</span>
                          </div>
                        </div>
                      </div>
                    </div>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </section>

      <section class="space-y-5">
        <div class="card flex flex-col gap-4 p-6 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('departmentUsage.reportTitle') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('departmentUsage.reportDescription') }}</p>
          </div>
          <div class="flex items-center gap-3">
            <span class="text-xs font-medium text-gray-500 dark:text-dark-400">{{ t('departmentUsage.granularityLabel') }}</span>
            <div class="inline-flex rounded-lg border border-gray-200 bg-gray-50 p-0.5 dark:border-dark-700 dark:bg-dark-800">
              <button
                v-for="option in granularityOptions"
                :key="option.value"
                type="button"
                class="rounded-md px-3 py-1 text-xs font-medium transition-colors"
                :class="granularity === option.value
                  ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-700 dark:text-primary-400'
                  : 'text-gray-500 hover:text-gray-700 dark:text-dark-400 dark:hover:text-dark-200'"
                @click="changeGranularity(option.value)"
              >
                {{ option.label }}
              </button>
            </div>
          </div>
        </div>

        <div class="grid gap-6 lg:grid-cols-2">
          <DepartmentTrendChart
            :title="t('departmentUsage.reportTotalTitle')"
            :labels="trendBuckets"
            :series="totalTrendSeries"
            :loading="trendLoading"
            :empty-text="t('departmentUsage.reportEmpty')"
          />
          <DepartmentTrendChart
            :title="t('departmentUsage.reportModelsTitle')"
            :labels="trendBuckets"
            :series="modelTrendSeries"
            :loading="trendLoading"
            :empty-text="t('departmentUsage.reportEmpty')"
          />
        </div>

        <DepartmentUsageHeatmap
          :title="t('departmentUsage.heatmapTitle')"
          :description="t('departmentUsage.heatmapDescription')"
          :points="heatmapPoints"
          :loading="heatmapLoading"
          :empty-text="t('departmentUsage.heatmapEmpty')"
        />
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import DepartmentTrendChart from '@/components/charts/DepartmentTrendChart.vue'
import DepartmentUsageHeatmap from '@/components/charts/DepartmentUsageHeatmap.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  getDepartmentUsage,
  getDepartmentUsageHeatmap,
  getDepartmentUsageTrend,
  type DepartmentModelTrendPoint,
  type DepartmentTrendGranularity,
  type DepartmentTrendPoint,
  type DepartmentUsageHeatmapPoint,
  type DepartmentUsageStat
} from '@/api/usage'
import { formatDateLocalInput, formatNumber } from '@/utils/format'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()

const today = new Date()
const startDate = ref(formatDateLocalInput(new Date(today.getTime() - 29 * 86400000)))
const endDate = ref(formatDateLocalInput(today))
const departments = ref<DepartmentUsageStat[]>([])
const previousDepartments = ref<DepartmentUsageStat[]>([])
const previousRangeLabel = ref('')
const loading = ref(false)
let requestSequence = 0

const granularity = ref<DepartmentTrendGranularity>('day')
const totalTrend = ref<DepartmentTrendPoint[]>([])
const modelTrend = ref<DepartmentModelTrendPoint[]>([])
const trendLoading = ref(false)
let trendSequence = 0

const heatmapPoints = ref<DepartmentUsageHeatmapPoint[]>([])
const heatmapLoading = ref(false)
let heatmapSequence = 0

const granularityOptions = computed<Array<{ value: DepartmentTrendGranularity; label: string }>>(() => [
  { value: 'day', label: t('departmentUsage.granularityDay') },
  { value: 'week', label: t('departmentUsage.granularityWeek') },
  { value: 'month', label: t('departmentUsage.granularityMonth') }
])

const rangeLabel = computed(() => `${startDate.value} - ${endDate.value}`)

type SortKey = 'name' | 'tokens' | 'requests' | 'duration'

const sortKey = ref<SortKey>('name')
const sortDirection = ref<'asc' | 'desc'>('asc')

const expandedGroups = ref<number[]>([])

const sortedDepartments = computed(() =>
  [...departments.value]
    .sort((a, b) => {
      const factor = sortDirection.value === 'asc' ? 1 : -1
      switch (sortKey.value) {
        case 'tokens':
          return (a.total_tokens - b.total_tokens) * factor
        case 'requests':
          return (a.requests - b.requests) * factor
        case 'duration':
          return ((a.avg_duration_ms || 0) - (b.avg_duration_ms || 0)) * factor
        default:
          return (a.group_name || '').localeCompare(b.group_name || '', undefined, { sensitivity: 'base' }) * factor
      }
    })
    .map((department) => ({ ...department, delta: deltaByGroup.value.get(department.group_id) }))
)

interface DeltaInfo {
  text: string
  tone: 'up' | 'down' | 'flat'
}

// Compare against the immediately preceding window of the same length. Any
// failure is non-fatal: the current period simply renders without a delta.
const deltaByGroup = computed(() => {
  const map = new Map<number, DeltaInfo>()
  const previousById = new Map(previousDepartments.value.map((department) => [department.group_id, department]))
  for (const department of departments.value) {
    const previous = previousById.get(department.group_id)
    if (!previous || previous.total_tokens <= 0) {
      map.set(department.group_id, { text: `— ${t('departmentUsage.noBaseline')}`, tone: 'flat' })
      continue
    }
    const change = ((department.total_tokens - previous.total_tokens) / previous.total_tokens) * 100
    if (Math.abs(change) < 0.05) {
      map.set(department.group_id, { text: '±0.0%', tone: 'flat' })
      continue
    }
    map.set(department.group_id, {
      text: `${change > 0 ? '+' : ''}${change.toFixed(1)}%`,
      tone: change > 0 ? 'up' : 'down'
    })
  }
  return map
})

const trendBuckets = computed(() => totalTrend.value.map((point) => point.bucket))

const totalTrendSeries = computed(() => [
  {
    label: t('departmentUsage.reportTotalTitle'),
    data: totalTrend.value.map((point) => point.total_tokens)
  }
])

// Keep the model chart readable: show the busiest models and roll the rest into
// a single "other" series instead of drawing an unbounded set of lines.
const MODEL_TREND_LIMIT = 5

const modelTrendSeries = computed(() => {
  const totalsByModel = new Map<string, number>()
  const byBucket = new Map<string, Map<string, number>>()

  for (const point of modelTrend.value) {
    totalsByModel.set(point.model, (totalsByModel.get(point.model) || 0) + point.total_tokens)
    let bucket = byBucket.get(point.bucket)
    if (!bucket) {
      bucket = new Map<string, number>()
      byBucket.set(point.bucket, bucket)
    }
    bucket.set(point.model, (bucket.get(point.model) || 0) + point.total_tokens)
  }

  const topModels = [...totalsByModel.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, MODEL_TREND_LIMIT)
    .map(([model]) => model)
  const topModelSet = new Set(topModels)

  const series = topModels.map((model) => ({
    label: model,
    data: trendBuckets.value.map((bucket) => byBucket.get(bucket)?.get(model) || 0)
  }))

  const hasOther = [...totalsByModel.keys()].some((model) => !topModelSet.has(model))
  if (hasOther) {
    series.push({
      label: t('departmentUsage.otherModels'),
      data: trendBuckets.value.map((bucket) => {
        const bucketModels = byBucket.get(bucket)
        if (!bucketModels) return 0
        let sum = 0
        for (const [model, tokens] of bucketModels) {
          if (!topModelSet.has(model)) sum += tokens
        }
        return sum
      })
    })
  }

  return series
})

function formatTokens(value: number): string {
  return formatNumber(value || 0)
}

function formatLatency(value: number): string {
  if (!value || value <= 0) return '—'
  if (value < 1000) return `${Math.round(value)} ms`
  return `${(value / 1000).toFixed(2)} s`
}

function cacheHitRate(department: DepartmentUsageStat): string {
  const read = department.cache_read_tokens || 0
  const denominator = read + (department.cache_creation_tokens || 0) + (department.input_tokens || 0)
  if (denominator <= 0) return '—'
  return `${((read / denominator) * 100).toFixed(1)}%`
}

function streamShare(department: DepartmentUsageStat): string {
  if (!department.requests) return '—'
  return `${(((department.stream_requests || 0) / department.requests) * 100).toFixed(1)}%`
}

function topShare(department: DepartmentUsageStat, count: number): string {
  if (count <= 0 || !department.total_tokens) return '—'
  const top = (department.top_models || [])
    .slice(0, count)
    .reduce((sum, model) => sum + (model.total_tokens || 0), 0)
  return `${((top / department.total_tokens) * 100).toFixed(1)}%`
}

function deltaClass(delta: DeltaInfo): string {
  if (delta.tone === 'up') return 'text-emerald-600 dark:text-emerald-400'
  if (delta.tone === 'down') return 'text-red-500 dark:text-red-400'
  return 'text-gray-400 dark:text-dark-500'
}

function toggleSort(key: SortKey) {
  if (sortKey.value === key) {
    sortDirection.value = sortDirection.value === 'asc' ? 'desc' : 'asc'
    return
  }
  sortKey.value = key
  sortDirection.value = key === 'name' ? 'asc' : 'desc'
}

function isExpanded(groupId: number): boolean {
  return expandedGroups.value.includes(groupId)
}

function toggleGroup(groupId: number) {
  expandedGroups.value = isExpanded(groupId)
    ? expandedGroups.value.filter((id) => id !== groupId)
    : [...expandedGroups.value, groupId]
}

function shiftDate(value: string, days: number): string {
  const date = new Date(`${value}T00:00:00`)
  date.setDate(date.getDate() + days)
  return formatDateLocalInput(date)
}

function rangeLength(start: string, end: string): number {
  const startMs = new Date(`${start}T00:00:00`).getTime()
  const endMs = new Date(`${end}T00:00:00`).getTime()
  return Math.max(1, Math.round((endMs - startMs) / 86400000) + 1)
}

function resolvedTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || ''
  } catch {
    return ''
  }
}

async function loadDepartments() {
  const sequence = ++requestSequence
  loading.value = true
  const length = rangeLength(startDate.value, endDate.value)
  const previousEnd = shiftDate(startDate.value, -1)
  const previousStart = shiftDate(previousEnd, -(length - 1))
  try {
    const [current, previous] = await Promise.all([
      getDepartmentUsage({
        start_date: startDate.value,
        end_date: endDate.value
      }),
      getDepartmentUsage({
        start_date: previousStart,
        end_date: previousEnd
      }).catch(() => null)
    ])
    if (sequence === requestSequence) {
      departments.value = current.departments || []
      previousDepartments.value = previous?.departments || []
      previousRangeLabel.value = `${previousStart} - ${previousEnd}`
    }
  } catch (error) {
    if (sequence === requestSequence) {
      departments.value = []
      previousDepartments.value = []
      appStore.showError(t('departmentUsage.loadFailed'))
      console.error('Failed to load department usage:', error)
    }
  } finally {
    if (sequence === requestSequence) loading.value = false
  }
}

async function loadTrend() {
  const sequence = ++trendSequence
  trendLoading.value = true
  try {
    const response = await getDepartmentUsageTrend({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value
    })
    if (sequence === trendSequence) {
      totalTrend.value = response.total_trend || []
      modelTrend.value = response.model_trend || []
    }
  } catch (error) {
    if (sequence === trendSequence) {
      totalTrend.value = []
      modelTrend.value = []
      appStore.showError(t('departmentUsage.reportLoadFailed'))
      console.error('Failed to load department usage trend:', error)
    }
  } finally {
    if (sequence === trendSequence) trendLoading.value = false
  }
}

async function loadHeatmap() {
  const sequence = ++heatmapSequence
  heatmapLoading.value = true
  try {
    const response = await getDepartmentUsageHeatmap({
      start_date: startDate.value,
      end_date: endDate.value,
      timezone: resolvedTimezone()
    })
    if (sequence === heatmapSequence) {
      heatmapPoints.value = response.points || []
    }
  } catch (error) {
    if (sequence === heatmapSequence) {
      heatmapPoints.value = []
      appStore.showError(t('departmentUsage.reportLoadFailed'))
      console.error('Failed to load department usage heatmap:', error)
    }
  } finally {
    if (sequence === heatmapSequence) heatmapLoading.value = false
  }
}

function loadUsage() {
  void loadDepartments()
  void loadTrend()
  void loadHeatmap()
}

function changeGranularity(value: DepartmentTrendGranularity) {
  if (granularity.value === value) return
  granularity.value = value
  void loadTrend()
}

onMounted(loadUsage)
</script>

<style scoped>
.skeleton {
  border-radius: 0.5rem;
  background: linear-gradient(90deg, #e5e7eb 25%, #f3f4f6 50%, #e5e7eb 75%);
  background-size: 200% 100%;
  animation: department-usage-shimmer 1.8s ease-in-out infinite;
}

:global(.dark) .skeleton {
  background: linear-gradient(90deg, #334155 25%, #1e293b 50%, #334155 75%);
  background-size: 200% 100%;
}

@keyframes department-usage-shimmer {
  0% { background-position: -200% 0; }
  100% { background-position: 200% 0; }
}
</style>
