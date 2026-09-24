<template>
  <BaseDialog
    :show="show"
    :title="t('saturation.config.title')"
    width="normal"
    :close-on-click-outside="false"
    @close="emit('close')"
  >
    <div class="space-y-5">
      <p class="text-sm leading-6 text-gray-500 dark:text-dark-400">
        {{ t('saturation.config.description') }}
      </p>

      <div>
        <div class="mb-2 flex items-center justify-between">
          <span class="text-sm font-medium text-gray-700 dark:text-dark-200">
            {{ t('saturation.config.combos') }}
          </span>
          <button type="button" class="btn btn-secondary !px-2 !py-1 text-xs" @click="addCombo">
            <Icon name="plus" size="sm" />
            {{ t('saturation.config.addCombo') }}
          </button>
        </div>
        <div class="space-y-2">
          <div
            v-for="(combo, index) in draftCombos"
            :key="index"
            class="flex items-center gap-2 rounded-xl border border-gray-200 bg-gray-50 p-2 dark:border-dark-700 dark:bg-dark-800"
          >
            <div class="flex-1">
              <input
                v-model="combo.model"
                class="input"
                list="saturation-model-options"
                :placeholder="t('saturation.config.modelPlaceholder')"
              />
            </div>
            <div class="w-32">
              <select v-model="combo.effort" class="input">
                <option v-for="effort in effortOptions" :key="effort" :value="effort">
                  {{ effort }}
                </option>
              </select>
            </div>
            <button
              type="button"
              class="btn btn-ghost !px-2 !py-1 text-gray-400 hover:text-red-500"
              :disabled="draftCombos.length <= 1"
              :title="t('common.delete')"
              @click="removeCombo(index)"
            >
              <Icon name="trash" size="sm" />
            </button>
          </div>
        </div>
        <datalist id="saturation-model-options">
          <option v-for="model in modelOptions" :key="model" :value="model" />
        </datalist>
      </div>

      <div>
        <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-dark-200">
          {{ t('saturation.config.threshold') }}
        </label>
        <input
          v-model.number="draftThreshold"
          type="number"
          min="1"
          max="100"
          step="1"
          class="input w-32"
        />
        <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">
          {{ t('saturation.config.thresholdHint') }}
        </p>
      </div>

      <p
        v-if="error"
        class="rounded-xl bg-red-50 px-3 py-2 text-sm text-red-600 dark:bg-red-950/30 dark:text-red-400"
      >
        {{ error }}
      </p>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary" @click="emit('close')">
        {{ t('common.cancel') }}
      </button>
      <button type="button" class="btn btn-primary" :disabled="saving" @click="save">
        <span v-if="saving" class="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"></span>
        {{ t('common.save') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  updateSaturationConfig,
  type SaturationConfig,
  type SaturationBaselineCombo
} from '@/api/saturation'

const props = defineProps<{
  show: boolean
  config: SaturationConfig | null
  modelOptions: string[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved', config: SaturationConfig): void
}>()

const { t } = useI18n()

const effortOptions = ['max', 'high', 'medium', 'low', 'xhigh', 'unspecified']
const draftCombos = ref<SaturationBaselineCombo[]>([])
const draftThreshold = ref(50)
const saving = ref(false)
const error = ref('')

watch(
  () => [props.show, props.config] as const,
  ([show, config]) => {
    if (!show) return
    error.value = ''
    draftCombos.value = (config?.baseline_combos || []).map((combo) => ({ ...combo }))
    if (draftCombos.value.length === 0) {
      draftCombos.value = [{ model: '', effort: 'max' }]
    }
    draftThreshold.value = config?.threshold_percent || 50
  },
  { immediate: true }
)

function addCombo() {
  draftCombos.value.push({ model: '', effort: 'max' })
}

function removeCombo(index: number) {
  if (draftCombos.value.length <= 1) return
  draftCombos.value.splice(index, 1)
}

async function save() {
  const combos = draftCombos.value
    .map((combo) => ({ model: combo.model.trim(), effort: combo.effort }))
    .filter((combo) => combo.model !== '')
  if (combos.length === 0 || !draftThreshold.value || draftThreshold.value <= 0 || draftThreshold.value > 100) {
    error.value = t('saturation.config.invalid')
    return
  }
  saving.value = true
  error.value = ''
  try {
    const saved = await updateSaturationConfig({
      baseline_combos: combos,
      threshold_percent: draftThreshold.value
    })
    emit('saved', saved)
  } catch (err) {
    error.value = err instanceof Error ? err.message : t('saturation.config.saveFailed')
  } finally {
    saving.value = false
  }
}
</script>
