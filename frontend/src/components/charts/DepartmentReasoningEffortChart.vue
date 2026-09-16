<template>
  <div class="card p-5">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ title }}</h3>
        <p v-if="description" class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ description }}</p>
      </div>
      <div class="flex flex-wrap items-center justify-end gap-x-3 gap-y-2">
        <p v-if="efforts.length" class="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-gray-500 dark:text-dark-400">
          <span v-for="effort in efforts" :key="effort" class="inline-flex items-center gap-1">
            <span class="h-2 w-2 rounded-sm" :style="{ backgroundColor: effortColor(effort) }"></span>
            {{ effortLabel(effort) }}
          </span>
        </p>
        <slot name="actions" />
      </div>
    </div>

    <div v-if="loading" class="mt-4 flex h-40 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="hasData" class="mt-4 space-y-4">
      <div v-for="row in rows" :key="rowKey(row)" class="space-y-1.5">
        <div class="flex items-baseline justify-between gap-3 text-xs">
          <span class="truncate font-medium text-gray-700 dark:text-dark-200">{{ rowLabel(row) }}</span>
          <span class="flex-none tabular-nums text-gray-400 dark:text-dark-500">
            {{ t('departmentUsage.reasoningTotalRequests', { count: formatNumber(row.total_requests) }) }}
          </span>
        </div>
        <div class="flex h-2.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
          <div
            v-for="segment in rowSegments(row)"
            :key="segment.effort"
            class="h-full"
            :style="{ width: `${segment.share}%`, backgroundColor: effortColor(segment.effort) }"
            :title="segmentTitle(segment)"
          ></div>
        </div>
        <p class="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-gray-500 dark:text-dark-400">
          <span v-for="segment in rowSegments(row)" :key="segment.effort">
            {{ effortLabel(segment.effort) }}
            <span class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ segment.share.toFixed(1) }}%</span>
          </span>
        </p>
      </div>
    </div>
    <div v-else class="mt-4 flex h-40 items-center justify-center text-sm text-gray-500 dark:text-dark-400">
      {{ emptyText }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { DepartmentReasoningEffortRow } from '@/api/usage'
import { formatNumber } from '@/utils/format'

const props = defineProps<{
  title: string
  rows: DepartmentReasoningEffortRow[]
  efforts: string[]
  labelMode: 'department' | 'model' | 'bucket'
  emptyText: string
  description?: string
  loading?: boolean
}>()

const { t, te } = useI18n()

// Stable colors per effort tier so the department, model, and trend charts read
// the same way. Unknown tiers fall back to a hash over a fixed palette.
const EFFORT_COLORS: Record<string, string> = {
  max: '#7c3aed',
  xhigh: '#2563eb',
  high: '#0ea5e9',
  medium: '#10b981',
  low: '#f59e0b',
  minimal: '#f97316',
  none: '#94a3b8',
  unspecified: '#cbd5e1'
}

const FALLBACK_COLORS = ['#6366f1', '#14b8a6', '#ef4444', '#8b5cf6', '#22c55e']

const hasData = computed(() => props.rows.some((row) => row.total_requests > 0))

interface EffortSegment {
  effort: string
  share: number
  requests: number
  tokens: number
}

function rowKey(row: DepartmentReasoningEffortRow): string {
  if (props.labelMode === 'department') return `group-${row.group_id}`
  if (props.labelMode === 'model') return `model-${row.model}`
  return `bucket-${row.bucket}`
}

function rowLabel(row: DepartmentReasoningEffortRow): string {
  if (props.labelMode === 'department') return row.group_name || t('departmentUsage.unassigned')
  if (props.labelMode === 'model') return row.model || t('departmentUsage.unassigned')
  return row.bucket
}

function rowSegments(row: DepartmentReasoningEffortRow): EffortSegment[] {
  const total = row.total_requests || 0
  return [...row.efforts]
    .filter((bucket) => bucket.requests > 0)
    .sort((a, b) => b.requests - a.requests)
    .map((bucket) => ({
      effort: bucket.effort,
      share: total > 0 ? (bucket.requests / total) * 100 : 0,
      requests: bucket.requests,
      tokens: bucket.total_tokens
    }))
}

function segmentTitle(segment: EffortSegment): string {
  return [
    effortLabel(segment.effort),
    `${segment.share.toFixed(1)}%`,
    `${formatNumber(segment.requests)} ${t('departmentUsage.requests')}`,
    `${formatNumber(segment.tokens)} ${t('departmentUsage.tokens')}`
  ].join(' · ')
}

function effortLabel(effort: string): string {
  if (effort === 'unspecified') return t('departmentUsage.reasoningUnspecified')
  const key = `departmentUsage.effort${effort.charAt(0).toUpperCase()}${effort.slice(1).toLowerCase()}`
  return te(key) ? t(key) : effort
}

function effortColor(effort: string): string {
  const known = EFFORT_COLORS[effort]
  if (known) return known
  let hash = 0
  for (let index = 0; index < effort.length; index += 1) {
    hash = (hash * 31 + effort.charCodeAt(index)) % 997
  }
  return FALLBACK_COLORS[hash % FALLBACK_COLORS.length]
}
</script>
