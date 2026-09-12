<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center justify-between gap-4">
          <div>
            <h1 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('manager.team.title') }}</h1>
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('manager.team.description') }}</p>
          </div>
          <button
            @click="loadMembers"
            :disabled="loading"
            class="btn btn-secondary"
            :title="t('common.refresh')"
          >
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </template>

      <template #table>
        <div v-if="loading && members.length === 0" class="flex justify-center py-12">
          <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
        </div>

        <EmptyState
          v-else-if="members.length === 0"
          icon="users"
          :title="t('manager.team.emptyTitle')"
          :description="t('manager.team.emptyDescription')"
        />

        <div v-else class="space-y-6">
          <div
            v-for="member in members"
            :key="member.id"
            class="card p-4"
          >
            <!-- Member header -->
            <div class="mb-4 flex flex-wrap items-center justify-between gap-2">
              <div class="flex items-center gap-3">
                <div class="flex h-9 w-9 items-center justify-center rounded-full bg-primary-100 dark:bg-primary-900/30">
                  <span class="text-sm font-medium text-primary-700 dark:text-primary-300">
                    {{ (member.username || member.email).charAt(0).toUpperCase() }}
                  </span>
                </div>
                <div>
                  <div class="font-medium text-gray-900 dark:text-white">
                    {{ member.username || member.email }}
                    <span class="ml-2 text-xs text-gray-400">#{{ member.id }}</span>
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">{{ member.email }}</div>
                </div>
              </div>
              <span
                :class="[
                  'rounded-full px-2 py-0.5 text-xs font-medium',
                  member.status === 'active'
                    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
                    : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-400'
                ]"
              >
                {{ member.status === 'active' ? t('manager.team.active') : t('manager.team.disabled') }}
              </span>
            </div>

            <!-- Member subscriptions -->
            <div v-if="member.subscriptions.length === 0" class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('manager.team.noSubscriptions') }}
            </div>
            <div v-else class="space-y-3">
              <div
                v-for="sub in member.subscriptions"
                :key="sub.id"
                class="rounded-xl border border-gray-100 p-3 dark:border-dark-700"
              >
                <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
                  <div class="flex items-center gap-2">
                    <span class="font-medium text-gray-900 dark:text-white">
                      {{ sub.group?.name || `#${sub.group_id}` }}
                    </span>
                    <span
                      :class="[
                        'rounded-full px-2 py-0.5 text-xs font-medium',
                        sub.status === 'active'
                          ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
                          : sub.status === 'expired'
                            ? 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-400'
                            : 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
                      ]"
                    >
                      {{ t(`userSubscriptions.status.${sub.status}`) }}
                    </span>
                  </div>
                  <div class="flex items-center gap-2">
                    <button
                      type="button"
                      class="btn btn-secondary btn-sm"
                      @click="openProgress(sub)"
                    >
                      {{ t('manager.team.viewProgress') }}
                    </button>
                    <button
                      v-if="sub.status === 'active'"
                      type="button"
                      class="btn btn-secondary btn-sm"
                      @click="openResetQuota(sub)"
                    >
                      {{ t('manager.team.resetQuota') }}
                    </button>
                  </div>
                </div>

                <!-- Inline usage bars -->
                <div class="space-y-1.5">
                  <div v-if="sub.group?.daily_limit_usd" class="flex items-center gap-2 text-xs">
                    <span class="w-8 text-gray-500 dark:text-gray-400">{{ t('manager.team.daily') }}</span>
                    <div class="h-1.5 flex-1 rounded-full bg-gray-200 dark:bg-dark-600">
                      <div
                        class="h-1.5 rounded-full"
                        :class="progressBarClass(sub.daily_usage_usd, sub.group.daily_limit_usd)"
                        :style="{ width: progressWidth(sub.daily_usage_usd, sub.group.daily_limit_usd) }"
                      ></div>
                    </div>
                    <span class="tabular-nums text-gray-600 dark:text-gray-300">
                      ${{ (sub.daily_usage_usd || 0).toFixed(2) }} / ${{ sub.group.daily_limit_usd.toFixed(2) }}
                    </span>
                  </div>
                  <div v-if="sub.group?.weekly_limit_usd" class="flex items-center gap-2 text-xs">
                    <span class="w-8 text-gray-500 dark:text-gray-400">{{ t('manager.team.weekly') }}</span>
                    <div class="h-1.5 flex-1 rounded-full bg-gray-200 dark:bg-dark-600">
                      <div
                        class="h-1.5 rounded-full"
                        :class="progressBarClass(sub.weekly_usage_usd, sub.group.weekly_limit_usd)"
                        :style="{ width: progressWidth(sub.weekly_usage_usd, sub.group.weekly_limit_usd) }"
                      ></div>
                    </div>
                    <span class="tabular-nums text-gray-600 dark:text-gray-300">
                      ${{ (sub.weekly_usage_usd || 0).toFixed(2) }} / ${{ sub.group.weekly_limit_usd.toFixed(2) }}
                    </span>
                  </div>
                  <div v-if="sub.group?.monthly_limit_usd" class="flex items-center gap-2 text-xs">
                    <span class="w-8 text-gray-500 dark:text-gray-400">{{ t('manager.team.monthly') }}</span>
                    <div class="h-1.5 flex-1 rounded-full bg-gray-200 dark:bg-dark-600">
                      <div
                        class="h-1.5 rounded-full"
                        :class="progressBarClass(sub.monthly_usage_usd, sub.group.monthly_limit_usd)"
                        :style="{ width: progressWidth(sub.monthly_usage_usd, sub.group.monthly_limit_usd) }"
                      ></div>
                    </div>
                    <span class="tabular-nums text-gray-600 dark:text-gray-300">
                      ${{ (sub.monthly_usage_usd || 0).toFixed(2) }} / ${{ sub.group.monthly_limit_usd.toFixed(2) }}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <Pagination
            :page="page"
            :page-size="pageSize"
            :total="total"
            @page-change="handlePageChange"
          />
        </div>
      </template>
    </TablePageLayout>

    <!-- Progress Modal -->
    <BaseDialog :show="showProgressModal" :title="t('manager.team.progressTitle')" width="normal" @close="showProgressModal = false">
      <div v-if="progressLoading" class="flex justify-center py-8">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>
      <pre v-else class="max-h-96 overflow-auto whitespace-pre-wrap rounded-lg bg-gray-50 p-3 text-xs text-gray-700 dark:bg-dark-700 dark:text-gray-300">{{ progressText }}</pre>
      <template #footer>
        <div class="flex justify-end">
          <button type="button" class="btn btn-secondary" @click="showProgressModal = false">{{ t('common.close') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!-- Reset Quota Confirmation -->
    <ConfirmDialog
      :show="showResetDialog"
      :title="t('manager.team.resetQuotaTitle')"
      :message="t('manager.team.resetQuotaConfirm', { user: resettingSub?.user?.email || resettingUserEmail || '' })"
      :confirm-text="t('manager.team.resetQuota')"
      :cancel-text="t('common.cancel')"
      @confirm="confirmResetQuota"
      @cancel="showResetDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { managerAPI, type ManagerMember } from '@/api/manager'
import type { UserSubscription, SubscriptionProgress } from '@/types'
import { useAppStore } from '@/stores/app'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const members = ref<ManagerMember[]>([])
const page = ref(1)
const pageSize = ref(getPersistedPageSize())
const total = ref(0)

const showProgressModal = ref(false)
const progressLoading = ref(false)
const progressText = ref('')

const showResetDialog = ref(false)
const resettingSub = ref<UserSubscription | null>(null)
const resettingUserEmail = ref('')
const resetInProgress = ref(false)

async function loadMembers() {
  if (loading.value) return
  loading.value = true
  try {
    const resp = await managerAPI.listMembers(page.value, pageSize.value)
    members.value = resp.items ?? []
    total.value = resp.total ?? 0
  } catch (e: any) {
    appStore.showError(e?.message || t('manager.team.loadFailed'))
  } finally {
    loading.value = false
  }
}

function handlePageChange(p: number) {
  page.value = p
  loadMembers()
}

async function openProgress(sub: UserSubscription) {
  showProgressModal.value = true
  progressLoading.value = true
  progressText.value = ''
  try {
    const progress: SubscriptionProgress = await managerAPI.getSubscriptionProgress(sub.id)
    progressText.value = JSON.stringify(progress, null, 2)
  } catch (e: any) {
    progressText.value = e?.message || t('manager.team.loadFailed')
  } finally {
    progressLoading.value = false
  }
}

function openResetQuota(sub: UserSubscription) {
  resettingSub.value = sub
  resettingUserEmail.value = ''
  showResetDialog.value = true
}

async function confirmResetQuota() {
  const sub = resettingSub.value
  if (!sub || resetInProgress.value) return
  resetInProgress.value = true
  try {
    await managerAPI.resetQuota(sub.id, { daily: true, weekly: true, monthly: true })
    appStore.showSuccess(t('manager.team.resetQuotaSuccess'))
    showResetDialog.value = false
    await loadMembers()
  } catch (e: any) {
    appStore.showError(e?.message || t('manager.team.resetQuotaFailed'))
  } finally {
    resetInProgress.value = false
  }
}

function progressWidth(usage: number | undefined, limit: number): string {
  if (!limit || limit <= 0) return '0%'
  const pct = Math.min(100, ((usage || 0) / limit) * 100)
  return `${pct.toFixed(1)}%`
}

function progressBarClass(usage: number | undefined, limit: number): string {
  const pct = limit > 0 ? ((usage || 0) / limit) * 100 : 0
  if (pct >= 100) return 'bg-red-500'
  if (pct >= 80) return 'bg-amber-500'
  return 'bg-emerald-500'
}

onMounted(loadMembers)
</script>
