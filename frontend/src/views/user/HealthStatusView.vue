<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Header -->
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 class="text-xl font-bold text-gray-900 dark:text-white">
            {{ t('healthStatus.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('healthStatus.description') }}
          </p>
        </div>
        <div class="flex items-center gap-2">
          <!-- Search -->
          <input
            v-model="searchQuery"
            type="text"
            :placeholder="t('healthStatus.searchPlaceholder')"
            class="input text-sm w-48"
          />
          <!-- Refresh -->
          <button
            class="inline-flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 shadow-sm transition hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700"
            :disabled="refreshing"
            @click="handleRefresh"
          >
            <svg
              class="h-4 w-4"
              :class="{ 'animate-spin': refreshing }"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            {{ refreshing ? t('healthStatus.refreshing') : t('healthStatus.refresh') }}
          </button>
          <!-- Status Filter -->
          <select v-model="statusFilter" class="input text-sm">
            <option value="all">{{ t('healthStatus.filter.all') }}</option>
            <option value="available">{{ t('healthStatus.filter.available') }}</option>
            <option value="degraded">{{ t('healthStatus.filter.degraded') }}</option>
            <option value="unavailable">{{ t('healthStatus.filter.unavailable') }}</option>
          </select>
          <select v-model="summaryHours" class="input text-sm" @change="loadSummaries">
            <option :value="6">6h</option>
            <option :value="12">12h</option>
            <option :value="24">24h</option>
          </select>
        </div>
      </div>

      <!-- Status Stats Bar -->
      <div class="flex flex-wrap items-center gap-3">
        <span class="inline-flex items-center gap-1.5 rounded-full bg-emerald-100 px-3 py-1 text-sm font-medium text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300">
          <span class="h-2 w-2 rounded-full bg-emerald-500"></span>
          {{ t('healthStatus.online') }} {{ onlineCount }}
        </span>
        <span class="inline-flex items-center gap-1.5 rounded-full bg-red-100 px-3 py-1 text-sm font-medium text-red-700 dark:bg-red-900/40 dark:text-red-300">
          <span class="h-2 w-2 rounded-full bg-red-500"></span>
          {{ t('healthStatus.offline') }} {{ offlineCount }}
        </span>
        <span class="inline-flex items-center gap-1.5 rounded-full bg-gray-100 px-3 py-1 text-sm font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-400">
          <span class="h-2 w-2 rounded-full bg-gray-400"></span>
          {{ t('healthStatus.unknown') }} {{ unknownCount }}
        </span>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="flex justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <!-- Empty -->
      <div v-else-if="filteredGroups.length === 0" class="card p-12 text-center">
        <div class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700">
          <svg class="h-8 w-8 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z" />
          </svg>
        </div>
        <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('healthStatus.empty') }}
        </h3>
        <p class="text-gray-500 dark:text-dark-400">
          {{ t('healthStatus.emptyDesc') }}
        </p>
      </div>

      <!-- Group Cards Grid -->
      <div v-else class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <div
          v-for="group in filteredGroups"
          :key="group.group_id"
          class="card overflow-hidden transition-shadow hover:shadow-md"
        >
          <!-- Header -->
          <div class="flex items-center justify-between border-b border-gray-100 px-4 py-3 dark:border-dark-700">
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2 min-w-0">
                <div :class="['h-2.5 w-2.5 shrink-0 rounded-full', statusDotClass(group.status)]"></div>
                <span class="truncate font-medium text-gray-900 dark:text-white">
                  {{ cleanGroupName(group.group_name || String(group.group_id)) }}
                </span>
                <span
                  v-if="group.rate_multiplier && group.rate_multiplier !== 1.0"
                  class="shrink-0 rounded bg-blue-100 px-1.5 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/40 dark:text-blue-300"
                >
                  ×{{ group.rate_multiplier }}
                </span>
              </div>
              <div v-if="group.probe_model" class="mt-1 ml-5">
                <span class="inline-block rounded bg-gray-100 px-1.5 py-0.5 text-[11px] text-gray-500 dark:bg-dark-600 dark:text-gray-400">
                  {{ group.probe_model }}
                </span>
              </div>
            </div>
            <span :class="statusBadgeClass(group.status)" class="shrink-0 ml-2">
              {{ statusLabel(group.status) }}
            </span>
          </div>

          <!-- Metrics -->
          <div class="space-y-3 p-4">
            <!-- Latency -->
            <div class="flex items-center justify-between text-sm">
              <span class="text-gray-500 dark:text-gray-400">{{ t('healthStatus.latency') }}</span>
              <span class="font-mono text-gray-900 dark:text-white">
                {{ group.latency_ms > 0 ? group.latency_ms + ' ms' : '-' }}
              </span>
            </div>

            <!-- Last Check -->
            <div class="flex items-center justify-between text-sm">
              <span class="text-gray-500 dark:text-gray-400">{{ t('healthStatus.lastCheck') }}</span>
              <span class="text-xs text-gray-500 dark:text-gray-400">
                {{ formatRelativeTime(group.checked_at) }}
              </span>
            </div>

            <!-- 24h Availability Bar -->
            <div v-if="groupSummaryMap[group.group_id]?.length" class="mt-2">
              <div class="mb-1 flex items-center justify-between text-xs">
                <span class="text-gray-500 dark:text-gray-400">{{ t('healthStatus.availability') }}</span>
                <span class="font-medium" :class="availabilityTextClass(groupAvailabilityPct(group.group_id))">
                  {{ groupAvailabilityPct(group.group_id).toFixed(1) }}%
                </span>
              </div>
              <!-- Timeline blocks -->
              <div class="flex gap-px overflow-hidden rounded" :title="t('healthStatus.timelineHint')">
                <div
                  v-for="(bucket, idx) in groupSummaryMap[group.group_id]"
                  :key="idx"
                  class="h-5 flex-1 transition-colors"
                  :class="timelineBlockClass(bucket.availability_pct)"
                  :title="`${formatTime(bucket.bucket_time)}: ${bucket.availability_pct.toFixed(1)}% (${bucket.avg_latency_ms.toFixed(0)}ms)`"
                ></div>
              </div>
              <div class="mt-1 flex justify-between text-[10px] text-gray-400 dark:text-gray-500">
                <span>{{ summaryHours }}h {{ t('healthStatus.ago') }}</span>
                <span>{{ t('healthStatus.now') }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { healthStatusAPI } from '@/api/healthStatus'
import type { HealthStatusResult, HealthStatusSummary } from '@/api/healthStatus'

const { t } = useI18n()

const loading = ref(true)
const refreshing = ref(false)
const statusFilter = ref('all')
const summaryHours = ref(24)
const searchQuery = ref('')

const latestResults = ref<HealthStatusResult[]>([])
const allSummaries = ref<HealthStatusSummary[]>([])

// Status counts
const onlineCount = computed(() => latestResults.value.filter(g => g.status === 1).length)
const offlineCount = computed(() => latestResults.value.filter(g => g.status === 0).length)
const unknownCount = computed(() => latestResults.value.filter(g => g.status !== 0 && g.status !== 1).length)

// Group summaries by group_id, sorted by bucket_time ascending
const groupSummaryMap = computed<Record<number, HealthStatusSummary[]>>(() => {
  const map: Record<number, HealthStatusSummary[]> = {}
  for (const s of allSummaries.value) {
    if (!map[s.group_id]) map[s.group_id] = []
    map[s.group_id].push(s)
  }
  // Sort each group's summaries by time
  for (const gid of Object.keys(map)) {
    map[Number(gid)].sort((a, b) => new Date(a.bucket_time).getTime() - new Date(b.bucket_time).getTime())
  }
  return map
})

// Calculated availability per group
function groupAvailabilityPct(groupId: number): number {
  const buckets = groupSummaryMap.value[groupId]
  if (!buckets?.length) return 0
  const total = buckets.reduce((sum, b) => sum + b.total_probes, 0)
  const success = buckets.reduce((sum, b) => sum + b.success_count, 0)
  return total > 0 ? (success / total) * 100 : 0
}

const filteredGroups = computed(() => {
  let list = latestResults.value

  // Search filter
  if (searchQuery.value.trim()) {
    const query = searchQuery.value.trim().toLowerCase()
    list = list.filter(g => {
      const name = cleanGroupName(g.group_name || String(g.group_id)).toLowerCase()
      return name.includes(query)
    })
  }

  // Status filter
  if (statusFilter.value === 'available') list = list.filter(g => g.status === 1)
  else if (statusFilter.value === 'degraded') list = list.filter(g => g.status === 2 || g.status === 3)
  else if (statusFilter.value === 'unavailable') list = list.filter(g => g.status === 0)

  return list
})

function statusLabel(status: number): string {
  switch (status) {
    case 0: return t('healthStatus.statusLabels.unavailable')
    case 1: return t('healthStatus.statusLabels.available')
    case 2: return t('healthStatus.statusLabels.degraded')
    case 3: return t('healthStatus.statusLabels.rateLimited')
    default: return t('healthStatus.statusLabels.unknown')
  }
}

function statusDotClass(status: number): string {
  switch (status) {
    case 0: return 'bg-red-500'
    case 1: return 'bg-emerald-500'
    case 2: return 'bg-amber-500'
    case 3: return 'bg-orange-500'
    default: return 'bg-gray-400'
  }
}

function statusBadgeClass(status: number): string {
  const base = 'rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap'
  switch (status) {
    case 0: return `${base} bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300`
    case 1: return `${base} bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300`
    case 2: return `${base} bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300`
    case 3: return `${base} bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300`
    default: return `${base} bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-400`
  }
}

function timelineBlockClass(pct: number): string {
  if (pct >= 95) return 'bg-emerald-400 dark:bg-emerald-500'
  if (pct >= 80) return 'bg-amber-400 dark:bg-amber-500'
  if (pct > 0) return 'bg-red-400 dark:bg-red-500'
  return 'bg-gray-200 dark:bg-dark-600'
}

function availabilityTextClass(pct: number): string {
  if (pct >= 95) return 'text-emerald-600 dark:text-emerald-400'
  if (pct >= 80) return 'text-amber-600 dark:text-amber-400'
  return 'text-red-600 dark:text-red-400'
}

function formatRelativeTime(ts: string): string {
  if (!ts) return '-'
  try {
    const diff = Date.now() - new Date(ts).getTime()
    const mins = Math.floor(diff / 60000)
    if (mins < 1) return t('healthStatus.justNow')
    if (mins < 60) return t('healthStatus.minutesAgo', { n: mins })
    const hours = Math.floor(mins / 60)
    if (hours < 24) return t('healthStatus.hoursAgo', { n: hours })
    return new Date(ts).toLocaleString()
  } catch {
    return ts
  }
}

function formatTime(ts: string): string {
  if (!ts) return '-'
  try {
    return new Date(ts).toLocaleString()
  } catch {
    return ts
  }
}

function cleanGroupName(name: string): string {
  return name.replace(/\s*\(gAI\)\s*$/, '')
}

async function loadLatest() {
  try {
    latestResults.value = await healthStatusAPI.getLatest()
  } catch {
    // silent
  }
}

async function loadSummaries() {
  try {
    allSummaries.value = await healthStatusAPI.getAllSummaries(summaryHours.value)
  } catch {
    // silent
  }
}

async function handleRefresh() {
  refreshing.value = true
  await Promise.all([loadLatest(), loadSummaries()])
  refreshing.value = false
}

onMounted(async () => {
  loading.value = true
  await Promise.all([loadLatest(), loadSummaries()])
  loading.value = false
})
</script>
