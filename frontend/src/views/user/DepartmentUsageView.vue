<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <section class="relative overflow-hidden rounded-3xl border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-700 dark:bg-dark-900 md:p-8">
        <div class="pointer-events-none absolute -right-16 -top-20 h-56 w-56 rounded-full bg-primary-500/10 blur-3xl"></div>
        <div class="relative max-w-2xl">
          <div class="mb-3 inline-flex items-center gap-2 rounded-full bg-primary-500/10 px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em] text-primary-600 dark:text-primary-400">
            <Icon name="chart" size="sm" />
            {{ t('departmentUsage.badge') }}
          </div>
          <h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-white md:text-3xl">
            {{ t('departmentUsage.title') }}
          </h1>
          <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-dark-400">
            {{ t('departmentUsage.description') }}
          </p>
        </div>
      </section>

      <!-- Filters live outside the overflow-hidden header so the date picker
           popover is never clipped by the decorative blur wrapper. -->
      <div class="flex flex-wrap items-center gap-3">
        <span class="text-sm font-medium text-gray-600 dark:text-dark-300">
          {{ t('departmentUsage.timeRange') }}
        </span>
        <DateRangePicker
          v-model:start-date="startDate"
          v-model:end-date="endDate"
          @change="loadUsage"
        />
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="anyLoading"
          :title="t('common.refresh')"
          @click="loadUsage"
        >
          <Icon name="refresh" size="sm" :class="anyLoading ? 'animate-spin' : ''" />
          <span class="hidden sm:inline">{{ t('common.refresh') }}</span>
        </button>
      </div>

      <DepartmentKpiCards
        :summary="summary"
        :previous-summary="previousSummary"
        :previous-range-label="previousRangeLabel"
        :coverage-hint="coverageHint"
        :loading="loading"
      />

      <DepartmentInsights :insights="insights" :loading="loading || clientSoftwareLoading" />

      <section class="card overflow-hidden">
        <div class="flex flex-col gap-2 border-b border-gray-200 px-5 py-5 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between sm:px-6">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('departmentUsage.tableTitle') }}</h2>
            <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ rangeLabel }}</p>
          </div>
          <span class="text-xs text-gray-400 dark:text-dark-500">{{ t('departmentUsage.tokenOnlyNote') }}</span>
        </div>

        <div v-if="loading" class="space-y-4 p-6">
          <div v-for="index in 5" :key="index" class="flex items-center justify-between gap-4">
            <div class="skeleton h-4 w-40"></div>
            <div class="skeleton h-4 w-28"></div>
          </div>
        </div>
        <div v-else-if="departments.length === 0" class="px-6 py-16 text-center">
          <div class="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-gray-100 text-gray-400 dark:bg-dark-800 dark:text-dark-500">
            <Icon name="chart" size="md" />
          </div>
          <p class="mt-4 text-sm font-medium text-gray-700 dark:text-dark-200">{{ t('departmentUsage.noData') }}</p>
          <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ t('departmentUsage.noDataHint') }}</p>
        </div>
        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[56rem]">
            <thead class="bg-gray-50 dark:bg-dark-950/60">
              <tr>
                <th class="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400 sm:px-6">
                  <button
                    type="button"
                    class="inline-flex items-center gap-1 uppercase tracking-wider transition-colors hover:text-gray-700 dark:hover:text-dark-200"
                    :aria-label="t('departmentUsage.sortByName')"
                    :title="t('departmentUsage.sortLabel')"
                    @click="toggleSort('name')"
                  >
                    {{ t('departmentUsage.department') }}
                    <Icon v-if="sortKey === 'name'" :name="sortDirection === 'asc' ? 'arrowUp' : 'arrowDown'" size="sm" />
                  </button>
                </th>
                <th class="px-5 py-3 text-right text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                  <button
                    type="button"
                    class="ml-auto inline-flex items-center gap-1 uppercase tracking-wider transition-colors hover:text-gray-700 dark:hover:text-dark-200"
                    :aria-label="t('departmentUsage.sortByRequests')"
                    :title="t('departmentUsage.sortLabel')"
                    @click="toggleSort('requests')"
                  >
                    {{ t('departmentUsage.requests') }}
                    <Icon v-if="sortKey === 'requests'" :name="sortDirection === 'asc' ? 'arrowUp' : 'arrowDown'" size="sm" />
                  </button>
                </th>
                <th class="px-5 py-3 text-right text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                  <button
                    type="button"
                    class="ml-auto inline-flex items-center gap-1 uppercase tracking-wider transition-colors hover:text-gray-700 dark:hover:text-dark-200"
                    :aria-label="t('departmentUsage.sortByTokens')"
                    :title="t('departmentUsage.sortLabel')"
                    @click="toggleSort('tokens')"
                  >
                    {{ t('departmentUsage.tokens') }}
                    <Icon v-if="sortKey === 'tokens'" :name="sortDirection === 'asc' ? 'arrowUp' : 'arrowDown'" size="sm" />
                  </button>
                </th>
                <th class="px-5 py-3 text-right text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                  <button
                    type="button"
                    class="ml-auto inline-flex items-center gap-1 uppercase tracking-wider transition-colors hover:text-gray-700 dark:hover:text-dark-200"
                    :aria-label="t('departmentUsage.sortByPerCapita')"
                    :title="t('departmentUsage.sortLabel')"
                    @click="toggleSort('perCapita')"
                  >
                    {{ t('departmentUsage.tokensPerUser') }}
                    <Icon v-if="sortKey === 'perCapita'" :name="sortDirection === 'asc' ? 'arrowUp' : 'arrowDown'" size="sm" />
                  </button>
                </th>
                <th class="px-5 py-3 text-right text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                  {{ t('departmentUsage.cacheHitRate') }}
                </th>
                <th class="px-5 py-3 text-right text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                  {{ t('departmentUsage.modelCount') }}
                </th>
                <th class="px-5 py-3 text-right text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400 sm:px-6">
                  {{ t('departmentUsage.activeUsers') }}
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
              <template v-for="department in sortedDepartments" :key="department.group_id">
                <tr class="transition-colors hover:bg-gray-50 dark:hover:bg-dark-800/50">
                  <td class="px-5 py-4 text-sm font-medium text-gray-800 dark:text-dark-100 sm:px-6">
                    <div class="flex items-center gap-2">
                      <button
                        type="button"
                        class="inline-flex h-6 w-6 flex-none items-center justify-center rounded-md text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-dark-200"
                        :aria-expanded="isExpanded(department.group_id)"
                        :aria-label="isExpanded(department.group_id) ? t('departmentUsage.collapseTopModels') : t('departmentUsage.expandTopModels')"
                        :title="isExpanded(department.group_id) ? t('departmentUsage.collapseTopModels') : t('departmentUsage.expandTopModels')"
                        @click="toggleGroup(department.group_id)"
                      >
                        <Icon :name="isExpanded(department.group_id) ? 'chevronDown' : 'chevronRight'" size="sm" />
                      </button>
                      <span>{{ department.group_name || t('departmentUsage.unassigned') }}</span>
                      <span
                        v-if="department.rank"
                        class="flex-none rounded-md px-1.5 py-0.5 text-[10px] font-semibold tabular-nums"
                        :class="rankChangeClass(department.rank)"
                        :title="rankChangeTitle(department.rank)"
                      >
                        {{ rankChangeText(department.rank) }}
                      </span>
                      <Icon
                        v-if="department.flags.length > 0"
                        name="exclamationTriangle"
                        size="sm"
                        class="flex-none text-amber-500 dark:text-amber-400"
                        :title="flagTitle(department.flags)"
                      />
                      <button
                        type="button"
                        class="inline-flex h-6 w-6 flex-none items-center justify-center rounded-md text-gray-400 transition-colors hover:bg-primary-50 hover:text-primary-600 dark:hover:bg-primary-500/10 dark:hover:text-primary-400"
                        :aria-label="t('departmentUsage.reasoningFilterByDepartment')"
                        :title="t('departmentUsage.reasoningFilterByDepartment')"
                        @click="focusReasoningDepartment(department.group_id)"
                      >
                        <Icon name="chart" size="sm" />
                      </button>
                    </div>
                  </td>
                  <td class="px-5 py-4 text-right text-sm tabular-nums text-gray-700 dark:text-dark-200">
                    {{ formatTokens(department.requests) }}
                  </td>
                  <td class="px-5 py-4 text-right text-sm sm:px-6">
                    <div class="font-semibold tabular-nums text-primary-600 dark:text-primary-400">
                      {{ formatTokens(department.total_tokens) }}
                    </div>
                    <div
                      v-if="department.delta"
                      class="mt-0.5 text-[11px] tabular-nums"
                      :class="deltaClass(department.delta)"
                      :title="`${t('departmentUsage.vsPrevPeriod')} · ${previousRangeLabel}`"
                    >
                      {{ department.delta.text }}
                    </div>
                    <div class="mt-1 flex items-center justify-end gap-2">
                      <div class="h-1 w-16 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-800">
                        <div
                          class="h-full rounded-full bg-primary-500/60"
                          :style="{ width: `${department.sharePercent}%` }"
                        />
                      </div>
                      <span
                        class="text-[11px] tabular-nums text-gray-400 dark:text-dark-500"
                        :title="teamShareText(department)"
                      >
                        {{ department.sharePercent.toFixed(1) }}%
                      </span>
                    </div>
                  </td>
                  <td class="px-5 py-4 text-right text-sm tabular-nums text-gray-700 dark:text-dark-200">
                    <div>{{ perCapitaTokens(department) }}</div>
                    <div
                      v-if="department.intensityRatio"
                      class="mt-0.5 text-[11px] text-gray-400 dark:text-dark-500"
                      :title="t('departmentUsage.intensityVsTeam', { ratio: department.intensityRatio.toFixed(1) })"
                    >
                      ×{{ department.intensityRatio.toFixed(1) }}
                    </div>
                  </td>
                  <td
                    class="px-5 py-4 text-right text-sm tabular-nums text-gray-700 dark:text-dark-200"
                    :title="teamCacheHitRate > 0 ? t('departmentUsage.teamAverageCacheRate', { value: `${(teamCacheHitRate * 100).toFixed(1)}%` }) : undefined"
                  >
                    {{ cacheHitRate(department) }}
                  </td>
                  <td class="px-5 py-4 text-right text-sm tabular-nums text-gray-700 dark:text-dark-200">
                    {{ formatTokens(department.model_count) }}
                  </td>
                  <td class="px-5 py-4 text-right text-sm tabular-nums text-gray-700 dark:text-dark-200">
                    {{ formatTokens(department.active_user_count) }}
                  </td>
                </tr>
                <tr v-if="isExpanded(department.group_id)" class="bg-gray-50/70 dark:bg-dark-950/40">
                  <td colspan="7" class="px-5 py-4 sm:px-6">
                    <div class="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
                      <div>
                        <p class="mb-2 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">
                          {{ t('departmentUsage.tokenBreakdown') }}
                        </p>
                        <dl class="space-y-1.5 text-sm">
                          <div class="flex items-center justify-between gap-3">
                            <dt class="text-gray-500 dark:text-dark-400">{{ t('departmentUsage.inputTokens') }}</dt>
                            <dd class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ formatTokens(department.input_tokens) }}</dd>
                          </div>
                          <div class="flex items-center justify-between gap-3">
                            <dt class="text-gray-500 dark:text-dark-400">{{ t('departmentUsage.outputTokens') }}</dt>
                            <dd class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ formatTokens(department.output_tokens) }}</dd>
                          </div>
                          <div class="flex items-center justify-between gap-3">
                            <dt class="text-gray-500 dark:text-dark-400">{{ t('departmentUsage.cacheCreation') }}</dt>
                            <dd class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ formatTokens(department.cache_creation_tokens) }}</dd>
                          </div>
                          <div class="flex items-center justify-between gap-3">
                            <dt class="text-gray-500 dark:text-dark-400">{{ t('departmentUsage.cacheRead') }}</dt>
                            <dd class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ formatTokens(department.cache_read_tokens) }}</dd>
                          </div>
                        </dl>
                      </div>

                      <div>
                        <p class="mb-2 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">
                          {{ t('departmentUsage.usageProfile') }}
                        </p>
                        <dl class="space-y-1.5 text-sm">
                          <div class="flex items-center justify-between gap-3">
                            <dt class="text-gray-500 dark:text-dark-400">{{ t('departmentUsage.streamShare') }}</dt>
                            <dd class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ streamShare(department) }}</dd>
                          </div>
                          <div class="flex items-center justify-between gap-3">
                            <dt class="text-gray-500 dark:text-dark-400">{{ t('departmentUsage.avgLatency') }}</dt>
                            <dd class="tabular-nums font-medium" :class="durationClass(department.avg_duration_ms)">{{ formatLatency(department.avg_duration_ms) }}</dd>
                          </div>
                          <div class="flex items-center justify-between gap-3">
                            <dt class="text-gray-500 dark:text-dark-400">{{ t('departmentUsage.firstToken') }}</dt>
                            <dd class="tabular-nums font-medium" :class="firstTokenClass(department.avg_first_token_ms)">{{ formatLatency(department.avg_first_token_ms) }}</dd>
                          </div>
                          <div class="flex items-center justify-between gap-3">
                            <dt class="text-gray-500 dark:text-dark-400">{{ t('departmentUsage.images') }}</dt>
                            <dd class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ formatTokens(department.image_count) }}</dd>
                          </div>
                          <div class="flex items-center justify-between gap-3">
                            <dt class="text-gray-500 dark:text-dark-400">{{ t('departmentUsage.videos') }}</dt>
                            <dd class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ formatTokens(department.video_count) }}</dd>
                          </div>
                        </dl>
                      </div>

                      <div>
                        <p class="mb-2 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">
                          {{ t('departmentUsage.concentration') }}
                        </p>
                        <div class="mb-2 flex items-center gap-4 text-xs text-gray-500 dark:text-dark-400">
                          <span>{{ t('departmentUsage.topOneShare') }} <span class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ topShare(department, 1) }}</span></span>
                          <span>{{ t('departmentUsage.topNShare', { n: department.top_models.length }) }} <span class="tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ topShare(department, department.top_models.length) }}</span></span>
                        </div>
                        <p class="mb-2 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">
                          {{ t('departmentUsage.topModels') }}
                        </p>
                        <p v-if="department.top_models.length === 0" class="text-sm text-gray-400 dark:text-dark-500">
                          {{ t('departmentUsage.noModelData') }}
                        </p>
                        <div v-else class="space-y-2">
                          <div
                            v-for="model in department.top_models"
                            :key="model.model"
                            class="flex items-center justify-between gap-4 text-sm"
                          >
                            <span class="truncate text-gray-600 dark:text-dark-300">{{ model.model || t('departmentUsage.unassigned') }}</span>
                            <span class="flex-none tabular-nums font-medium text-gray-700 dark:text-dark-200">{{ formatTokens(model.total_tokens) }}</span>
                          </div>
                        </div>
                      </div>
                    </div>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>

        <div
          v-if="!loading && unusedDepartments.length > 0"
          class="border-t border-gray-200 px-5 py-4 dark:border-dark-700 sm:px-6"
        >
          <p class="text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500">
            {{ t('departmentUsage.unusedDepartmentsTitle', { count: unusedDepartments.length }) }}
          </p>
          <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">
            {{ t('departmentUsage.unusedDepartmentsHint') }}
          </p>
          <div class="mt-2 flex flex-wrap gap-2">
            <span
              v-for="department in unusedDepartments"
              :key="department.group_id"
              class="rounded-full bg-gray-100 px-2.5 py-1 text-xs text-gray-600 dark:bg-dark-800 dark:text-dark-300"
            >
              {{ department.group_name || t('departmentUsage.unassigned') }}
            </span>
          </div>
        </div>
      </section>

      <div class="grid gap-6 lg:grid-cols-2">
        <section class="space-y-5">
          <div class="card flex flex-col gap-4 p-6 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('departmentUsage.rankingTitle') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('departmentUsage.rankingDescription') }}</p>
            </div>
          </div>

          <DepartmentRankingChart
            :departments="sortedDepartments"
            :loading="loading"
            :empty-text="t('departmentUsage.noData')"
          />
        </section>

        <section class="space-y-5">
          <div class="card flex flex-col gap-4 p-6 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('departmentUsage.modelDistributionTitle') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('departmentUsage.modelDistributionDescription') }}</p>
            </div>
          </div>

          <DepartmentModelDistribution
            :rows="modelStats"
            :loading="modelsLoading"
            :empty-text="t('departmentUsage.modelDistributionEmpty')"
          />
        </section>
      </div>

      <section class="space-y-5">
        <div class="card flex flex-col gap-4 p-6 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('departmentUsage.clientSoftwareTitle') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('departmentUsage.clientSoftwareDescription') }}</p>
          </div>
          <label class="flex flex-none items-center gap-2 text-xs font-medium text-gray-500 dark:text-dark-400">
            {{ t('departmentUsage.reasoningScopeLabel') }}
            <select
              v-model.number="clientSoftwareGroupID"
              class="input w-40 py-1 text-xs"
              @change="changeClientSoftwareGroup"
            >
              <option :value="0">{{ t('departmentUsage.allDepartments') }}</option>
              <option v-for="department in sortedDepartments" :key="department.group_id" :value="department.group_id">
                {{ department.group_name || t('departmentUsage.unassigned') }}
              </option>
            </select>
          </label>
        </div>

        <DepartmentClientSoftwareTable
          :clients="clientSoftwareData"
          :loading="clientSoftwareLoading"
          :empty-text="t('departmentUsage.clientSoftwareEmpty')"
        />
      </section>

      <section class="space-y-3">
        <div v-if="heatmapSummary && !heatmapLoading" class="flex flex-wrap gap-2">
          <span class="inline-flex items-center gap-1.5 rounded-full border border-gray-200 bg-white px-3 py-1 text-xs text-gray-600 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-300">
            <Icon name="clock" size="sm" class="text-primary-500" />
            {{ t('departmentUsage.peakPeriod', {
              label: `${weekdayLabel(heatmapSummary.peakWeekday)} ${hourLabel(heatmapSummary.peakHour)}`,
              requests: formatNumber(heatmapSummary.peakRequests)
            }) }}
          </span>
          <span class="inline-flex items-center gap-1.5 rounded-full border border-gray-200 bg-white px-3 py-1 text-xs text-gray-600 dark:border-dark-700 dark:bg-dark-900 dark:text-dark-300">
            <Icon name="chart" size="sm" class="text-primary-500" />
            {{ t('departmentUsage.offHoursShare', { percent: heatmapSummary.offHoursShare.toFixed(1) }) }}
          </span>
        </div>

        <DepartmentUsageHeatmap
          :title="t('departmentUsage.heatmapTitle')"
          :description="t('departmentUsage.heatmapDescription')"
          :points="heatmapPoints"
          :loading="heatmapLoading"
          :empty-text="t('departmentUsage.heatmapEmpty')"
        />
      </section>

      <section ref="reasoningSection" class="space-y-5">
        <div class="card flex items-center justify-between gap-4 p-6">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('departmentUsage.reasoningTitle') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('departmentUsage.reasoningDescription') }}</p>
          </div>
          <button
            type="button"
            class="btn btn-secondary flex-none"
            :aria-expanded="reasoningExpanded"
            @click="toggleReasoning"
          >
            <Icon :name="reasoningExpanded ? 'chevronUp' : 'chevronDown'" size="sm" />
            <span class="hidden sm:inline">
              {{ reasoningExpanded ? t('departmentUsage.collapseReasoning') : t('departmentUsage.expandReasoning') }}
            </span>
          </button>
        </div>

        <template v-if="reasoningExpanded">
          <div class="card flex flex-col gap-4 p-6 lg:flex-row lg:items-center lg:justify-end">
            <div class="flex flex-wrap items-center gap-3">
              <label class="flex items-center gap-2 text-xs font-medium text-gray-500 dark:text-dark-400">
                {{ t('departmentUsage.reasoningScopeLabel') }}
                <select v-model.number="reasoningGroupID" class="input w-40 py-1 text-xs" @change="loadReasoning">
                  <option :value="0">{{ t('departmentUsage.reasoningGroupAll') }}</option>
                  <option v-for="department in sortedDepartments" :key="department.group_id" :value="department.group_id">
                    {{ department.group_name || t('departmentUsage.unassigned') }}
                  </option>
                </select>
              </label>
              <label class="flex items-center gap-2 text-xs font-medium text-gray-500 dark:text-dark-400">
                {{ t('departmentUsage.reasoningModelLabel') }}
                <select v-model="reasoningModel" class="input w-48 py-1 text-xs" @change="loadReasoning">
                  <option value="">{{ t('departmentUsage.reasoningModelAll') }}</option>
                  <option v-for="model in reasoningModelOptions" :key="model" :value="model">
                    {{ model }}
                  </option>
                </select>
              </label>
            </div>
          </div>

          <DepartmentReasoningEffortChart
            :title="reasoningDepartmentsTitle"
            :description="reasoningDepartmentsDescription"
            :rows="reasoningDepartments"
            :efforts="reasoningEfforts"
            label-mode="department"
            :loading="reasoningLoading"
            :empty-text="t('departmentUsage.reasoningEmpty')"
          />

          <div class="grid gap-6 lg:grid-cols-2">
            <DepartmentReasoningEffortChart
              :title="t('departmentUsage.reasoningModelsTitle')"
              :rows="reasoningModels"
              :efforts="reasoningEfforts"
              label-mode="model"
              :loading="reasoningLoading"
              :empty-text="t('departmentUsage.reasoningEmpty')"
            />
            <DepartmentReasoningEffortChart
              :title="t('departmentUsage.reasoningTrendTitle')"
              :rows="reasoningTrend"
              :efforts="reasoningEfforts"
              label-mode="bucket"
              :loading="reasoningLoading"
              :empty-text="t('departmentUsage.reasoningEmpty')"
            />
          </div>
        </template>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import DepartmentUsageHeatmap from '@/components/charts/DepartmentUsageHeatmap.vue'
import DepartmentReasoningEffortChart from '@/components/charts/DepartmentReasoningEffortChart.vue'
import DepartmentClientSoftwareTable from '@/components/charts/DepartmentClientSoftwareTable.vue'
import DepartmentInsights from '@/components/charts/DepartmentInsights.vue'
import DepartmentKpiCards from '@/components/charts/DepartmentKpiCards.vue'
import DepartmentModelDistribution from '@/components/charts/DepartmentModelDistribution.vue'
import DepartmentRankingChart from '@/components/charts/DepartmentRankingChart.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  getDepartmentClientSoftware,
  getDepartmentModelStats,
  getDepartmentReasoningEffort,
  getDepartmentUsage,
  getDepartmentUsageHeatmap,
  type ClientSoftwareStat,
  type DepartmentModelStat,
  type DepartmentReasoningEffortRow,
  type DepartmentUsageHeatmapPoint,
  type DepartmentUsageStat,
  type DepartmentUsageSummary,
  type UnusedDepartment
} from '@/api/usage'
import { formatCompactNumber, formatDateLocalInput, formatNumber } from '@/utils/format'
import { buildDepartmentInsights, type DepartmentInsight } from '@/utils/departmentInsights'
import { durationSeverity, firstTokenSeverity, LATENCY_TEXT_CLASSES, type LatencySeverity } from '@/utils/latencyHealth'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()

const today = new Date()
const startDate = ref(formatDateLocalInput(new Date(today.getTime() - 29 * 86400000)))
const endDate = ref(formatDateLocalInput(today))
const departments = ref<DepartmentUsageStat[]>([])
const summary = ref<DepartmentUsageSummary | null>(null)
const unusedDepartments = ref<UnusedDepartment[]>([])
const previousDepartments = ref<DepartmentUsageStat[]>([])
const previousSummary = ref<DepartmentUsageSummary | null>(null)
const previousRangeLabel = ref('')
const loading = ref(false)
let requestSequence = 0

const heatmapPoints = ref<DepartmentUsageHeatmapPoint[]>([])
const heatmapLoading = ref(false)
let heatmapSequence = 0

const clientSoftwareData = ref<ClientSoftwareStat[]>([])
// Kept separate from the table rows so the team insights always describe the
// whole team even while the table is filtered to one department.
const teamClientSoftwareData = ref<ClientSoftwareStat[]>([])
const clientSoftwareLoading = ref(false)
const clientSoftwareGroupID = ref(0)
let clientSoftwareSequence = 0

const modelStats = ref<DepartmentModelStat[]>([])
const modelsLoading = ref(false)
let modelsSequence = 0

// GPT reasoning-effort mix. The report always uses the effective effort within
// the GPT family. Shares are derived on the client from the tier request
// counts, so the same payload serves the department, model, and trend views.
const reasoningSection = ref<HTMLElement | null>(null)
const reasoningExpanded = ref(true)
const reasoningDepartments = ref<DepartmentReasoningEffortRow[]>([])
const reasoningModels = ref<DepartmentReasoningEffortRow[]>([])
const reasoningTrend = ref<DepartmentReasoningEffortRow[]>([])
const reasoningEfforts = ref<string[]>([])
const reasoningLoading = ref(false)
const reasoningGroupID = ref(0)
const reasoningModel = ref('')
let reasoningSequence = 0

const anyLoading = computed(() =>
  loading.value ||
  heatmapLoading.value ||
  clientSoftwareLoading.value ||
  modelsLoading.value ||
  reasoningLoading.value
)

// GPT model options are cached from every reasoning payload plus the
// model-distribution payload, so narrowing to one model never shrinks the
// dropdown options.
const reasoningKnownModels = ref<string[]>([])
function rememberReasoningModels(names: Array<string | undefined | null>) {
  const merged = new Set(reasoningKnownModels.value)
  for (const name of names) {
    if (name) merged.add(name)
  }
  reasoningKnownModels.value = [...merged].sort((a, b) => a.localeCompare(b))
}
const reasoningModelOptions = computed<string[]>(() => {
  const names = new Set<string>(reasoningKnownModels.value)
  for (const row of reasoningModels.value) {
    if (row.model) names.add(row.model)
  }
  for (const stat of modelStats.value) {
    if (stat.model && stat.model.toLowerCase().startsWith('gpt')) names.add(stat.model)
  }
  return [...names].sort((a, b) => a.localeCompare(b))
})

const reasoningDepartmentsTitle = computed(() =>
  reasoningModel.value
    ? t('departmentUsage.reasoningDepartmentsTitleForModel', { model: reasoningModel.value })
    : t('departmentUsage.reasoningDepartmentsTitle')
)

const reasoningDepartmentsDescription = computed(() =>
  reasoningModel.value
    ? t('departmentUsage.reasoningDepartmentsDescriptionForModel', { model: reasoningModel.value })
    : t('departmentUsage.reasoningDepartmentsDescription')
)

const rangeLabel = computed(() => `${startDate.value} - ${endDate.value}`)

type SortKey = 'name' | 'tokens' | 'requests' | 'perCapita'

const sortKey = ref<SortKey>('tokens')
const sortDirection = ref<'asc' | 'desc'>('desc')

const expandedGroups = ref<number[]>([])

function perCapitaValue(department: DepartmentUsageStat): number {
  if (!department.active_user_count || department.active_user_count <= 0) return 0
  return department.total_tokens / department.active_user_count
}

const sortedDepartments = computed(() =>
  [...departments.value]
    .sort((a, b) => {
      const factor = sortDirection.value === 'asc' ? 1 : -1
      switch (sortKey.value) {
        case 'tokens':
          return (a.total_tokens - b.total_tokens) * factor
        case 'requests':
          return (a.requests - b.requests) * factor
        case 'perCapita':
          return (perCapitaValue(a) - perCapitaValue(b)) * factor
        default:
          return (a.group_name || '').localeCompare(b.group_name || '', undefined, { sensitivity: 'base' }) * factor
      }
    })
    .map((department) => ({
      ...department,
      delta: deltaByGroup.value.get(department.group_id),
      rank: rankChange(department),
      flags: departmentFlags(department),
      sharePercent: teamSharePercent(department),
      intensityRatio: perCapitaRatio(department)
    }))
)

// Extra conclusions derived from the payload the page already loads.
const translate = (key: string, named?: Record<string, unknown>): string =>
  named ? t(key, named) : t(key)

const insights = computed<DepartmentInsight[]>(() =>
  buildDepartmentInsights({
    summary: summary.value,
    departments: departments.value,
    previousDepartments: previousDepartments.value,
    clients: teamClientSoftwareData.value,
    unusedDepartments: unusedDepartments.value,
    translate
  })
)

const coveragePercent = computed<number | null>(() => {
  const total = summary.value?.total_departments || 0
  if (total <= 0) return null
  const unused = Math.min(unusedDepartments.value.length, total)
  return Math.round(((total - unused) / total) * 100)
})

const coverageHint = computed<string | undefined>(() =>
  coveragePercent.value === null
    ? undefined
    : t('departmentUsage.coverageHint', { percent: coveragePercent.value })
)

const teamPerCapita = computed(() => {
  const current = summary.value
  if (!current || current.active_users <= 0) return 0
  return current.total_tokens / current.active_users
})

const teamCacheHitRate = computed(() => {
  let read = 0
  let effective = 0
  for (const department of departments.value) {
    read += department.cache_read_tokens || 0
    effective +=
      (department.input_tokens || 0) +
      (department.cache_creation_tokens || 0) +
      (department.cache_read_tokens || 0)
  }
  return effective > 0 ? read / effective : 0
})

const teamTokensPerRequest = computed(() => {
  let requests = 0
  let tokens = 0
  for (const department of departments.value) {
    requests += department.requests || 0
    tokens += department.total_tokens || 0
  }
  return requests > 0 ? tokens / requests : 0
})

function perCapitaRatio(department: DepartmentUsageStat): number | null {
  const value = perCapitaValue(department)
  if (value <= 0 || teamPerCapita.value <= 0) return null
  return value / teamPerCapita.value
}

function teamSharePercent(department: DepartmentUsageStat): number {
  const total = summary.value?.total_tokens || 0
  if (total <= 0) return 0
  return ((department.total_tokens || 0) / total) * 100
}

function teamShareText(department: DepartmentUsageStat): string {
  return t('departmentUsage.shareOfTeam', {
    percent: teamSharePercent(department).toFixed(1)
  })
}

function rankByTokens(list: DepartmentUsageStat[]): Map<number, number> {
  const sorted = [...list]
    .filter((department) => department.total_tokens > 0)
    .sort((a, b) => b.total_tokens - a.total_tokens || a.group_id - b.group_id)
  return new Map(sorted.map((department, index) => [department.group_id, index + 1]))
}

const currentTokenRanks = computed(() => rankByTokens(departments.value))
const previousTokenRanks = computed(() => rankByTokens(previousDepartments.value))

interface RankChange {
  direction: 'up' | 'down' | 'same'
  count: number
}

function rankChange(department: DepartmentUsageStat): RankChange | null {
  const current = currentTokenRanks.value.get(department.group_id)
  const previous = previousTokenRanks.value.get(department.group_id)
  if (!current || !previous) return null
  const diff = previous - current
  if (diff === 0) return { direction: 'same', count: 0 }
  return { direction: diff > 0 ? 'up' : 'down', count: Math.abs(diff) }
}

function rankChangeText(change: RankChange): string {
  if (change.direction === 'up') return `↑${change.count}`
  if (change.direction === 'down') return `↓${change.count}`
  return '–'
}

function rankChangeClass(change: RankChange): string {
  if (change.direction === 'up') return 'text-emerald-600 dark:text-emerald-400'
  if (change.direction === 'down') return 'text-red-500 dark:text-red-400'
  return 'text-gray-400 dark:text-dark-500'
}

function rankChangeTitle(change: RankChange): string {
  if (change.direction === 'up') return t('departmentUsage.rankUp', { count: change.count })
  if (change.direction === 'down') return t('departmentUsage.rankDown', { count: change.count })
  return t('departmentUsage.rankSame')
}

type DepartmentFlagKey = 'cacheLow' | 'heavyContext' | 'slowFirstToken'

const CACHE_FLAG_MIN_EFFECTIVE_TOKENS = 1_000_000
const HEAVY_CONTEXT_MIN_TOKENS_PER_REQUEST = 20_000
const FLAG_MIN_REQUESTS = 5

// Health markers that turn the table into an optimization checklist. Cache
// efficiency is judged against the team average, request size against twice the
// team average, and first-token latency against the shared latency-health tiers.
function departmentFlags(department: DepartmentUsageStat): DepartmentFlagKey[] {
  const flags: DepartmentFlagKey[] = []
  const effective =
    (department.input_tokens || 0) +
    (department.cache_creation_tokens || 0) +
    (department.cache_read_tokens || 0)
  if (effective >= CACHE_FLAG_MIN_EFFECTIVE_TOKENS && teamCacheHitRate.value > 0) {
    const rate = (department.cache_read_tokens || 0) / effective
    if (rate < teamCacheHitRate.value - 0.05) flags.push('cacheLow')
  }
  if (department.requests > 0 && teamTokensPerRequest.value > 0) {
    const perRequest = department.total_tokens / department.requests
    if (perRequest >= Math.max(HEAVY_CONTEXT_MIN_TOKENS_PER_REQUEST, teamTokensPerRequest.value * 2)) {
      flags.push('heavyContext')
    }
  }
  const firstToken = firstTokenSeverity(department.avg_first_token_ms || 0)
  if (department.requests >= FLAG_MIN_REQUESTS && (firstToken === 'slow' || firstToken === 'critical')) {
    flags.push('slowFirstToken')
  }
  return flags
}

function flagLabel(flag: DepartmentFlagKey): string {
  switch (flag) {
    case 'cacheLow':
      return t('departmentUsage.flagCacheLow')
    case 'heavyContext':
      return t('departmentUsage.flagHeavyContext')
    default:
      return t('departmentUsage.flagSlowFirstToken')
  }
}

function flagTitle(flags: DepartmentFlagKey[]): string {
  return flags.map(flagLabel).join(' · ')
}

function latencyClass(value: number, severity: (ms: number) => LatencySeverity): string {
  if (!value || value <= 0) return 'text-gray-700 dark:text-dark-200'
  return LATENCY_TEXT_CLASSES[severity(value)]
}

function durationClass(value: number): string {
  return latencyClass(value, durationSeverity)
}

function firstTokenClass(value: number): string {
  return latencyClass(value, firstTokenSeverity)
}

interface HeatmapSummary {
  peakWeekday: number
  peakHour: number
  peakRequests: number
  offHoursShare: number
}

// Work hours are Monday-Friday 09:00-18:59 in the requester's timezone, which is
// the same timezone the heatmap buckets are resolved in.
function isOffHours(point: DepartmentUsageHeatmapPoint): boolean {
  return point.weekday === 0 || point.weekday === 6 || point.hour < 9 || point.hour >= 19
}

const heatmapSummary = computed<HeatmapSummary | null>(() => {
  const points = heatmapPoints.value
  if (points.length === 0) return null
  let total = 0
  let offHours = 0
  let peak: DepartmentUsageHeatmapPoint | null = null
  for (const point of points) {
    total += point.requests || 0
    if (isOffHours(point)) offHours += point.requests || 0
    if (!peak || point.requests > peak.requests) peak = point
  }
  if (!peak || total <= 0 || peak.requests <= 0) return null
  return {
    peakWeekday: peak.weekday,
    peakHour: peak.hour,
    peakRequests: peak.requests,
    offHoursShare: (offHours / total) * 100
  }
})

function weekdayLabel(weekday: number): string {
  switch (weekday) {
    case 1:
      return t('departmentUsage.weekdayMon')
    case 2:
      return t('departmentUsage.weekdayTue')
    case 3:
      return t('departmentUsage.weekdayWed')
    case 4:
      return t('departmentUsage.weekdayThu')
    case 5:
      return t('departmentUsage.weekdayFri')
    case 6:
      return t('departmentUsage.weekdaySat')
    default:
      return t('departmentUsage.weekdaySun')
  }
}

function hourLabel(hour: number): string {
  return `${String(hour).padStart(2, '0')}:00`
}

interface DeltaInfo {
  text: string
  tone: 'up' | 'down' | 'flat'
}

// Compare against the immediately preceding window of the same length. Any
// failure is non-fatal: the current period simply renders without a delta.
const deltaByGroup = computed(() => {
  const map = new Map<number, DeltaInfo>()
  const previousById = new Map(previousDepartments.value.map((department) => [department.group_id, department]))
  for (const department of departments.value) {
    const previous = previousById.get(department.group_id)
    if (!previous || previous.total_tokens <= 0) {
      map.set(department.group_id, { text: `— ${t('departmentUsage.noBaseline')}`, tone: 'flat' })
      continue
    }
    const change = ((department.total_tokens - previous.total_tokens) / previous.total_tokens) * 100
    if (Math.abs(change) < 0.05) {
      map.set(department.group_id, { text: '±0.0%', tone: 'flat' })
      continue
    }
    map.set(department.group_id, {
      text: `${change > 0 ? '+' : ''}${change.toFixed(1)}%`,
      tone: change > 0 ? 'up' : 'down'
    })
  }
  return map
})

function formatTokens(value: number): string {
  return formatNumber(value || 0)
}

function formatLatency(value: number): string {
  if (!value || value <= 0) return '—'
  if (value < 1000) return `${Math.round(value)} ms`
  return `${(value / 1000).toFixed(2)} s`
}

function perCapitaTokens(department: DepartmentUsageStat): string {
  const value = perCapitaValue(department)
  if (value <= 0) return '—'
  return formatCompactNumber(value)
}

function cacheHitRate(department: DepartmentUsageStat): string {
  const read = department.cache_read_tokens || 0
  const denominator = read + (department.cache_creation_tokens || 0) + (department.input_tokens || 0)
  if (denominator <= 0) return '—'
  return `${((read / denominator) * 100).toFixed(1)}%`
}

function streamShare(department: DepartmentUsageStat): string {
  if (!department.requests) return '—'
  return `${(((department.stream_requests || 0) / department.requests) * 100).toFixed(1)}%`
}

function topShare(department: DepartmentUsageStat, count: number): string {
  if (count <= 0 || !department.total_tokens) return '—'
  const top = (department.top_models || [])
    .slice(0, count)
    .reduce((sum, model) => sum + (model.total_tokens || 0), 0)
  return `${((top / department.total_tokens) * 100).toFixed(1)}%`
}

function deltaClass(delta: DeltaInfo): string {
  if (delta.tone === 'up') return 'text-emerald-600 dark:text-emerald-400'
  if (delta.tone === 'down') return 'text-red-500 dark:text-red-400'
  return 'text-gray-400 dark:text-dark-500'
}

function toggleSort(key: SortKey) {
  if (sortKey.value === key) {
    sortDirection.value = sortDirection.value === 'asc' ? 'desc' : 'asc'
    return
  }
  sortKey.value = key
  sortDirection.value = key === 'name' ? 'asc' : 'desc'
}

function isExpanded(groupId: number): boolean {
  return expandedGroups.value.includes(groupId)
}

function toggleGroup(groupId: number) {
  expandedGroups.value = isExpanded(groupId)
    ? expandedGroups.value.filter((id) => id !== groupId)
    : [...expandedGroups.value, groupId]
}

function toggleReasoning() {
  reasoningExpanded.value = !reasoningExpanded.value
}

function shiftDate(value: string, days: number): string {
  const date = new Date(`${value}T00:00:00`)
  date.setDate(date.getDate() + days)
  return formatDateLocalInput(date)
}

function rangeLength(start: string, end: string): number {
  const startMs = new Date(`${start}T00:00:00`).getTime()
  const endMs = new Date(`${end}T00:00:00`).getTime()
  return Math.max(1, Math.round((endMs - startMs) / 86400000) + 1)
}

function resolvedTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || ''
  } catch {
    return ''
  }
}

async function loadDepartments() {
  const sequence = ++requestSequence
  loading.value = true
  const length = rangeLength(startDate.value, endDate.value)
  const previousEnd = shiftDate(startDate.value, -1)
  const previousStart = shiftDate(previousEnd, -(length - 1))
  try {
    const [current, previous] = await Promise.all([
      getDepartmentUsage({
        start_date: startDate.value,
        end_date: endDate.value
      }),
      getDepartmentUsage({
        start_date: previousStart,
        end_date: previousEnd
      }).catch(() => null)
    ])
    if (sequence === requestSequence) {
      departments.value = current.departments || []
      summary.value = current.summary || null
      unusedDepartments.value = current.unused_departments || []
      previousDepartments.value = previous?.departments || []
      previousSummary.value = previous?.summary || null
      previousRangeLabel.value = `${previousStart} - ${previousEnd}`
      // The department scope of the client software table must follow the
      // departments that exist in the current range.
      if (
        clientSoftwareGroupID.value !== 0 &&
        !departments.value.some((department) => department.group_id === clientSoftwareGroupID.value)
      ) {
        clientSoftwareGroupID.value = 0
        void loadClientSoftware()
      }
    }
  } catch (error) {
    if (sequence === requestSequence) {
      departments.value = []
      summary.value = null
      unusedDepartments.value = []
      previousDepartments.value = []
      previousSummary.value = null
      appStore.showError(t('departmentUsage.loadFailed'))
      console.error('Failed to load department usage:', error)
    }
  } finally {
    if (sequence === requestSequence) loading.value = false
  }
}

async function loadHeatmap() {
  const sequence = ++heatmapSequence
  heatmapLoading.value = true
  try {
    const response = await getDepartmentUsageHeatmap({
      start_date: startDate.value,
      end_date: endDate.value,
      timezone: resolvedTimezone()
    })
    if (sequence === heatmapSequence) {
      heatmapPoints.value = response.points || []
    }
  } catch (error) {
    if (sequence === heatmapSequence) {
      heatmapPoints.value = []
      appStore.showError(t('departmentUsage.reportLoadFailed'))
      console.error('Failed to load department usage heatmap:', error)
    }
  } finally {
    if (sequence === heatmapSequence) heatmapLoading.value = false
  }
}

async function loadClientSoftware() {
  const sequence = ++clientSoftwareSequence
  clientSoftwareLoading.value = true
  try {
    const response = await getDepartmentClientSoftware({
      start_date: startDate.value,
      end_date: endDate.value,
      group_id: clientSoftwareGroupID.value || undefined,
      limit: 10
    })
    if (sequence === clientSoftwareSequence) {
      clientSoftwareData.value = response.clients || []
      if (clientSoftwareGroupID.value === 0) {
        teamClientSoftwareData.value = response.clients || []
      }
    }
  } catch (error) {
    if (sequence === clientSoftwareSequence) {
      clientSoftwareData.value = []
      if (clientSoftwareGroupID.value === 0) teamClientSoftwareData.value = []
      appStore.showError(t('departmentUsage.clientSoftwareLoadFailed'))
      console.error('Failed to load department client software:', error)
    }
  } finally {
    if (sequence === clientSoftwareSequence) clientSoftwareLoading.value = false
  }
}

function changeClientSoftwareGroup() {
  void loadClientSoftware()
}

async function loadModels() {
  const sequence = ++modelsSequence
  modelsLoading.value = true
  try {
    const response = await getDepartmentModelStats({
      start_date: startDate.value,
      end_date: endDate.value,
      limit: 8
    })
    if (sequence === modelsSequence) {
      modelStats.value = response.models || []
      rememberReasoningModels(modelStats.value.map((stat) => stat.model))
    }
  } catch (error) {
    if (sequence === modelsSequence) {
      modelStats.value = []
      appStore.showError(t('departmentUsage.modelDistributionLoadFailed'))
      console.error('Failed to load department model stats:', error)
    }
  } finally {
    if (sequence === modelsSequence) modelsLoading.value = false
  }
}

async function loadReasoning() {
  if (!reasoningExpanded.value) return
  const sequence = ++reasoningSequence
  reasoningLoading.value = true
  try {
    const response = await getDepartmentReasoningEffort({
      start_date: startDate.value,
      end_date: endDate.value,
      group_id: reasoningGroupID.value || undefined,
      model: reasoningModel.value || undefined,
      granularity: 'day'
    })
    if (sequence === reasoningSequence) {
      reasoningDepartments.value = response.departments || []
      reasoningModels.value = response.models || []
      reasoningTrend.value = response.trend || []
      reasoningEfforts.value = response.efforts || []
      rememberReasoningModels(reasoningModels.value.map((row) => row.model))
    }
  } catch (error) {
    if (sequence === reasoningSequence) {
      reasoningDepartments.value = []
      reasoningModels.value = []
      reasoningTrend.value = []
      reasoningEfforts.value = []
      appStore.showError(t('departmentUsage.reasoningLoadFailed'))
      console.error('Failed to load department reasoning effort:', error)
    }
  } finally {
    if (sequence === reasoningSequence) reasoningLoading.value = false
  }
}

async function focusReasoningDepartment(groupID: number) {
  reasoningGroupID.value = groupID
  if (!reasoningExpanded.value) reasoningExpanded.value = true
  await nextTick()
  void loadReasoning()
  reasoningSection.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function loadUsage() {
  void loadDepartments()
  void loadHeatmap()
  void loadClientSoftware()
  void loadModels()
  void loadReasoning()
}

onMounted(loadUsage)
</script>

<style scoped>
.skeleton {
  border-radius: 0.5rem;
  background: linear-gradient(90deg, #e5e7eb 25%, #f3f4f6 50%, #e5e7eb 75%);
  background-size: 200% 100%;
  animation: department-usage-shimmer 1.8s ease-in-out infinite;
}

:global(.dark) .skeleton {
  background: linear-gradient(90deg, #334155 25%, #1e293b 50%, #334155 75%);
  background-size: 200% 100%;
}

@keyframes department-usage-shimmer {
  0% { background-position: -200% 0; }
  100% { background-position: 200% 0; }
}
</style>
