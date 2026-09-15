<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="space-y-3">
          <div class="flex flex-wrap items-center justify-between gap-4">
            <div class="lg:hidden">
              <h1 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('admin.managerScopes.title') }}
              </h1>
              <p class="text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.managerScopes.description') }}
              </p>
            </div>
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="loadingManagers || loadingDepartments"
              :title="t('common.refresh')"
              @click="refresh"
            >
              <Icon name="refresh" size="md" :class="(loadingManagers || loadingDepartments) ? 'animate-spin' : ''" />
              <span class="hidden sm:inline">{{ t('common.refresh') }}</span>
            </button>
          </div>

          <div
            v-if="errorMessage"
            class="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300"
            role="alert"
          >
            {{ errorMessage }}
          </div>
          <div
            v-if="successMessage"
            class="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-300"
            role="status"
          >
            {{ successMessage }}
          </div>

          <div class="flex flex-col gap-2 sm:max-w-md">
            <label for="manager-scope-manager" class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.managerScopes.manager') }}
            </label>
            <Select
              id="manager-scope-manager"
              :model-value="selectedManagerId"
              :options="managerOptions"
              :placeholder="t('admin.managerScopes.selectManager')"
              :disabled="loadingManagers || managers.length === 0"
              searchable
              @update:model-value="handleManagerChange"
            />
          </div>
        </div>
      </template>

      <template #table>
        <div v-if="loadingManagers && managers.length === 0" class="flex justify-center py-16">
          <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
        </div>

        <EmptyState
          v-else-if="managers.length === 0"
          icon="users"
          :title="t('admin.managerScopes.noManagers')"
          :description="t('admin.managerScopes.noManagersDescription')"
        />

        <template v-else>
          <div class="hidden overflow-x-auto md:block">
            <table>
              <thead>
                <tr>
                  <th>{{ t('admin.managerScopes.manager') }}</th>
                  <th>{{ t('common.status') }}</th>
                  <th>{{ t('admin.managerScopes.primaryDepartment') }}</th>
                  <th class="text-right">{{ t('common.actions') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="manager in managers" :key="manager.id">
                  <td>
                    <div class="font-medium text-gray-900 dark:text-white">
                      {{ manager.username || manager.email }}
                      <span class="ml-1 text-xs text-gray-400">#{{ manager.id }}</span>
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">{{ manager.email }}</div>
                  </td>
                  <td>
                    <span :class="statusClass(manager.status)">
                      {{ manager.status === 'active' ? t('common.active') : t('common.disabled') }}
                    </span>
                  </td>
                  <td class="text-gray-600 dark:text-gray-300">
                    {{ departmentName(manager.primary_dept_id) || t('admin.managerScopes.notAssigned') }}
                  </td>
                  <td class="text-right">
                    <button type="button" class="btn btn-secondary btn-sm" @click="selectManager(manager.id)">
                      {{ t('admin.managerScopes.editDepartments') }}
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div class="space-y-3 p-4 md:hidden">
            <article
              v-for="manager in managers"
              :key="manager.id"
              class="rounded-xl border border-gray-200 p-4 dark:border-dark-700"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="truncate font-medium text-gray-900 dark:text-white">
                    {{ manager.username || manager.email }}
                    <span class="text-xs text-gray-400">#{{ manager.id }}</span>
                  </div>
                  <div class="truncate text-xs text-gray-500 dark:text-gray-400">{{ manager.email }}</div>
                </div>
                <span :class="statusClass(manager.status)">
                  {{ manager.status === 'active' ? t('common.active') : t('common.disabled') }}
                </span>
              </div>
              <div class="mt-3 text-sm text-gray-600 dark:text-gray-300">
                {{ t('admin.managerScopes.primaryDepartment') }}:
                {{ departmentName(manager.primary_dept_id) || t('admin.managerScopes.notAssigned') }}
              </div>
              <button type="button" class="btn btn-secondary mt-4 w-full" @click="selectManager(manager.id)">
                {{ t('admin.managerScopes.editDepartments') }}
              </button>
            </article>
          </div>
        </template>
      </template>

      <template #pagination>
        <Pagination
          :page="page"
          :page-size="pageSize"
          :total="total"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <BaseDialog
      :show="showDepartmentDialog"
      :title="t('admin.managerScopes.editDepartments')"
      width="normal"
      @close="closeDialog"
    >
      <div v-if="selectedManager" class="space-y-4">
        <div>
          <div class="font-medium text-gray-900 dark:text-white">
            {{ selectedManager.username || selectedManager.email }}
          </div>
          <div class="text-sm text-gray-500 dark:text-gray-400">{{ selectedManager.email }}</div>
        </div>

        <div v-if="loadingManagerDepartments" class="flex justify-center py-10">
          <div class="h-7 w-7 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
        </div>
        <div
          v-else-if="departments.length === 0"
          class="rounded-xl border border-dashed border-gray-300 px-4 py-8 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
        >
          {{ t('admin.managerScopes.noDepartments') }}
        </div>
        <div v-else class="max-h-[min(28rem,60vh)] overflow-y-auto rounded-xl border border-gray-200 dark:border-dark-700">
          <label
            v-for="department in departments"
            :key="department.dept_id"
            class="flex cursor-pointer items-start gap-3 border-b border-gray-100 px-4 py-3 last:border-b-0 hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-700/50"
          >
            <input
              v-model="selectedDepartmentIds"
              type="checkbox"
              :value="department.dept_id"
              class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-800"
            />
            <span class="min-w-0 flex-1">
              <span class="block text-sm font-medium text-gray-900 dark:text-white">{{ department.name }}</span>
              <span class="mt-0.5 block text-xs text-gray-500 dark:text-gray-400">
                #{{ department.dept_id }}
                <span v-if="!department.is_active" class="ml-2 text-amber-600 dark:text-amber-400">
                  {{ t('admin.managerScopes.inactive') }}
                </span>
              </span>
            </span>
          </label>
        </div>
      </div>

      <template #footer>
        <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button type="button" class="btn btn-secondary" :disabled="saving" @click="closeDialog">
            {{ t('common.cancel') }}
          </button>
          <button
            type="button"
            class="btn btn-primary"
            :disabled="saving || loadingManagerDepartments || !selectedManager"
            @click="saveDepartments"
          >
            <span v-if="saving" class="mr-2 inline-block h-4 w-4 animate-spin rounded-full border-2 border-white/40 border-t-white"></span>
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type ManagerScopeDepartment, type ManagerScopeManager } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'

const { t } = useI18n()
const appStore = useAppStore()

const departments = ref<ManagerScopeDepartment[]>([])
const managers = ref<ManagerScopeManager[]>([])
const page = ref(1)
const pageSize = ref(getPersistedPageSize())
const total = ref(0)
const selectedManagerId = ref<number | null>(null)
const selectedDepartmentIds = ref<number[]>([])
const showDepartmentDialog = ref(false)
const loadingDepartments = ref(false)
const loadingManagers = ref(false)
const loadingManagerDepartments = ref(false)
const saving = ref(false)
const errorMessage = ref('')
const successMessage = ref('')

const selectedManager = computed(() => managers.value.find((manager) => manager.id === selectedManagerId.value) ?? null)
const managerOptions = computed<SelectOption[]>(() =>
  managers.value.map((manager) => ({
    value: manager.id,
    label: `${manager.username || manager.email} (${manager.email})`
  }))
)

function getErrorMessage(error: unknown): string {
  return (error as { message?: string })?.message || t('admin.managerScopes.loadFailed')
}

async function loadDepartments() {
  loadingDepartments.value = true
  try {
    departments.value = await adminAPI.managerScopes.listDepartments()
  } catch (error) {
    errorMessage.value = getErrorMessage(error)
  } finally {
    loadingDepartments.value = false
  }
}

async function loadManagers() {
  loadingManagers.value = true
  try {
    const response = await adminAPI.managerScopes.listManagers(page.value, pageSize.value)
    managers.value = response.items ?? []
    total.value = response.total ?? 0
    if (selectedManagerId.value && !managers.value.some((manager) => manager.id === selectedManagerId.value)) {
      closeDialog()
      selectedManagerId.value = null
      selectedDepartmentIds.value = []
    }
  } catch (error) {
    errorMessage.value = getErrorMessage(error)
  } finally {
    loadingManagers.value = false
  }
}

async function refresh() {
  errorMessage.value = ''
  successMessage.value = ''
  await Promise.all([loadDepartments(), loadManagers()])
  if (selectedManagerId.value) {
    await loadSelectedManagerDepartments()
  }
}

async function loadSelectedManagerDepartments() {
  if (!selectedManagerId.value) return
  loadingManagerDepartments.value = true
  try {
    const assigned = await adminAPI.managerScopes.getManagerDepartments(selectedManagerId.value)
    selectedDepartmentIds.value = assigned.map((department) => department.dept_id)
  } catch (error) {
    errorMessage.value = getErrorMessage(error)
  } finally {
    loadingManagerDepartments.value = false
  }
}

async function selectManager(managerId: number) {
  selectedManagerId.value = managerId
  errorMessage.value = ''
  successMessage.value = ''
  await loadSelectedManagerDepartments()
  if (!errorMessage.value) showDepartmentDialog.value = true
}

function handleManagerChange(value: string | number | boolean | null) {
  if (typeof value !== 'number' && typeof value !== 'string') return
  const managerId = Number(value)
  if (Number.isInteger(managerId) && managerId > 0) selectManager(managerId)
}

async function saveDepartments() {
  if (!selectedManagerId.value || saving.value) return
  saving.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const updated = await adminAPI.managerScopes.updateManagerDepartments(
      selectedManagerId.value,
      selectedDepartmentIds.value.map(Number)
    )
    selectedDepartmentIds.value = updated.map((department) => department.dept_id)
    await Promise.all([loadManagers(), loadSelectedManagerDepartments()])
    successMessage.value = t('admin.managerScopes.saveSuccess')
    showDepartmentDialog.value = false
    appStore.showSuccess(t('admin.managerScopes.saveSuccess'))
  } catch (error) {
    errorMessage.value = getErrorMessage(error)
  } finally {
    saving.value = false
  }
}

function closeDialog() {
  showDepartmentDialog.value = false
}

function handlePageChange(nextPage: number) {
  page.value = nextPage
  loadManagers()
}

function handlePageSizeChange(nextPageSize: number) {
  pageSize.value = nextPageSize
  page.value = 1
  loadManagers()
}

function departmentName(departmentId?: number | null): string | undefined {
  if (!departmentId) return undefined
  return departments.value.find((department) => department.dept_id === departmentId)?.name
}

function statusClass(status: string): string {
  return status === 'active'
    ? 'rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
    : 'rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-400'
}

onMounted(refresh)
</script>
