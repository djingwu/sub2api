<template>
  <div class="card p-5">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ title }}</h3>
        <p v-if="description" class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ description }}</p>
      </div>
      <slot name="actions" />
    </div>

    <div v-if="loading" class="mt-4 flex h-56 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="hasData" class="mt-4 overflow-x-auto">
      <div class="min-w-[40rem]">
        <div class="flex">
          <div class="w-10 flex-none"></div>
          <div class="grid flex-1 gap-0.5" :style="{ gridTemplateColumns: 'repeat(24, minmax(0, 1fr))' }">
            <div
              v-for="hour in 24"
              :key="hour"
              class="pb-1 text-center text-[10px] text-gray-400 dark:text-dark-500"
            >
              {{ (hour - 1) % 3 === 0 ? String(hour - 1).padStart(2, '0') : '' }}
            </div>
          </div>
        </div>
        <div v-for="row in rows" :key="row.weekday" class="flex items-center">
          <div class="w-10 flex-none pr-2 text-right text-[10px] font-medium text-gray-500 dark:text-dark-400">
            {{ weekdayLabel(row.weekday) }}
          </div>
          <div class="grid flex-1 gap-0.5" :style="{ gridTemplateColumns: 'repeat(24, minmax(0, 1fr))' }">
            <div
              v-for="cell in row.cells"
              :key="cell.hour"
              class="h-5 rounded-sm"
              :style="cellStyle(cell)"
              :title="cellTitle(cell)"
            ></div>
          </div>
        </div>
      </div>
    </div>
    <div
      v-else
      class="mt-4 flex h-56 items-center justify-center text-sm text-gray-500 dark:text-dark-400"
    >
      {{ emptyText }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { DepartmentUsageHeatmapPoint } from '@/api/usage'
import { formatNumber } from '@/utils/format'

const props = defineProps<{
  title: string
  points: DepartmentUsageHeatmapPoint[]
  emptyText: string
  description?: string
  loading?: boolean
}>()

const { t } = useI18n()

// Postgres DOW runs 0 = Sunday .. 6 = Saturday; render Monday first so the map
// reads like a work week.
const WEEKDAY_ORDER = [1, 2, 3, 4, 5, 6, 0]

interface HeatmapCell {
  weekday: number
  hour: number
  requests: number
  totalTokens: number
}

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))

const totalsByCell = computed(() => {
  const map = new Map<string, { requests: number; totalTokens: number }>()
  for (const point of props.points) {
    map.set(`${point.weekday}-${point.hour}`, {
      requests: point.requests,
      totalTokens: point.total_tokens
    })
  }
  return map
})

const maxRequests = computed(() => {
  let max = 0
  for (const point of props.points) {
    if (point.requests > max) max = point.requests
  }
  return max
})

const rows = computed(() =>
  WEEKDAY_ORDER.map((weekday) => ({
    weekday,
    cells: Array.from({ length: 24 }, (_, hour): HeatmapCell => {
      const entry = totalsByCell.value.get(`${weekday}-${hour}`)
      return {
        weekday,
        hour,
        requests: entry?.requests || 0,
        totalTokens: entry?.totalTokens || 0
      }
    })
  }))
)

const hasData = computed(() => props.points.some((point) => point.requests > 0 || point.total_tokens > 0))

function weekdayLabel(weekday: number): string {
  switch (weekday) {
    case 1:
      return t('departmentUsage.weekdayMon')
    case 2:
      return t('departmentUsage.weekdayTue')
    case 3:
      return t('departmentUsage.weekdayWed')
    case 4:
      return t('departmentUsage.weekdayThu')
    case 5:
      return t('departmentUsage.weekdayFri')
    case 6:
      return t('departmentUsage.weekdaySat')
    default:
      return t('departmentUsage.weekdaySun')
  }
}

function cellStyle(cell: HeatmapCell) {
  if (cell.requests <= 0) {
    return {
      backgroundColor: isDarkMode.value ? 'rgba(148, 163, 184, 0.12)' : 'rgba(148, 163, 184, 0.18)'
    }
  }
  const ratio = maxRequests.value > 0 ? cell.requests / maxRequests.value : 0
  const alpha = 0.15 + ratio * 0.85
  return { backgroundColor: `rgba(59, 130, 246, ${alpha.toFixed(3)})` }
}

function cellTitle(cell: HeatmapCell): string {
  const time = `${weekdayLabel(cell.weekday)} ${String(cell.hour).padStart(2, '0')}:00`
  return `${time} · ${t('departmentUsage.heatmapCell', {
    requests: formatNumber(cell.requests),
    tokens: formatNumber(cell.totalTokens)
  })}`
}
</script>
