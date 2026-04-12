<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
      <!-- Header -->
      <div class="mb-6">
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('modelSquare.title') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('modelSquare.description') }}</p>
      </div>

      <!-- Filters Bar -->
      <div class="mb-6 flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
        <!-- Search -->
        <div class="relative w-full sm:w-64">
          <input
            v-model="searchQuery"
            type="text"
            :placeholder="t('modelSquare.searchPlaceholder')"
            class="w-full rounded-lg border border-gray-300 bg-white px-4 py-2 pl-10 text-sm text-gray-900 placeholder-gray-400 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-800 dark:text-white dark:placeholder-gray-500 dark:focus:border-primary-500"
          />
          <svg class="absolute left-3 top-2.5 h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" />
          </svg>
        </div>

        <!-- Group Filter -->
        <Select
          :model-value="selectedGroupId"
          :options="groupFilterOptions"
          class="w-full sm:w-44"
          @update:model-value="onGroupFilterChange"
        />

        <!-- Provider Filter -->
        <Select
          :model-value="selectedProvider"
          :options="providerFilterOptions"
          class="w-full sm:w-40"
          @update:model-value="onProviderFilterChange"
        />

        <!-- Type Filter -->
        <Select
          :model-value="selectedMode"
          :options="modeFilterOptions"
          class="w-full sm:w-36"
          @update:model-value="onModeFilterChange"
        />

        <!-- Sort -->
        <Select
          :model-value="sortBy"
          :options="sortOptions"
          class="w-full sm:w-40"
          @update:model-value="onSortChange"
        />

        <!-- Spacer -->
        <div class="hidden flex-1 sm:block"></div>

        <!-- View Toggle + Model Count -->
        <div class="flex items-center gap-3">
          <span class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('modelSquare.modelCount', { count: filteredModels.length }) }}
          </span>
          <div class="flex rounded-lg border border-gray-300 dark:border-dark-600">
            <button
              @click="viewMode = 'grid'"
              :class="[
                'rounded-l-lg px-3 py-1.5 text-sm transition-colors',
                viewMode === 'grid'
                  ? 'bg-primary-500 text-white'
                  : 'bg-white text-gray-600 hover:bg-gray-50 dark:bg-dark-800 dark:text-gray-400 dark:hover:bg-dark-700'
              ]"
              :title="t('modelSquare.gridView')"
            >
              <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6A2.25 2.25 0 016 3.75h2.25A2.25 2.25 0 0110.5 6v2.25a2.25 2.25 0 01-2.25 2.25H6a2.25 2.25 0 01-2.25-2.25V6zM3.75 15.75A2.25 2.25 0 016 13.5h2.25a2.25 2.25 0 012.25 2.25V18a2.25 2.25 0 01-2.25 2.25H6A2.25 2.25 0 013.75 18v-2.25zM13.5 6a2.25 2.25 0 012.25-2.25H18A2.25 2.25 0 0120.25 6v2.25A2.25 2.25 0 0118 10.5h-2.25a2.25 2.25 0 01-2.25-2.25V6zM13.5 15.75a2.25 2.25 0 012.25-2.25H18a2.25 2.25 0 012.25 2.25V18A2.25 2.25 0 0118 20.25h-2.25A2.25 2.25 0 0113.5 18v-2.25z" />
              </svg>
            </button>
            <button
              @click="viewMode = 'table'"
              :class="[
                'rounded-r-lg px-3 py-1.5 text-sm transition-colors',
                viewMode === 'table'
                  ? 'bg-primary-500 text-white'
                  : 'bg-white text-gray-600 hover:bg-gray-50 dark:bg-dark-800 dark:text-gray-400 dark:hover:bg-dark-700'
              ]"
              :title="t('modelSquare.tableView')"
            >
              <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 5.25h16.5m-16.5 4.5h16.5m-16.5 4.5h16.5m-16.5 4.5h16.5" />
              </svg>
            </button>
          </div>
        </div>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="flex items-center justify-center py-20">
        <svg class="h-8 w-8 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
      </div>

      <!-- Empty State -->
      <div v-else-if="flatModels.length === 0" class="flex flex-col items-center justify-center py-20 text-gray-500 dark:text-gray-400">
        <svg class="mb-4 h-16 w-16 text-gray-300 dark:text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1">
          <path stroke-linecap="round" stroke-linejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5m8.25 3v6.75m0 0l-3-3m3 3l3-3M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" />
        </svg>
        <p class="text-lg font-medium">{{ t('modelSquare.noModels') }}</p>
      </div>

      <!-- No Results -->
      <div v-else-if="filteredModels.length === 0" class="flex flex-col items-center justify-center py-20 text-gray-500 dark:text-gray-400">
        <svg class="mb-4 h-16 w-16 text-gray-300 dark:text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1">
          <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" />
        </svg>
        <p class="text-lg font-medium">{{ t('modelSquare.noResults') }}</p>
      </div>

      <!-- Grid View -->
      <div v-else-if="viewMode === 'grid'" class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <div
          v-for="(model, index) in paginatedModels"
          :key="`${model.group_id}-${model.model_name}-${index}`"
          class="group cursor-pointer rounded-xl border border-gray-200 bg-white p-5 shadow-sm transition-all duration-200 hover:border-primary-300 hover:shadow-md dark:border-dark-600 dark:bg-dark-800 dark:hover:border-primary-600"
          @click="copyModelName(model.model_name)"
        >
          <!-- Card Header -->
          <div class="mb-3 flex items-start justify-between">
            <div class="min-w-0 flex-1">
              <h3 class="truncate text-sm font-semibold text-gray-900 dark:text-white" :title="model.model_name">
                {{ model.model_name }}
              </h3>
              <div class="mt-1 flex items-center gap-2">
                <span :class="providerBadgeClass(model.provider)" class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium">
                  {{ model.provider || t('modelSquare.unknownProvider') }}
                </span>
                <span class="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-400">
                  {{ translateMode(model.mode) }}
                </span>
              </div>
            </div>
            <span class="inline-flex items-center rounded-full bg-green-50 px-2 py-0.5 text-xs font-medium text-green-700 dark:bg-green-900/20 dark:text-green-400">
              {{ t('modelSquare.available') }}
            </span>
          </div>

          <!-- Pricing -->
          <div v-if="model.has_pricing" class="space-y-2">
            <div class="flex items-center justify-between text-sm">
              <span class="text-gray-500 dark:text-gray-400">{{ t('modelSquare.columns.inputPrice') }}</span>
              <span class="font-mono font-medium text-gray-900 dark:text-white">${{ formatPrice(model.input_price_per_million) }}</span>
            </div>
            <div class="flex items-center justify-between text-sm">
              <span class="text-gray-500 dark:text-gray-400">{{ t('modelSquare.columns.outputPrice') }}</span>
              <span class="font-mono font-medium text-gray-900 dark:text-white">${{ formatPrice(model.output_price_per_million) }}</span>
            </div>
            <div v-if="model.cache_read_price_per_million > 0" class="flex items-center justify-between text-sm">
              <span class="text-gray-500 dark:text-gray-400">{{ t('modelSquare.columns.cacheReadPrice') }}</span>
              <span class="font-mono font-medium text-gray-900 dark:text-white">${{ formatPrice(model.cache_read_price_per_million) }}</span>
            </div>
            <div v-if="model.cache_write_price_per_million > 0" class="flex items-center justify-between text-sm">
              <span class="text-gray-500 dark:text-gray-400">{{ t('modelSquare.columns.cacheWritePrice') }}</span>
              <span class="font-mono font-medium text-gray-900 dark:text-white">${{ formatPrice(model.cache_write_price_per_million) }}</span>
            </div>
          </div>
          <div v-else class="py-3 text-center text-sm text-gray-400 dark:text-gray-500">
            {{ t('modelSquare.noPricing') }}
          </div>

          <!-- Card Footer -->
          <div class="mt-3 flex items-center justify-between border-t border-gray-100 pt-3 dark:border-dark-700">
            <span class="truncate text-xs text-gray-400 dark:text-gray-500" :title="model.group_name">
              {{ model.group_name }}
            </span>
            <span v-if="model.rate_multiplier !== 1" class="text-xs text-gray-400 dark:text-gray-500">
              {{ t('modelSquare.rateMultiplier') }}: {{ model.rate_multiplier }}x
            </span>
          </div>
        </div>
      </div>

      <!-- Table View -->
      <div v-else class="overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-600">
        <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-600">
          <thead class="bg-gray-50 dark:bg-dark-800">
            <tr>
              <th class="table-header-cell">{{ t('modelSquare.columns.provider') }}</th>
              <th class="table-header-cell">{{ t('modelSquare.columns.modelName') }}</th>
              <th class="table-header-cell text-right">{{ t('modelSquare.columns.inputPrice') }}</th>
              <th class="table-header-cell text-right">{{ t('modelSquare.columns.outputPrice') }}</th>
              <th class="table-header-cell text-right">{{ t('modelSquare.columns.cacheReadPrice') }}</th>
              <th class="table-header-cell text-right">{{ t('modelSquare.columns.cacheWritePrice') }}</th>
              <th class="table-header-cell">{{ t('modelSquare.columns.mode') }}</th>
              <th class="table-header-cell">{{ t('modelSquare.columns.group') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
            <tr
              v-for="(model, index) in paginatedModels"
              :key="`table-${model.group_id}-${model.model_name}-${index}`"
              class="cursor-pointer transition-colors hover:bg-gray-50 dark:hover:bg-dark-800"
              @click="copyModelName(model.model_name)"
            >
              <td class="table-cell">
                <span :class="providerBadgeClass(model.provider)" class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium">
                  {{ model.provider || t('modelSquare.unknownProvider') }}
                </span>
              </td>
              <td class="table-cell font-medium text-gray-900 dark:text-white">
                <span class="font-mono text-sm">{{ model.model_name }}</span>
              </td>
              <td class="table-cell text-right font-mono text-sm">
                <template v-if="model.has_pricing">${{ formatPrice(model.input_price_per_million) }}</template>
                <span v-else class="text-gray-400">-</span>
              </td>
              <td class="table-cell text-right font-mono text-sm">
                <template v-if="model.has_pricing">${{ formatPrice(model.output_price_per_million) }}</template>
                <span v-else class="text-gray-400">-</span>
              </td>
              <td class="table-cell text-right font-mono text-sm">
                <template v-if="model.has_pricing && model.cache_read_price_per_million > 0">${{ formatPrice(model.cache_read_price_per_million) }}</template>
                <span v-else class="text-gray-400">-</span>
              </td>
              <td class="table-cell text-right font-mono text-sm">
                <template v-if="model.has_pricing && model.cache_write_price_per_million > 0">${{ formatPrice(model.cache_write_price_per_million) }}</template>
                <span v-else class="text-gray-400">-</span>
              </td>
              <td class="table-cell">
                <span class="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-400">
                  {{ translateMode(model.mode) }}
                </span>
              </td>
              <td class="table-cell text-sm text-gray-500 dark:text-gray-400">
                {{ model.group_name }}
                <span v-if="model.rate_multiplier !== 1" class="ml-1 text-xs text-gray-400">({{ model.rate_multiplier }}x)</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Pagination -->
      <div v-if="filteredModels.length > pageSize" class="mt-6">
        <Pagination
          :total="filteredModels.length"
          :page="currentPage"
          :page-size="pageSize"
          :page-size-options="[24, 48, 96]"
          @update:page="currentPage = $event"
          @update:page-size="onPageSizeChange"
        />
      </div>

      <!-- Toast -->
      <Transition name="toast">
        <div
          v-if="showToast"
          class="fixed bottom-6 right-6 z-50 rounded-lg bg-gray-900 px-4 py-2 text-sm text-white shadow-lg dark:bg-gray-100 dark:text-gray-900"
        >
          {{ t('modelSquare.copied') }}
        </div>
      </Transition>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Select from '@/components/common/Select.vue'
import Pagination from '@/components/common/Pagination.vue'
import { modelsAPI, type GroupModels, type ModelInfo } from '@/api'

const { t } = useI18n()

// --- State ---
const loading = ref(true)
const groupsData = ref<GroupModels[]>([])
const searchQuery = ref('')
const selectedGroupId = ref<string | number>('all')
const selectedProvider = ref<string | number>('all')
const selectedMode = ref<string | number>('all')
const sortBy = ref<string | number>('name')
const viewMode = ref<'grid' | 'table'>('grid')
const currentPage = ref(1)
const pageSize = ref(24)
const showToast = ref(false)

// --- Flat model list (combining group info with each model) ---
interface FlatModel extends ModelInfo {
  group_id: number
  group_name: string
  platform: string
  rate_multiplier: number
}

const flatModels = computed<FlatModel[]>(() => {
  const result: FlatModel[] = []
  for (const group of groupsData.value) {
    for (const model of group.models) {
      result.push({
        ...model,
        group_id: group.group_id,
        group_name: group.group_name,
        platform: group.platform,
        rate_multiplier: group.rate_multiplier,
      })
    }
  }
  return result
})

// --- Filter Options ---
const groupFilterOptions = computed(() => {
  const options: { value: string | number; label: string }[] = [
    { value: 'all', label: t('modelSquare.allGroups') }
  ]
  for (const group of groupsData.value) {
    const rateLabel = group.rate_multiplier !== 1
      ? ` (${group.rate_multiplier}x)`
      : ''
    options.push({ value: group.group_id, label: group.group_name + rateLabel })
  }
  return options
})

const providerFilterOptions = computed(() => {
  const providers = new Set<string>()
  for (const m of flatModels.value) {
    if (m.provider) providers.add(m.provider)
  }
  const options: { value: string | number; label: string }[] = [
    { value: 'all', label: t('modelSquare.allProviders') }
  ]
  for (const p of Array.from(providers).sort()) {
    options.push({ value: p, label: p })
  }
  return options
})

const modeFilterOptions = computed(() => {
  const modes = new Set<string>()
  for (const m of flatModels.value) {
    if (m.mode) modes.add(m.mode)
  }
  const options: { value: string | number; label: string }[] = [
    { value: 'all', label: t('modelSquare.allTypes') }
  ]
  for (const mode of Array.from(modes).sort()) {
    options.push({ value: mode, label: translateMode(mode) })
  }
  return options
})

const sortOptions = computed(() => [
  { value: 'name', label: t('modelSquare.sortByName') },
  { value: 'input_price', label: t('modelSquare.sortByInputPrice') },
  { value: 'output_price', label: t('modelSquare.sortByOutputPrice') },
])

// --- Filtered & Sorted Models ---
const filteredModels = computed<FlatModel[]>(() => {
  let result = flatModels.value

  // Group filter
  if (selectedGroupId.value !== 'all') {
    result = result.filter(m => m.group_id === Number(selectedGroupId.value))
  }

  // Provider filter
  if (selectedProvider.value !== 'all') {
    result = result.filter(m => m.provider === selectedProvider.value)
  }

  // Mode filter
  if (selectedMode.value !== 'all') {
    result = result.filter(m => m.mode === selectedMode.value)
  }

  // Search filter
  if (searchQuery.value.trim()) {
    const query = searchQuery.value.trim().toLowerCase()
    result = result.filter(m =>
      m.model_name.toLowerCase().includes(query) ||
      (m.provider && m.provider.toLowerCase().includes(query))
    )
  }

  // Sort
  const sorted = [...result]
  switch (sortBy.value) {
    case 'name':
      sorted.sort((a, b) => a.model_name.localeCompare(b.model_name))
      break
    case 'input_price':
      sorted.sort((a, b) => a.input_price_per_million - b.input_price_per_million)
      break
    case 'output_price':
      sorted.sort((a, b) => a.output_price_per_million - b.output_price_per_million)
      break
  }

  return sorted
})

// --- Pagination ---
const paginatedModels = computed<FlatModel[]>(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredModels.value.slice(start, start + pageSize.value)
})

// Reset page when filters change
watch([searchQuery, selectedGroupId, selectedProvider, selectedMode, sortBy], () => {
  currentPage.value = 1
})

// --- Data Loading ---
async function loadData() {
  loading.value = true
  try {
    groupsData.value = await modelsAPI.getGroupedModels()
  } catch (error) {
    console.error('Failed to load model square data:', error)
    groupsData.value = []
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadData()
})

// --- Event Handlers ---
function onGroupFilterChange(value: string | number | boolean | null) {
  selectedGroupId.value = value as string | number
}

function onProviderFilterChange(value: string | number | boolean | null) {
  selectedProvider.value = value as string | number
}

function onModeFilterChange(value: string | number | boolean | null) {
  selectedMode.value = value as string | number
}

function onSortChange(value: string | number | boolean | null) {
  sortBy.value = value as string | number
}

function onPageSizeChange(size: number) {
  pageSize.value = size
  currentPage.value = 1
}

// --- Helpers ---
function formatPrice(price: number): string {
  if (price === 0) return '0'
  if (price < 0.001) return price.toFixed(4)
  if (price < 0.1) return price.toFixed(3)
  return price.toFixed(2)
}

function translateMode(mode: string): string {
  const key = `modelSquare.${mode}` as string
  const translated = t(key)
  // If translation key not found, vue-i18n returns the key itself
  if (translated === key) {
    return mode || t('modelSquare.unknownProvider')
  }
  return translated
}

function providerBadgeClass(provider: string): string {
  const base = 'inline-flex items-center'
  switch (provider?.toLowerCase()) {
    case 'anthropic':
      return `${base} bg-orange-50 text-orange-700 dark:bg-orange-900/20 dark:text-orange-400`
    case 'openai':
      return `${base} bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400`
    case 'google':
    case 'vertex_ai':
      return `${base} bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-400`
    default:
      return `${base} bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-400`
  }
}

let toastTimer: ReturnType<typeof setTimeout> | null = null

async function copyModelName(name: string) {
  try {
    if (navigator.clipboard) {
      await navigator.clipboard.writeText(name)
    } else {
      // Fallback for HTTP environments
      const textarea = document.createElement('textarea')
      textarea.value = name
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }

    showToast.value = true
    if (toastTimer) clearTimeout(toastTimer)
    toastTimer = setTimeout(() => {
      showToast.value = false
    }, 2000)
  } catch {
    // Silently fail
  }
}
</script>

<style scoped>
.table-header-cell {
  @apply px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400;
}

.table-cell {
  @apply whitespace-nowrap px-4 py-3 text-sm text-gray-700 dark:text-gray-300;
}

.toast-enter-active {
  transition: all 0.3s ease-out;
}

.toast-leave-active {
  transition: all 0.2s ease-in;
}

.toast-enter-from {
  opacity: 0;
  transform: translateY(10px);
}

.toast-leave-to {
  opacity: 0;
  transform: translateY(10px);
}
</style>
