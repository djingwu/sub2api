<template>
  <div class="card p-5">
    <div class="flex items-center justify-between gap-3">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ title }}</h3>
      <slot name="actions" />
    </div>
    <div v-if="loading" class="mt-4 flex h-64 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="labels.length > 0" class="mt-4 h-64">
      <Line :data="chartData" :options="lineOptions" />
    </div>
    <div
      v-else
      class="mt-4 flex h-64 items-center justify-center text-sm text-gray-500 dark:text-dark-400"
    >
      {{ emptyText }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { formatTokensK } from '@/utils/format'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

interface TrendSeries {
  label: string
  data: number[]
  color?: string
}

const props = defineProps<{
  title: string
  labels: string[]
  series: TrendSeries[]
  emptyText: string
  loading?: boolean
}>()

const palette = ['#3b82f6', '#10b981', '#f59e0b', '#06b6d4', '#8b5cf6', '#ef4444', '#ec4899', '#84cc16']

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))
const textColor = computed(() => (isDarkMode.value ? '#e5e7eb' : '#374151'))
const gridColor = computed(() => (isDarkMode.value ? '#374151' : '#e5e7eb'))

const chartData = computed(() => {
  const singleSeries = props.series.length === 1
  return {
    labels: props.labels,
    datasets: props.series.map((serie, index) => {
      const color = serie.color || palette[index % palette.length]
      return {
        label: serie.label,
        data: serie.data,
        borderColor: color,
        backgroundColor: `${color}20`,
        fill: singleSeries,
        tension: 0.3,
        pointRadius: 2
      }
    })
  }
})

const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: {
        color: textColor.value,
        usePointStyle: true,
        pointStyle: 'circle',
        padding: 12,
        font: { size: 11 }
      }
    },
    tooltip: {
      callbacks: {
        label: (context: { dataset: { label?: string }; raw: unknown }) =>
          `${context.dataset.label ?? ''}: ${formatTokensK(Number(context.raw))}`
      }
    }
  },
  scales: {
    x: {
      grid: { color: gridColor.value },
      ticks: { color: textColor.value, font: { size: 10 } }
    },
    y: {
      beginAtZero: true,
      grid: { color: gridColor.value },
      ticks: {
        color: textColor.value,
        font: { size: 10 },
        callback: (value: string | number) => formatTokensK(Number(value))
      }
    }
  }
}))
</script>
