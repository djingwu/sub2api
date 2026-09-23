<template>
  <section class="card p-5 sm:p-6">
    <div>
      <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('departmentUsage.insightsTitle') }}</h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('departmentUsage.insightsDescription') }}</p>
    </div>

    <div v-if="loading" class="mt-4 grid gap-3 sm:grid-cols-2">
      <div v-for="index in 4" :key="index" class="flex items-center gap-3 rounded-xl border border-gray-100 p-3 dark:border-dark-800">
        <div class="skeleton h-8 w-8 flex-none rounded-lg"></div>
        <div class="skeleton h-4 w-full"></div>
      </div>
    </div>
    <p v-else-if="insights.length === 0" class="mt-4 text-sm text-gray-500 dark:text-dark-400">
      {{ t('departmentUsage.insightsEmpty') }}
    </p>
    <ul v-else class="mt-4 grid gap-3 sm:grid-cols-2">
      <li
        v-for="(insight, index) in insights"
        :key="insight.id"
        class="insight-item flex items-start gap-3 rounded-xl border p-3"
        :class="toneClass(insight.tone)"
        :style="{ animationDelay: `${index * 50}ms` }"
      >
        <span class="flex h-8 w-8 flex-none items-center justify-center rounded-lg bg-white/70 dark:bg-dark-900/40">
          <Icon :name="insight.icon" size="sm" :class="iconClass(insight.tone)" />
        </span>
        <p class="pt-1 text-sm leading-5 text-gray-700 dark:text-dark-200">{{ insight.text }}</p>
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { DepartmentInsight, DepartmentInsightTone } from '@/utils/departmentInsights'

defineProps<{
  insights: DepartmentInsight[]
  loading?: boolean
}>()

const { t } = useI18n()

function toneClass(tone: DepartmentInsightTone): string {
  if (tone === 'positive') return 'border-emerald-200 bg-emerald-50/60 dark:border-emerald-500/20 dark:bg-emerald-500/10'
  if (tone === 'warning') return 'border-amber-200 bg-amber-50/60 dark:border-amber-500/20 dark:bg-amber-500/10'
  return 'border-gray-200 bg-gray-50/70 dark:border-dark-700 dark:bg-dark-800/40'
}

function iconClass(tone: DepartmentInsightTone): string {
  if (tone === 'positive') return 'text-emerald-600 dark:text-emerald-400'
  if (tone === 'warning') return 'text-amber-600 dark:text-amber-400'
  return 'text-primary-600 dark:text-primary-400'
}
</script>

<style scoped>
.insight-item {
  animation: insight-fade-in 0.4s ease-out forwards;
  opacity: 0;
  transform: translateY(6px);
}

.skeleton {
  border-radius: 0.5rem;
  background: linear-gradient(90deg, #e5e7eb 25%, #f3f4f6 50%, #e5e7eb 75%);
  background-size: 200% 100%;
  animation: department-insights-shimmer 1.8s ease-in-out infinite;
}

:global(.dark) .skeleton {
  background: linear-gradient(90deg, #334155 25%, #1e293b 50%, #334155 75%);
  background-size: 200% 100%;
}

@keyframes insight-fade-in {
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes department-insights-shimmer {
  0% { background-position: -200% 0; }
  100% { background-position: 200% 0; }
}
</style>
