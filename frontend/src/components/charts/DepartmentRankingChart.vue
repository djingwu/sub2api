<template>
  <div class="card p-5">
    <div v-if="loading" class="flex h-64 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="ranking.length === 0" class="flex h-64 items-center justify-center text-sm text-gray-500 dark:text-dark-400">
      {{ emptyText }}
    </div>
    <div v-else>
      <div class="space-y-3">
        <div
          v-for="(item, index) in ranking"
          :key="item.id"
          class="ranking-item group"
          :style="{ animationDelay: `${index * 80}ms` }"
        >
          <div class="flex items-center gap-3">
            <div
              class="flex h-7 w-7 flex-none items-center justify-center rounded-full text-xs font-bold transition-transform group-hover:scale-110"
              :class="getRankBadgeClass(item.rank)"
            >
              {{ item.rank }}
            </div>
            <div class="min-w-0 flex-1">
              <div class="flex items-baseline justify-between gap-2">
                <span class="truncate text-sm font-medium text-gray-800 dark:text-dark-100">
                  {{ item.label }}
                </span>
                <span class="flex-none text-xs tabular-nums text-gray-500 dark:text-dark-400">
                  <span class="font-semibold text-gray-700 dark:text-dark-200">{{ item.tokensText }}</span>
                  <span class="ml-2 text-gray-400 dark:text-dark-500">{{ item.share.toFixed(1) }}%</span>
                </span>
              </div>
              <div class="mt-1.5 h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
                <div
                  class="bar-fill h-full rounded-full"
                  :style="{ width: `${item.relative}%`, backgroundColor: getBarColor(item.rank) }"
                />
              </div>
            </div>
          </div>
        </div>
      </div>
      <p v-if="hiddenCount > 0" class="mt-4 text-center text-xs text-gray-400 dark:text-dark-500">
        {{ t('departmentUsage.rankingHidden', { count: hiddenCount }) }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { formatNumber } from '@/utils/format'
import type { DepartmentUsageStat } from '@/api/usage'

const { t } = useI18n()

const props = defineProps<{
  departments: DepartmentUsageStat[]
  anonymousLabels: Record<number, string>
  loading?: boolean
  emptyText: string
}>()

const RANKING_LIMIT = 10

const ranking = computed(() => {
  const sorted = [...props.departments].sort((a, b) => b.total_tokens - a.total_tokens)
  const total = sorted.reduce((sum, dept) => sum + dept.total_tokens, 0)
  const max = sorted[0]?.total_tokens || 1
  return sorted.slice(0, RANKING_LIMIT).map((dept, index) => ({
    id: dept.group_id,
    label: props.anonymousLabels[dept.group_id] || t('departmentUsage.unassigned'),
    tokensText: formatNumber(dept.total_tokens || 0),
    share: total > 0 ? (dept.total_tokens / total) * 100 : 0,
    relative: (dept.total_tokens / max) * 100,
    rank: index + 1
  }))
})

const hiddenCount = computed(() => Math.max(0, props.departments.length - RANKING_LIMIT))

function getRankBadgeClass(rank: number): string {
  if (rank === 1) return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-500/20 dark:text-yellow-300'
  if (rank === 2) return 'bg-gray-200 text-gray-700 dark:bg-gray-500/20 dark:text-gray-300'
  if (rank === 3) return 'bg-orange-100 text-orange-700 dark:bg-orange-500/20 dark:text-orange-300'
  return 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-400'
}

function getBarColor(rank: number): string {
  if (rank === 1) return '#f59e0b'
  if (rank === 2) return '#9ca3af'
  if (rank === 3) return '#f97316'
  return '#3b82f6'
}
</script>

<style scoped>
.ranking-item {
  animation: ranking-slide-in 0.5s ease-out forwards;
  opacity: 0;
  transform: translateX(-16px);
}

.bar-fill {
  animation: ranking-grow 0.8s ease-out forwards;
  transform-origin: left;
}

@keyframes ranking-slide-in {
  to {
    opacity: 1;
    transform: translateX(0);
  }
}

@keyframes ranking-grow {
  from {
    transform: scaleX(0);
  }
  to {
    transform: scaleX(1);
  }
}
</style>
