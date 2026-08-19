<template>
  <div>
    <label class="input-label">
      {{ t('admin.accounts.allowedUsers') }}
      <span class="font-normal text-gray-400">{{ t('common.selectedCount', { count: modelValue.length }) }}</span>
    </label>
    <div class="flex items-center gap-2 rounded-t-lg border border-b-0 border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-600 dark:bg-dark-800">
      <Icon name="search" size="sm" class="shrink-0 text-gray-400" />
      <input
        v-model="searchText"
        type="text"
        :placeholder="t('common.searchPlaceholder')"
        class="flex-1 bg-transparent text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none dark:text-gray-100 dark:placeholder:text-dark-400"
      />
    </div>
    <div
      class="grid max-h-32 grid-cols-2 gap-1 overflow-y-auto rounded-b-lg border border-t-0 border-gray-200 bg-gray-50 p-2 dark:border-dark-600 dark:bg-dark-800"
    >
      <label
        v-for="user in filteredUsers"
        :key="user.id"
        class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 transition-colors hover:bg-white dark:hover:bg-dark-700"
        :title="user.email"
      >
        <input
          type="checkbox"
          :value="user.id"
          :checked="modelValue.includes(user.id)"
          @change="handleChange(user.id, ($event.target as HTMLInputElement).checked)"
          class="h-3.5 w-3.5 shrink-0 rounded border-gray-300 text-primary-500 focus:ring-primary-500 dark:border-dark-500"
        />
        <span class="min-w-0 flex-1 truncate text-sm text-gray-700 dark:text-gray-300">
          {{ user.username || user.email }}
        </span>
      </label>
      <button
        type="button"
        v-if="hasMore"
        class="col-span-2 py-1 text-center text-xs text-primary-500 hover:underline"
        :disabled="loading"
        @click="loadMore"
      >
        {{ loading ? t('common.loading') : t('admin.users.loadMore') }}
      </button>
      <div
        v-if="filteredUsers.length === 0 && !loading"
        class="col-span-2 py-2 text-center text-sm text-gray-500 dark:text-gray-400"
      >
        {{ t('admin.users.noUsersAvailable') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { usersAPI } from '@/api/admin/users'
import type { AdminUser } from '@/types'

const { t } = useI18n()

interface Props {
  modelValue: number[]
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: number[]]
}>()

const searchText = ref('')
const users = ref<AdminUser[]>([])
const page = ref(1)
const loading = ref(false)
const hasMore = ref(false)

const filteredUsers = computed(() => {
  if (!searchText.value) {
    return users.value
  }
  const q = searchText.value.toLowerCase()
  return users.value.filter(
    (u) => u.email?.toLowerCase().includes(q) || u.username?.toLowerCase().includes(q)
  )
})

const loadUsers = async (reset = false) => {
  if (loading.value) return
  loading.value = true
  try {
    const targetPage = reset ? 1 : page.value
    const data = await usersAPI.list(targetPage, 50)
    const incoming = data.items ?? []
    users.value = reset ? incoming : [...users.value, ...incoming]
    page.value = targetPage + 1
    hasMore.value = data.total > users.value.length
  } catch {
    // 忽略加载失败：保留当前选项，用户仍可手动输入 ID（通过后备方式）
  } finally {
    loading.value = false
  }
}

const loadMore = () => loadUsers(false)

watch(searchText, () => {
  loadUsers(true)
})

onMounted(() => {
  loadUsers(true)
})

const handleChange = (userId: number, checked: boolean) => {
  const newValue = checked
    ? [...props.modelValue, userId]
    : props.modelValue.filter((id) => id !== userId)
  emit('update:modelValue', newValue)
}
</script>