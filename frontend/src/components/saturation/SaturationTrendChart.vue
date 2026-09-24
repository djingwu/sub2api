<template>
  <section class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">
          {{ t('saturation.trend.title') }}
        </h2>
        <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">
          {{ t('saturation.trend.subtitle') }}
        </p>
      </div>
      <div class="flex items-center gap-2">
        <div class="inline-flex overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
          <button
            v-for="option in metricOptions"
            :key="option.value"
            type="button"
            class="px-3 py-1 text-xs font-medium transition-colors"
            :class="metric === option.value
              ? 'bg-primary-500 text-white'
              : 'text-gray-600 hover:bg-gray-50 dark:text-dark-300 dark:hover:bg-dark-800'"
            @click="metric = option.value"
          >
            {{ option.label }}
          </button>
        </div>
        <select v-model.number="days" class="input w-28 !py-1 text-xs">
          <option :value="7">{{ t('saturation.trend.last7Days') }}</option>
          <option :value="14">{{ t('saturation.trend.last14Days') }}</option>
          <option :value="30">{{ t('saturation.trend.last30Days') }}</option>
          <option :value="90">{{ t('saturation.trend.last90Days') }}</option>
        </select>
      </div>
    </div>

    <div v-if="loading && points.length === 0" class="flex h-48 items-center justify-center">
      <div class="h-7 w-7 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
    </div>
    <div
      v-else-if="error && points.length === 0"
      class="flex h-48 items-center justify-center text-sm text-red-500 dark:text-red-400"
    >
      {{ error }}
    </div>
    <div v-else-if="points.length === 0" class="flex h-48 items-center justify-center text-sm text-gray-400 dark:text-dark-500">
      {{ t('saturation.users.empty') }}
    </div>
    <div v-else>
      <div class="flex items-end gap-[3px]" :style="{ height: '180px' }">
        <div
          v-for="point in points"
          :key="point.date"
          class="group relative flex min-w-0 flex-1 items-end self-stretch"
        >
          <div
            class="bar-fill w-full rounded-t transition-all"
            :class="barColor(point)"
            :style="{ height: barHeight(point) }"
          ></div>
          <div
            class="pointer-events-none absolute bottom-full left-1/2 z-10 mb-2 hidden w-44 -translate-x-1/2 rounded-lg bg-gray-900 px-3 py-2 text-xs text-white shadow-lg group-hover:block dark:bg-black"
          >
            <div class="font-semibold">{{ point.date }}</div>
            <div class="mt-1 tabular-nums">
              {{ t('saturation.trend.tooltipTokens', { value: formatCompactNumber(point.tokens) }) }}
            </div>
            <div class="tabular-nums">
              {{ t('saturation.trend.tooltipCost', { value: `$${point.cost_usd.toFixed(2)}` }) }}
            </div>
            <div class="tabular-nums text-gray-300">
              {{ t('saturation.trend.tooltipRequests', { value: formatNumber(point.requests) }) }}
            </div>
          </div>
        </div>
      </div>
      <div class="mt-2 flex justify-between text-[11px] text-gray-400 dark:text-dark-500">
        <span>{{ firstDate }}</span>
        <span>{{ middleDate }}</span>
        <span>{{ lastDate }}</span>
      </div>
      <div class="mt-3 flex flex-wrap items-center gap-x-5 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
        <span>
          {{ t('saturation.trend.totalTokens', { value: formatCompactNumber(totalTokens) }) }}
        </span>
        <span>
          {{ t('saturation.trend.totalCost', { value: `$${totalCost.toFixed(2)}` }) }}
        </span>
        <span>
          {{ t('saturation.trend.dailyAverage', { value: formatCompactNumber(averageValue) }) }}
        </span>
        <span>
          {{ t('saturation.trend.peakDay', { date: peakDate, value: metric === 'cost' ? `$${peakValue.toFixed(2)}` : formatCompactNumber(peakValue) }) }}
        </span>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getSaturationTrend, type SaturationTrendPoint } from '@/api/saturation'
import { formatCompactNumber, formatNumber } from '@/utils/format'

const { t } = useI18n()

type TrendMetric = 'tokens' | 'cost'

const points = ref<SaturationTrendPoint[]>([])
const loading = ref(false)
const error = ref('')
const metric = ref<TrendMetric>('tokens')
const days = ref(30)

const metricOptions = computed(() => [
  { value: 'tokens' as TrendMetric, label: t('saturation.trend.tokens') },
  { value: 'cost' as TrendMetric, label: t('saturation.trend.cost') }
])

const metricValue = (point: SaturationTrendPoint): number =>
  metric.value === 'cost' ? point.cost_usd : point.tokens

const maxValue = computed(() =>
  points.value.reduce((max, point) => Math.max(max, metricValue(point)), 0)
)

const totalTokens = computed(() => points.value.reduce((sum, point) => sum + point.tokens, 0))
const totalCost = computed(() => points.value.reduce((sum, point) => sum + point.cost_usd, 0))

const averageValue = computed(() => {
  if (points.value.length === 0) return 0
  const total = metric.value === 'cost' ? totalCost.value : totalTokens.value
  return total / points.value.length
})

const peak = computed(() => {
  let best: SaturationTrendPoint | null = null
  for (const point of points.value) {
    if (!best || metricValue(point) > metricValue(best)) best = point
  }
  return best
})

const peakDate = computed(() => peak.value?.date || '—')
const peakValue = computed(() => (peak.value ? metricValue(peak.value) : 0))

const firstDate = computed(() => points.value[0]?.date || '')
const lastDate = computed(() => points.value[points.value.length - 1]?.date || '')
const middleDate = computed(() => points.value[Math.floor(points.value.length / 2)]?.date || '')

function barColor(point: SaturationTrendPoint): string {
  if (metricValue(point) <= 0) return 'bg-gray-100 dark:bg-dark-800'
  return metric.value === 'cost' ? 'bg-emerald-500' : 'bg-primary-500'
}

function barHeight(point: SaturationTrendPoint): string {
  if (maxValue.value <= 0) return '2px'
  const ratio = metricValue(point) / maxValue.value
  return `${Math.max(ratio * 100, ratio > 0 ? 2 : 0.5)}%`
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    points.value = await getSaturationTrend(days.value)
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('saturation.loadFailed')
  } finally {
    loading.value = false
  }
}

watch(days, load)
onMounted(load)

defineExpose({ reload: load })
</script>
