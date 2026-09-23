<template>
  <div class="card p-5">
    <div v-if="loading" class="flex h-64 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="models.length === 0" class="flex h-64 items-center justify-center text-sm text-gray-500 dark:text-dark-400">
      {{ emptyText }}
    </div>
    <div v-else class="space-y-3">
      <div
        v-for="(model, index) in models"
        :key="model.model"
        class="model-item group"
        :style="{ animationDelay: `${index * 60}ms` }"
      >
        <div class="mb-1.5 flex items-baseline justify-between gap-3">
          <span class="truncate text-sm font-medium text-gray-800 dark:text-dark-100" :title="model.model">
            {{ model.model || t('departmentUsage.unassigned') }}
          </span>
          <span class="flex-none text-xs tabular-nums text-gray-500 dark:text-dark-400">
            <span class="font-semibold text-primary-600 dark:text-primary-400">{{ formatTokens(model.total_tokens) }}</span>
            <span class="ml-2 text-gray-400 dark:text-dark-500">{{ model.share.toFixed(1) }}%</span>
          </span>
        </div>
        <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
          <div
            class="bar-fill h-full rounded-full transition-transform group-hover:brightness-110"
            :style="{ width: `${model.relative}%`, backgroundColor: barColor(index) }"
          />
        </div>
        <div class="mt-1 flex items-center gap-3 text-[11px] tabular-nums text-gray-400 dark:text-dark-500">
          <span>{{ formatNumber(model.requests) }} {{ t('departmentUsage.requests') }}</span>
          <span>{{ formatNumber(model.user_count) }} {{ t('departmentUsage.activeUsers') }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { formatNumber } from '@/utils/format'
import type { DepartmentModelStat } from '@/api/usage'

const { t } = useI18n()

const props = defineProps<{
  rows: DepartmentModelStat[]
  loading?: boolean
  emptyText: string
}>()

const palette = ['#3b82f6', '#10b981', '#8b5cf6', '#f59e0b', '#06b6d4', '#ec4899', '#84cc16', '#f97316', '#6366f1', '#14b8a6']

const models = computed(() => {
  const rows = [...props.rows].sort((a, b) => b.total_tokens - a.total_tokens)
  const total = rows.reduce((sum, row) => sum + row.total_tokens, 0)
  const max = rows[0]?.total_tokens || 1
  return rows.map((row) => ({
    ...row,
    share: total > 0 ? (row.total_tokens / total) * 100 : 0,
    relative: (row.total_tokens / max) * 100
  }))
})

function formatTokens(value: number): string {
  return formatNumber(value || 0)
}

function barColor(index: number): string {
  return palette[index % palette.length]
}
</script>

<style scoped>
.model-item {
  animation: model-fade-in 0.4s ease-out forwards;
  opacity: 0;
  transform: translateY(8px);
}

.bar-fill {
  animation: model-grow 0.7s ease-out forwards;
  transform-origin: left;
}

@keyframes model-fade-in {
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes model-grow {
  from {
    transform: scaleX(0);
  }
  to {
    transform: scaleX(1);
  }
}
</style>
