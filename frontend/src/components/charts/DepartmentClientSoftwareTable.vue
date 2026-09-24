<template>
  <div class="card overflow-hidden">
    <div v-if="loading" class="space-y-4 p-6">
      <div v-for="index in 5" :key="index" class="flex items-center justify-between gap-4">
        <div class="skeleton h-4 w-40"></div>
        <div class="skeleton h-4 w-28"></div>
      </div>
    </div>
    <div v-else-if="rows.length === 0" class="px-6 py-16 text-center">
      <div class="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-gray-100 text-gray-400 dark:bg-dark-800 dark:text-dark-500">
        <Icon name="terminal" size="md" />
      </div>
      <p class="mt-4 text-sm font-medium text-gray-700 dark:text-dark-200">{{ emptyText }}</p>
    </div>
    <div v-else class="overflow-x-auto">
      <table class="w-full min-w-[44rem]">
        <thead class="bg-gray-50 dark:bg-dark-950/60">
          <tr>
            <th class="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400 sm:px-6">
              {{ t('departmentUsage.clientSoftware') }}
            </th>
            <th class="px-5 py-3 text-right text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
              {{ t('departmentUsage.requests') }}
            </th>
            <th class="px-5 py-3 text-right text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
              {{ t('departmentUsage.tokens') }}
            </th>
            <th class="px-5 py-3 text-right text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
              {{ t('departmentUsage.activeUsers') }}
            </th>
            <th class="px-5 py-3 text-right text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400 sm:px-6">
              {{ usingLabel || t('departmentUsage.departmentsUsing') }}
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
          <tr
            v-for="(row, index) in rows"
            :key="row.client_software"
            class="client-row transition-colors hover:bg-gray-50 dark:hover:bg-dark-800/50"
            :style="{ animationDelay: `${index * 40}ms` }"
          >
            <td class="px-5 py-3.5 sm:px-6">
              <div class="flex items-center gap-2.5">
                <span
                  class="inline-flex h-7 w-7 flex-none items-center justify-center rounded-lg text-xs font-bold"
                  :style="{ backgroundColor: `${badgeColor(row.client_software)}1f`, color: badgeColor(row.client_software) }"
                >
                  {{ clientInitial(row.client_software) }}
                </span>
                <div class="min-w-0">
                  <p class="truncate text-sm font-medium text-gray-800 dark:text-dark-100" :title="row.client_software">
                    {{ row.client_software }}
                  </p>
                  <div class="mt-1 h-1.5 w-28 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
                    <div
                      class="share-bar h-full rounded-full"
                      :style="{ width: `${row.relative}%`, backgroundColor: badgeColor(row.client_software), animationDelay: `${index * 40}ms` }"
                    />
                  </div>
                </div>
              </div>
            </td>
            <td class="px-5 py-3.5 text-right text-sm tabular-nums text-gray-700 dark:text-dark-200">
              {{ formatNumber(row.requests) }}
            </td>
            <td class="px-5 py-3.5 text-right text-sm">
              <div class="font-semibold tabular-nums text-primary-600 dark:text-primary-400">
                {{ formatTokens(row.total_tokens) }}
              </div>
              <div class="mt-0.5 text-[11px] tabular-nums text-gray-400 dark:text-dark-500">
                {{ row.share.toFixed(1) }}%
              </div>
            </td>
            <td class="px-5 py-3.5 text-right text-sm tabular-nums text-gray-700 dark:text-dark-200">
              {{ formatNumber(row.user_count) }}
            </td>
            <td class="px-5 py-3.5 text-right text-sm tabular-nums text-gray-700 dark:text-dark-200 sm:px-6">
              {{ formatNumber(row.department_count) }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { formatNumber } from '@/utils/format'
import type { ClientSoftwareStat } from '@/api/usage'

const { t } = useI18n()

const props = defineProps<{
  clients: ClientSoftwareStat[]
  loading?: boolean
  emptyText: string
  usingLabel?: string
}>()

const rows = computed(() => {
  const sorted = [...props.clients].sort((a, b) => b.total_tokens - a.total_tokens)
  const total = sorted.reduce((sum, client) => sum + client.total_tokens, 0)
  const max = sorted[0]?.total_tokens || 1
  return sorted.map((client) => ({
    ...client,
    share: total > 0 ? (client.total_tokens / total) * 100 : 0,
    relative: (client.total_tokens / max) * 100
  }))
})

const BADGE_PALETTE = ['#3b82f6', '#10b981', '#8b5cf6', '#f59e0b', '#06b6d4', '#ec4899', '#84cc16', '#f97316']

function badgeColor(name: string): string {
  let hash = 0
  for (let index = 0; index < name.length; index += 1) {
    hash = (hash * 31 + name.charCodeAt(index)) % 997
  }
  return BADGE_PALETTE[hash % BADGE_PALETTE.length]
}

function clientInitial(name: string): string {
  const trimmed = (name || '').trim()
  return trimmed ? trimmed.charAt(0).toUpperCase() : '?'
}

function formatTokens(value: number): string {
  return formatNumber(value || 0)
}
</script>

<style scoped>
.client-row {
  animation: client-fade-in 0.35s ease-out forwards;
  opacity: 0;
  transform: translateY(6px);
}

.share-bar {
  animation: client-grow 0.7s ease-out forwards;
  transform-origin: left;
}

.skeleton {
  border-radius: 0.5rem;
  background: linear-gradient(90deg, #e5e7eb 25%, #f3f4f6 50%, #e5e7eb 75%);
  background-size: 200% 100%;
  animation: client-software-shimmer 1.8s ease-in-out infinite;
}

:global(.dark) .skeleton {
  background: linear-gradient(90deg, #334155 25%, #1e293b 50%, #334155 75%);
  background-size: 200% 100%;
}

@keyframes client-fade-in {
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes client-grow {
  from {
    transform: scaleX(0);
  }
  to {
    transform: scaleX(1);
  }
}

@keyframes client-software-shimmer {
  0% { background-position: -200% 0; }
  100% { background-position: 200% 0; }
}
</style>
