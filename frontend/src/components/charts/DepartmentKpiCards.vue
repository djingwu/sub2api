<template>
  <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
    <div
      v-for="(card, index) in cards"
      :key="card.key"
      class="card kpi-card p-5"
      :style="{ animationDelay: `${index * 60}ms` }"
    >
      <div class="flex items-center justify-between gap-3">
        <p class="text-xs font-medium uppercase tracking-wider text-gray-400 dark:text-dark-500">
          {{ card.label }}
        </p>
        <div class="flex h-8 w-8 flex-none items-center justify-center rounded-xl bg-primary-500/10 text-primary-600 dark:text-primary-400">
          <Icon :name="card.icon" size="sm" />
        </div>
      </div>
      <div v-if="loading" class="mt-3 skeleton h-7 w-24"></div>
      <template v-else>
        <p class="mt-3 text-2xl font-bold tabular-nums text-gray-900 dark:text-white">
          {{ card.value }}
        </p>
        <p
          v-if="card.delta"
          class="mt-1 text-xs tabular-nums"
          :class="deltaClass(card.delta.tone)"
          :title="`${t('departmentUsage.vsPrevPeriod')}${previousRangeLabel ? ` · ${previousRangeLabel}` : ''}`"
        >
          {{ card.delta.text }}
        </p>
        <p v-if="card.hint" class="mt-1 text-xs text-gray-400 dark:text-dark-500">
          {{ card.hint }}
        </p>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { formatCompactNumber, formatNumber } from '@/utils/format'
import type { DepartmentUsageSummary } from '@/api/usage'

const { t } = useI18n()

const props = defineProps<{
  summary: DepartmentUsageSummary | null
  previousSummary: DepartmentUsageSummary | null
  previousRangeLabel?: string
  coverageHint?: string
  activeLabel?: string
  loading?: boolean
}>()

interface DeltaInfo {
  text: string
  tone: 'up' | 'down' | 'flat'
}

interface KpiCard {
  key: string
  label: string
  icon: 'chatBubble' | 'database' | 'grid' | 'users'
  value: string
  delta: DeltaInfo | null
  hint?: string
}

function deltaOf(current: number, previous: number | null | undefined): DeltaInfo | null {
  if (previous === null || previous === undefined || previous <= 0) return null
  const change = ((current - previous) / previous) * 100
  if (Math.abs(change) < 0.05) return { text: '±0.0%', tone: 'flat' }
  return {
    text: `${change > 0 ? '+' : ''}${change.toFixed(1)}%`,
    tone: change > 0 ? 'up' : 'down'
  }
}

const cards = computed<KpiCard[]>(() => {
  const current = props.summary
  const previous = props.previousSummary
  return [
    {
      key: 'requests',
      label: t('departmentUsage.kpiTotalRequests'),
      icon: 'chatBubble',
      value: formatNumber(current?.total_requests || 0),
      delta: deltaOf(current?.total_requests || 0, previous?.total_requests)
    },
    {
      key: 'tokens',
      label: t('departmentUsage.kpiTotalTokens'),
      icon: 'database',
      value: formatCompactNumber(current?.total_tokens || 0),
      delta: deltaOf(current?.total_tokens || 0, previous?.total_tokens)
    },
    {
      key: 'departments',
      label: props.activeLabel || t('departmentUsage.kpiActiveDepartments'),
      icon: 'grid',
      value: formatNumber(current?.active_departments || 0),
      delta: deltaOf(current?.active_departments || 0, previous?.active_departments),
      hint: props.coverageHint
    },
    {
      key: 'users',
      label: t('departmentUsage.kpiActiveUsers'),
      icon: 'users',
      value: formatNumber(current?.active_users || 0),
      delta: deltaOf(current?.active_users || 0, previous?.active_users)
    }
  ]
})

function deltaClass(tone: DeltaInfo['tone']): string {
  if (tone === 'up') return 'text-emerald-600 dark:text-emerald-400'
  if (tone === 'down') return 'text-red-500 dark:text-red-400'
  return 'text-gray-400 dark:text-dark-500'
}
</script>

<style scoped>
.kpi-card {
  animation: kpi-fade-in 0.4s ease-out forwards;
  opacity: 0;
  transform: translateY(8px);
}

.skeleton {
  border-radius: 0.5rem;
  background: linear-gradient(90deg, #e5e7eb 25%, #f3f4f6 50%, #e5e7eb 75%);
  background-size: 200% 100%;
  animation: department-kpi-shimmer 1.8s ease-in-out infinite;
}

:global(.dark) .skeleton {
  background: linear-gradient(90deg, #334155 25%, #1e293b 50%, #334155 75%);
  background-size: 200% 100%;
}

@keyframes kpi-fade-in {
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes department-kpi-shimmer {
  0% { background-position: -200% 0; }
  100% { background-position: 200% 0; }
}
</style>
