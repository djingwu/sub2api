<template>
  <div class="space-y-6">
    <section class="relative overflow-hidden rounded-3xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-700 dark:bg-dark-900 md:p-8">
      <div class="pointer-events-none absolute -right-16 -top-20 h-56 w-56 rounded-full bg-primary-500/10 blur-3xl"></div>
      <div class="relative flex flex-wrap items-start justify-between gap-4">
        <div class="max-w-2xl">
          <div class="mb-3 inline-flex items-center gap-2 rounded-full bg-primary-500/10 px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em] text-primary-600 dark:text-primary-400">
            <Icon name="chart" size="sm" />
            {{ t('saturation.badge') }}
          </div>
          <h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-white md:text-3xl">
            {{ t('saturation.titleAdmin') }}
          </h1>
          <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-dark-400">
            {{ t('saturation.descriptionAdmin') }}
          </p>
        </div>
        <div class="flex items-center gap-2">
          <button
            type="button"
            class="btn btn-secondary"
            @click="showConfigDialog = true"
          >
            <Icon name="cog" size="sm" />
            {{ t('saturation.config.edit') }}
          </button>
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadAll">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            <span class="hidden sm:inline">{{ t('common.refresh') }}</span>
          </button>
        </div>
      </div>
    </section>

    <div v-if="snapshotLoading && !snapshot" class="flex justify-center py-16">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
    </div>
    <div
      v-else-if="snapshotError && !snapshot"
      class="rounded-2xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-600 dark:border-red-900/40 dark:bg-red-950/30 dark:text-red-400"
    >
      {{ snapshotError }}
    </div>

    <template v-if="snapshot">
      <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <div
          v-for="card in kpiCards"
          :key="card.label"
          class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900"
        >
          <div class="text-xs font-medium uppercase tracking-wide text-gray-400 dark:text-dark-500">
            {{ card.label }}
          </div>
          <div class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
            {{ card.value }}
          </div>
          <div v-if="card.hint" class="mt-1 text-xs text-gray-400 dark:text-dark-500">
            {{ card.hint }}
          </div>
        </div>
      </div>

      <div class="grid gap-4 lg:grid-cols-2">
        <section class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
          <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ t('saturation.distribution.title') }}
              </h2>
              <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">
                {{ t('saturation.distribution.subtitle') }}
              </p>
            </div>
            <div class="inline-flex overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
              <button
                type="button"
                class="px-3 py-1 text-xs font-medium transition-colors"
                :class="!lifetimeView ? 'bg-primary-500 text-white' : 'text-gray-600 hover:bg-gray-50 dark:text-dark-300 dark:hover:bg-dark-800'"
                @click="lifetimeView = false"
              >
                {{ t('saturation.distribution.current') }}
              </button>
              <button
                type="button"
                class="px-3 py-1 text-xs font-medium transition-colors"
                :class="lifetimeView ? 'bg-primary-500 text-white' : 'text-gray-600 hover:bg-gray-50 dark:text-dark-300 dark:hover:bg-dark-800'"
                @click="lifetimeView = true"
              >
                {{ t('saturation.distribution.lifetime') }}
              </button>
            </div>
          </div>
          <div class="space-y-3">
            <div v-for="band in activeBands" :key="band.key" class="flex items-center gap-3">
              <span class="w-28 text-xs text-gray-500 dark:text-dark-400">{{ bandLabel(band) }}</span>
              <div class="h-2.5 flex-1 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
                <div
                  class="h-full rounded-full transition-all"
                  :class="bandColor(band.key)"
                  :style="{ width: bandWidth(band) }"
                ></div>
              </div>
              <span class="w-24 text-right text-xs text-gray-500 dark:text-dark-400">
                {{ t('saturation.distribution.users', { count: band.users }) }}
              </span>
            </div>
          </div>
        </section>

        <section class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">
            {{ t('saturation.baseline.title') }}
          </h2>
          <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">
            {{ t('saturation.baseline.description') }}
          </p>
          <div class="mt-4 flex flex-wrap gap-2">
            <span
              v-for="combo in snapshot.config.baseline_combos"
              :key="combo.model + ':' + combo.effort"
              class="inline-flex items-center gap-1 rounded-full bg-primary-500/10 px-3 py-1 text-xs font-medium text-primary-600 dark:text-primary-400"
            >
              {{ combo.model }}
              <span class="text-primary-400 dark:text-primary-500">/{{ combo.effort }}</span>
            </span>
            <span class="inline-flex items-center rounded-full bg-gray-100 px-3 py-1 text-xs text-gray-500 dark:bg-dark-800 dark:text-dark-400">
              {{ t('saturation.baseline.threshold', { value: snapshot.config.threshold_percent }) }}
            </span>
          </div>
          <div class="mt-5 space-y-4">
            <div>
              <div class="mb-1 flex items-center justify-between text-sm">
                <span class="text-gray-500 dark:text-dark-400">{{ t('saturation.baseline.costShare') }}</span>
                <span class="font-semibold text-gray-900 dark:text-white">{{ pct(snapshot.summary.baseline_cost_share) }}</span>
              </div>
              <div class="h-2.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
                <div
                  class="h-full rounded-full bg-primary-500 transition-all"
                  :style="{ width: clampedWidth(snapshot.summary.baseline_cost_share) }"
                ></div>
              </div>
            </div>
            <div>
              <div class="mb-1 flex items-center justify-between text-sm">
                <span class="text-gray-500 dark:text-dark-400">{{ t('saturation.baseline.tokenShare') }}</span>
                <span class="font-semibold text-gray-900 dark:text-white">{{ pct(snapshot.summary.baseline_token_share) }}</span>
              </div>
              <div class="h-2.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
                <div
                  class="h-full rounded-full bg-emerald-500 transition-all"
                  :style="{ width: clampedWidth(snapshot.summary.baseline_token_share) }"
                ></div>
              </div>
            </div>
          </div>
          <p class="mt-4 text-sm text-gray-500 dark:text-dark-400">
            {{ t('saturation.baseline.met', { count: snapshot.summary.compliant_users, total: usersWithUsageLabel }) }}
          </p>
          <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">
            {{ t('saturation.baseline.hint') }}
          </p>
        </section>
      </div>

      <SaturationTrendChart />

      <section class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">
          {{ t('saturation.combos.title') }}
        </h2>
        <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">
          {{ t('saturation.combos.description') }}
        </p>
        <div class="mt-4 overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-100 text-left text-xs uppercase tracking-wide text-gray-400 dark:border-dark-700 dark:text-dark-500">
                <th class="px-3 py-2">{{ t('saturation.combos.model') }}</th>
                <th class="px-3 py-2">{{ t('saturation.combos.effort') }}</th>
                <th class="px-3 py-2 text-right">{{ t('saturation.combos.requests') }}</th>
                <th class="px-3 py-2 text-right">{{ t('saturation.combos.tokens') }}</th>
                <th class="px-3 py-2 text-right">{{ t('saturation.combos.cost') }}</th>
                <th class="px-3 py-2 text-right">{{ t('saturation.combos.unitCost') }}</th>
                <th class="px-3 py-2 text-right">{{ t('saturation.combos.users') }}</th>
                <th class="px-3 py-2 text-right">{{ t('saturation.combos.costShare') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="combo in snapshot.combos"
                :key="combo.model + ':' + combo.effort"
                class="border-b border-gray-50 last:border-0 dark:border-dark-800"
              >
                <td class="px-3 py-2.5 font-medium text-gray-900 dark:text-white">
                  <div class="flex items-center gap-2">
                    {{ combo.model }}
                    <span
                      v-if="combo.baseline"
                      class="rounded-full bg-emerald-500/10 px-2 py-0.5 text-[10px] font-semibold uppercase text-emerald-600 dark:text-emerald-400"
                    >
                      {{ t('saturation.combos.badge') }}
                    </span>
                  </div>
                </td>
                <td class="px-3 py-2.5 text-gray-500 dark:text-dark-400">{{ combo.effort }}</td>
                <td class="px-3 py-2.5 text-right text-gray-600 dark:text-dark-300">{{ formatNumber(combo.requests) }}</td>
                <td class="px-3 py-2.5 text-right text-gray-600 dark:text-dark-300">{{ formatCompactNumber(combo.tokens) }}</td>
                <td class="px-3 py-2.5 text-right text-gray-600 dark:text-dark-300">{{ fmtUsd(combo.cost_usd) }}</td>
                <td class="px-3 py-2.5 text-right text-gray-600 dark:text-dark-300">{{ fmtUsd(combo.usd_per_million_tokens) }}</td>
                <td class="px-3 py-2.5 text-right text-gray-600 dark:text-dark-300">{{ formatNumber(combo.users) }}</td>
                <td class="px-3 py-2.5 text-right text-gray-600 dark:text-dark-300">{{ pct(combo.cost_share) }}</td>
              </tr>
              <tr v-if="snapshot.combos.length === 0">
                <td :colspan="8" class="px-3 py-8 text-center text-sm text-gray-400 dark:text-dark-500">
                  {{ t('saturation.users.empty') }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('saturation.users.title') }}
            </h2>
            <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">
              {{ t('saturation.users.subtitle') }}
            </p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <div class="relative">
              <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input
                v-model="filters.search"
                class="input !pl-9"
                :placeholder="t('saturation.users.searchPlaceholder')"
              />
            </div>
            <select v-model="filters.band" class="input w-40">
              <option value="">{{ t('saturation.users.allBands') }}</option>
              <option value="dormant">{{ t('saturation.distribution.dormant') }}</option>
              <option value="light">{{ t('saturation.distribution.light') }}</option>
              <option value="active">{{ t('saturation.distribution.active') }}</option>
              <option value="saturated">{{ t('saturation.distribution.saturated') }}</option>
            </select>
            <select v-model="filters.compliance" class="input w-32">
              <option value="">{{ t('saturation.users.allCompliance') }}</option>
              <option value="compliant">{{ t('saturation.users.compliant') }}</option>
              <option value="non_compliant">{{ t('saturation.users.nonCompliant') }}</option>
            </select>
          </div>
        </div>

        <div v-if="usersLoading" class="flex justify-center py-10">
          <div class="h-7 w-7 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
        </div>
        <div
          v-else-if="usersError"
          class="mt-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-600 dark:bg-red-950/30 dark:text-red-400"
        >
          {{ usersError }}
        </div>
        <template v-else>
          <div class="mt-4 overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="border-b border-gray-100 text-left text-xs uppercase tracking-wide text-gray-400 dark:border-dark-700 dark:text-dark-500">
                  <th class="cursor-pointer px-3 py-2" @click="toggleSort('email')">
                    {{ t('saturation.users.user') }}
                    <span v-if="filters.sort === 'email'">{{ filters.order === 'asc' ? '↑' : '↓' }}</span>
                  </th>
                  <th class="px-3 py-2">{{ t('saturation.users.dept') }}</th>
                  <th class="cursor-pointer px-3 py-2 text-right" @click="toggleSort('used_percent')">
                    {{ t('saturation.users.saturation') }}
                    <span v-if="filters.sort === 'used_percent' || filters.sort === 'used_usd'">{{ filters.order === 'asc' ? '↑' : '↓' }}</span>
                  </th>
                  <th class="cursor-pointer px-3 py-2 text-right" @click="toggleSort('lifetime_used_usd')">
                    {{ t('saturation.users.lifetime') }}
                    <span v-if="filters.sort === 'lifetime_used_usd' || filters.sort === 'lifetime_percent'">{{ filters.order === 'asc' ? '↑' : '↓' }}</span>
                  </th>
                  <th class="cursor-pointer px-3 py-2 text-right" @click="toggleSort('baseline_cost_share')">
                    {{ t('saturation.users.baselineShare') }}
                    <span v-if="filters.sort === 'baseline_cost_share'">{{ filters.order === 'asc' ? '↑' : '↓' }}</span>
                  </th>
                  <th class="px-3 py-2">{{ t('saturation.users.topCombo') }}</th>
                  <th class="cursor-pointer px-3 py-2 text-right" @click="toggleSort('lifetime_requests')">
                    {{ t('saturation.users.requests') }}
                    <span v-if="filters.sort === 'lifetime_requests'">{{ filters.order === 'asc' ? '↑' : '↓' }}</span>
                  </th>
                  <th class="cursor-pointer px-3 py-2 text-right" @click="toggleSort('last_used_at')">
                    {{ t('saturation.users.lastUsed') }}
                    <span v-if="filters.sort === 'last_used_at'">{{ filters.order === 'asc' ? '↑' : '↓' }}</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="user in users"
                  :key="user.user_id"
                  class="border-b border-gray-50 last:border-0 hover:bg-gray-50/60 dark:border-dark-800 dark:hover:bg-dark-800/40"
                >
                  <td class="px-3 py-2.5">
                    <div class="font-medium text-gray-900 dark:text-white">{{ user.username || user.email }}</div>
                    <div class="text-xs text-gray-400 dark:text-dark-500">{{ user.email }}</div>
                  </td>
                  <td class="px-3 py-2.5 text-gray-500 dark:text-dark-400">
                    {{ user.dept_name || '—' }}
                  </td>
                  <td class="px-3 py-2.5">
                    <div class="flex items-center justify-end gap-2">
                      <div class="h-2 w-20 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
                        <div
                          class="h-full rounded-full"
                          :class="bandColor(bandKey(user.used_percent))"
                          :style="{ width: clampedWidth(user.used_percent) }"
                        ></div>
                      </div>
                      <span class="w-14 text-right font-medium text-gray-900 dark:text-white">{{ pct(user.used_percent) }}</span>
                    </div>
                    <div class="mt-0.5 text-right text-xs text-gray-400 dark:text-dark-500">
                      {{ fmtUsd(user.used_usd) }} / {{ fmtUsd(user.quota_usd) }}
                    </div>
                  </td>
                  <td class="px-3 py-2.5 text-right text-gray-600 dark:text-dark-300">
                    <div>{{ fmtUsd(user.lifetime_used_usd) }}</div>
                    <div class="text-xs text-gray-400 dark:text-dark-500">
                      {{ t('saturation.users.cycles', { count: user.cycles }) }} · {{ pct(user.lifetime_percent) }}
                    </div>
                  </td>
                  <td class="px-3 py-2.5 text-right">
                    <span
                      class="rounded-full px-2 py-0.5 text-xs font-medium"
                      :class="user.compliant
                        ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                        : 'bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-dark-400'"
                    >
                      {{ pct(user.baseline_cost_share) }}
                    </span>
                    <div class="mt-0.5 text-xs text-gray-400 dark:text-dark-500">
                      Token {{ pct(user.baseline_token_share) }}
                    </div>
                  </td>
                  <td class="px-3 py-2.5 text-gray-500 dark:text-dark-400">
                    <template v-if="user.top_model">
                      {{ user.top_model }}
                      <span class="text-gray-400 dark:text-dark-500">/{{ user.top_effort }}</span>
                    </template>
                    <template v-else>—</template>
                  </td>
                  <td class="px-3 py-2.5 text-right text-gray-600 dark:text-dark-300">
                    {{ formatNumber(user.lifetime_requests) }}
                    <div class="text-xs text-gray-400 dark:text-dark-500">
                      {{ t('saturation.users.windowRequests', { count: formatNumber(user.window_requests) }) }}
                    </div>
                  </td>
                  <td class="px-3 py-2.5 text-right text-gray-500 dark:text-dark-400">
                    {{ user.last_used_at ? formatDate(user.last_used_at) : '—' }}
                  </td>
                </tr>
                <tr v-if="users.length === 0">
                  <td :colspan="8" class="px-3 py-10 text-center text-sm text-gray-400 dark:text-dark-500">
                    {{ t('saturation.users.empty') }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div v-if="userTotal > 0" class="mt-4">
            <Pagination
              :total="userTotal"
              :page="filters.page"
              :page-size="filters.page_size"
              @update:page="onPageChange"
              @update:pageSize="onPageSizeChange"
            />
          </div>
        </template>
      </section>
    </template>

    <SaturationConfigDialog
      :show="showConfigDialog"
      :config="snapshot?.config || null"
      :model-options="modelOptions"
      @close="showConfigDialog = false"
      @saved="onConfigSaved"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import SaturationConfigDialog from '@/components/saturation/SaturationConfigDialog.vue'
import SaturationTrendChart from '@/components/saturation/SaturationTrendChart.vue'
import {
  getSaturationSnapshot,
  getSaturationUsers,
  type SaturationBandStat,
  type SaturationConfig,
  type SaturationSnapshot,
  type SaturationUser,
  type SaturationUserQuery
} from '@/api/saturation'
import { formatCompactNumber, formatDate, formatNumber } from '@/utils/format'

const { t } = useI18n()

const snapshot = ref<SaturationSnapshot | null>(null)
const snapshotLoading = ref(false)
const snapshotError = ref('')
const users = ref<SaturationUser[]>([])
const userTotal = ref(0)
const usersLoading = ref(false)
const usersError = ref('')
const lifetimeView = ref(false)
const showConfigDialog = ref(false)

const filters = ref<Required<Pick<SaturationUserQuery, 'page' | 'page_size' | 'search' | 'band' | 'compliance' | 'sort' | 'order'>>>({
  page: 1,
  page_size: 20,
  search: '',
  band: '',
  compliance: '',
  sort: 'used_percent',
  order: 'desc'
})

const modelOptions = computed(() => {
  if (!snapshot.value) return []
  const models = new Set<string>()
  for (const combo of snapshot.value.combos) models.add(combo.model)
  for (const combo of snapshot.value.config.baseline_combos) models.add(combo.model)
  return Array.from(models).sort()
})

const activeBands = computed<SaturationBandStat[]>(() =>
  lifetimeView.value ? snapshot.value?.lifetime_bands || [] : snapshot.value?.current_bands || []
)

const usersWithUsageLabel = computed(() => {
  const summary = snapshot.value?.summary
  if (!summary) return '0'
  return formatNumber(Math.max(summary.user_count - summary.never_used_users, 0))
})

const kpiCards = computed(() => {
  const summary = snapshot.value?.summary
  if (!summary) return []
  return [
    {
      label: t('saturation.kpi.users'),
      value: formatNumber(summary.user_count),
      hint: t('saturation.kpi.usersHint')
    },
    {
      label: t('saturation.kpi.avgSaturation'),
      value: pct(summary.average_saturation),
      hint: t('saturation.kpi.avgSaturationHint', { value: pct(summary.average_lifetime_saturation) })
    },
    {
      label: t('saturation.kpi.used'),
      value: pct(saturationPercent(summary.used_usd, summary.quota_usd)),
      hint: `${fmtUsd(summary.used_usd)} / ${fmtUsd(summary.quota_usd)}`
    },
    {
      label: t('saturation.kpi.lifetimeUsed'),
      value: fmtUsd(summary.lifetime_used_usd),
      hint: t('saturation.kpi.lifetimeUsedHint')
    },
    {
      label: t('saturation.kpi.dormant'),
      value: formatNumber(summary.dormant_users),
      hint: t('saturation.kpi.dormantHint')
    },
    {
      label: t('saturation.kpi.heavy'),
      value: formatNumber(summary.heavy_users),
      hint: t('saturation.kpi.heavyHint')
    },
    {
      label: t('saturation.kpi.reset'),
      value: formatNumber(summary.reset_users),
      hint: t('saturation.kpi.resetHint')
    },
    {
      label: t('saturation.kpi.compliant'),
      value: `${formatNumber(summary.compliant_users)}`,
      hint: t('saturation.kpi.compliantHint', { rate: pct(summary.compliant_rate) })
    }
  ]
})

const loading = computed(() => snapshotLoading.value || usersLoading.value)

function pct(value: number | null | undefined): string {
  return `${(value || 0).toFixed(1)}%`
}

function fmtUsd(value: number | null | undefined): string {
  return `$${(value || 0).toFixed(2)}`
}

function saturationPercent(used: number, quota: number): number {
  if (!quota) return 0
  return (used / quota) * 100
}

function clampedWidth(value: number | null | undefined): string {
  return `${Math.min(Math.max(value || 0, 0), 100)}%`
}

function bandKey(percent: number): string {
  if (percent < 20) return 'dormant'
  if (percent < 50) return 'light'
  if (percent < 80) return 'active'
  return 'saturated'
}

function bandColor(key: string): string {
  switch (key) {
    case 'light':
      return 'bg-amber-400'
    case 'active':
      return 'bg-primary-500'
    case 'saturated':
      return 'bg-emerald-500'
    default:
      return 'bg-gray-400'
  }
}

function bandLabel(band: SaturationBandStat): string {
  return t(`saturation.distribution.${band.key}`)
}

function bandWidth(band: SaturationBandStat): string {
  const total = activeBands.value.reduce((sum, item) => sum + item.users, 0)
  if (!total) return '0%'
  return `${(band.users / total) * 100}%`
}

async function loadSnapshot() {
  snapshotLoading.value = true
  snapshotError.value = ''
  try {
    snapshot.value = await getSaturationSnapshot()
  } catch (err) {
    snapshotError.value = err instanceof Error ? err.message : t('saturation.loadFailed')
  } finally {
    snapshotLoading.value = false
  }
}

async function loadUsers() {
  usersLoading.value = true
  usersError.value = ''
  try {
    const data = await getSaturationUsers(filters.value)
    users.value = data.items || []
    userTotal.value = data.total || 0
  } catch (err) {
    usersError.value = err instanceof Error ? err.message : t('saturation.loadFailed')
  } finally {
    usersLoading.value = false
  }
}

function loadAll() {
  void loadSnapshot()
  void loadUsers()
}

function toggleSort(field: string) {
  if (filters.value.sort === field) {
    filters.value.order = filters.value.order === 'desc' ? 'asc' : 'desc'
  } else {
    filters.value.sort = field
    filters.value.order = 'desc'
  }
}

function onPageChange(page: number) {
  filters.value.page = page
  void loadUsers()
}

function onPageSizeChange(pageSize: number) {
  filters.value.page_size = pageSize
  filters.value.page = 1
  void loadUsers()
}

function onConfigSaved(config: SaturationConfig) {
  showConfigDialog.value = false
  if (snapshot.value) {
    snapshot.value = { ...snapshot.value, config }
  }
  loadAll()
}

let searchTimer: ReturnType<typeof setTimeout> | undefined
watch(
  () => [filters.value.search, filters.value.band, filters.value.compliance],
  () => {
    if (searchTimer) clearTimeout(searchTimer)
    searchTimer = setTimeout(() => {
      filters.value.page = 1
      void loadUsers()
    }, 300)
  }
)

watch(
  () => [filters.value.sort, filters.value.order],
  () => {
    filters.value.page = 1
    void loadUsers()
  }
)

onMounted(loadAll)
</script>
