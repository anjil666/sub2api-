<template>
  <div class="space-y-6">
    <!-- Loading State -->
    <div v-if="loadingConfig" class="flex justify-center py-12">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
    </div>

    <template v-else>
      <!-- Config Card -->
      <div class="card p-6">
        <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('admin.healthProbe.config.title') }}
            </h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.healthProbe.config.description') }}
            </p>
          </div>
          <div class="flex items-center gap-2">
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="triggering"
              @click="handleTrigger"
            >
              {{ triggering ? t('common.loading') : t('admin.healthProbe.triggerNow') }}
            </button>
          </div>
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
          <!-- Enabled -->
          <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 md:col-span-3">
            <input v-model="configForm.enabled" type="checkbox" />
            <span>{{ t('admin.healthProbe.config.enabled') }}</span>
          </label>

          <!-- Interval -->
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t('admin.healthProbe.config.intervalMinutes') }}
            </label>
            <input v-model.number="configForm.interval_minutes" type="number" min="1" class="input w-full" />
          </div>

          <!-- Timeout -->
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t('admin.healthProbe.config.timeoutSeconds') }}
            </label>
            <input v-model.number="configForm.timeout_seconds" type="number" min="1" class="input w-full" />
          </div>

          <!-- Retention -->
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t('admin.healthProbe.config.retentionHours') }}
            </label>
            <input v-model.number="configForm.retention_hours" type="number" min="1" class="input w-full" />
          </div>

          <!-- Slow Threshold -->
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t('admin.healthProbe.config.slowThresholdMs') }}
            </label>
            <input v-model.number="configForm.slow_threshold_ms" type="number" min="100" class="input w-full" />
          </div>
        </div>

        <!-- Webhook Section -->
        <div class="mt-6 border-t border-gray-100 pt-4 dark:border-dark-700">
          <h4 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('admin.healthProbe.config.webhook.title') }}
          </h4>
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
            <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 md:col-span-3">
              <input v-model="configForm.webhook_enabled" type="checkbox" />
              <span>{{ t('admin.healthProbe.config.webhook.enabled') }}</span>
            </label>

            <div class="md:col-span-3">
              <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('admin.healthProbe.config.webhook.url') }}
              </label>
              <input
                v-model="configForm.webhook_url"
                class="input w-full"
                placeholder="https://..."
                :disabled="!configForm.webhook_enabled"
              />
            </div>

            <div>
              <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('admin.healthProbe.config.webhook.debounceCount') }}
              </label>
              <input
                v-model.number="configForm.webhook_debounce_count"
                type="number"
                min="1"
                class="input w-full"
                :disabled="!configForm.webhook_enabled"
              />
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.healthProbe.config.webhook.debounceHint') }}
              </p>
            </div>

            <div>
              <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('admin.healthProbe.config.webhook.cooldownMinutes') }}
              </label>
              <input
                v-model.number="configForm.webhook_cooldown_minutes"
                type="number"
                min="0"
                class="input w-full"
                :disabled="!configForm.webhook_enabled"
              />
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.healthProbe.config.webhook.cooldownHint') }}
              </p>
            </div>
          </div>
        </div>

        <div class="mt-4">
          <button type="button" class="btn btn-primary btn-sm" :disabled="savingConfig" @click="saveConfig">
            {{ savingConfig ? t('common.loading') : t('common.save') }}
          </button>
        </div>
      </div>

      <!-- Latest Results -->
      <div class="card p-6">
        <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('admin.healthProbe.results.title') }}
            </h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.healthProbe.results.description') }}
            </p>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="loadingResults" @click="loadResults">
            {{ loadingResults ? t('common.loading') : t('common.refresh') }}
          </button>
        </div>

        <div v-if="loadingResults" class="flex justify-center py-8">
          <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
        </div>

        <div v-else-if="results.length === 0" class="py-8 text-center text-gray-500 dark:text-gray-400">
          {{ t('admin.healthProbe.results.empty') }}
        </div>

        <div v-else class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-200 dark:border-dark-600">
                <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">{{ t('admin.healthProbe.results.group') }}</th>
                <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">{{ t('admin.healthProbe.results.model') }}</th>
                <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">{{ t('admin.healthProbe.results.status') }}</th>
                <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">{{ t('admin.healthProbe.results.latency') }}</th>
                <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">{{ t('admin.healthProbe.results.error') }}</th>
                <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">{{ t('admin.healthProbe.results.time') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="r in results"
                :key="r.id"
                class="border-b border-gray-100 dark:border-dark-700 hover:bg-gray-50 dark:hover:bg-dark-700/50"
              >
                <td class="px-3 py-2 text-gray-900 dark:text-white">{{ r.group_id }}</td>
                <td class="px-3 py-2 text-gray-700 dark:text-gray-300">{{ r.probe_model || '-' }}</td>
                <td class="px-3 py-2">
                  <span :class="statusBadgeClass(r.status)">{{ statusLabel(r.status) }}</span>
                </td>
                <td class="px-3 py-2 font-mono text-gray-700 dark:text-gray-300">
                  {{ r.latency_ms > 0 ? r.latency_ms + ' ms' : '-' }}
                </td>
                <td class="max-w-xs truncate px-3 py-2 text-gray-500 dark:text-gray-400">
                  <span v-if="r.error_type">{{ r.error_type }}</span>
                  <span v-if="r.error_message" class="ml-1 text-xs">{{ r.error_message }}</span>
                  <span v-if="!r.error_type && !r.error_message">-</span>
                </td>
                <td class="px-3 py-2 text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap">
                  {{ formatTime(r.checked_at) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Summaries -->
      <div class="card p-6">
        <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('admin.healthProbe.summaries.title') }}
            </h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.healthProbe.summaries.description') }}
            </p>
          </div>
          <div class="flex items-center gap-2">
            <select v-model="summaryHours" class="input" @change="loadSummaries">
              <option :value="6">6h</option>
              <option :value="12">12h</option>
              <option :value="24">24h</option>
              <option :value="48">48h</option>
              <option :value="72">72h</option>
            </select>
          </div>
        </div>

        <div v-if="loadingSummaries" class="flex justify-center py-8">
          <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
        </div>

        <div v-else-if="summaries.length === 0" class="py-8 text-center text-gray-500 dark:text-gray-400">
          {{ t('admin.healthProbe.summaries.empty') }}
        </div>

        <div v-else class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-200 dark:border-dark-600">
                <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">{{ t('admin.healthProbe.results.group') }}</th>
                <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">{{ t('admin.healthProbe.summaries.bucket') }}</th>
                <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">{{ t('admin.healthProbe.summaries.totalProbes') }}</th>
                <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">{{ t('admin.healthProbe.summaries.successCount') }}</th>
                <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">{{ t('admin.healthProbe.summaries.availability') }}</th>
                <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">{{ t('admin.healthProbe.summaries.avgLatency') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="s in summaries"
                :key="s.id"
                class="border-b border-gray-100 dark:border-dark-700 hover:bg-gray-50 dark:hover:bg-dark-700/50"
              >
                <td class="px-3 py-2 text-gray-900 dark:text-white">{{ s.group_id }}</td>
                <td class="px-3 py-2 text-xs text-gray-500 dark:text-gray-400 whitespace-nowrap">{{ formatTime(s.bucket_time) }}</td>
                <td class="px-3 py-2 text-gray-700 dark:text-gray-300">{{ s.total_probes }}</td>
                <td class="px-3 py-2 text-gray-700 dark:text-gray-300">{{ s.success_count }}</td>
                <td class="px-3 py-2">
                  <span :class="availabilityBadgeClass(s.availability_pct)">
                    {{ s.availability_pct.toFixed(1) }}%
                  </span>
                </td>
                <td class="px-3 py-2 font-mono text-gray-700 dark:text-gray-300">{{ s.avg_latency_ms.toFixed(0) }} ms</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api'
import { useAppStore } from '@/stores'
import type { HealthProbeConfig, HealthProbeResult, HealthProbeSummary, UpdateHealthProbeConfigRequest } from '@/api/admin/healthProbe'

const { t } = useI18n()
const appStore = useAppStore()

// Config
const loadingConfig = ref(true)
const savingConfig = ref(false)
const triggering = ref(false)
const configForm = ref<UpdateHealthProbeConfigRequest>({
  enabled: false,
  interval_minutes: 5,
  timeout_seconds: 15,
  retention_hours: 72,
  slow_threshold_ms: 5000,
  webhook_enabled: false,
  webhook_url: '',
  webhook_debounce_count: 2,
  webhook_cooldown_minutes: 30,
})

// Results
const loadingResults = ref(false)
const results = ref<HealthProbeResult[]>([])

// Summaries
const loadingSummaries = ref(false)
const summaries = ref<HealthProbeSummary[]>([])
const summaryHours = ref(24)

async function loadConfig() {
  loadingConfig.value = true
  try {
    const cfg = await adminAPI.healthProbe.getConfig()
    configForm.value = {
      enabled: cfg.enabled,
      interval_minutes: cfg.interval_minutes,
      timeout_seconds: cfg.timeout_seconds,
      retention_hours: cfg.retention_hours,
      slow_threshold_ms: cfg.slow_threshold_ms,
      webhook_enabled: cfg.webhook_enabled,
      webhook_url: cfg.webhook_url,
      webhook_debounce_count: cfg.webhook_debounce_count,
      webhook_cooldown_minutes: cfg.webhook_cooldown_minutes,
    }
  } catch (e: any) {
    appStore.showToast(e.message || 'Failed to load config', 'error')
  } finally {
    loadingConfig.value = false
  }
}

async function saveConfig() {
  savingConfig.value = true
  try {
    await adminAPI.healthProbe.updateConfig(configForm.value)
    appStore.showToast(t('admin.healthProbe.config.saveSuccess'), 'success')
  } catch (e: any) {
    appStore.showToast(e.message || 'Failed to save config', 'error')
  } finally {
    savingConfig.value = false
  }
}

async function handleTrigger() {
  triggering.value = true
  try {
    await adminAPI.healthProbe.triggerProbe()
    appStore.showToast(t('admin.healthProbe.triggerSuccess'), 'success')
    // Reload results after a short delay
    setTimeout(() => loadResults(), 3000)
  } catch (e: any) {
    appStore.showToast(e.message || 'Failed to trigger probe', 'error')
  } finally {
    triggering.value = false
  }
}

async function loadResults() {
  loadingResults.value = true
  try {
    results.value = await adminAPI.healthProbe.getLatestResults()
  } catch (e: any) {
    appStore.showToast(e.message || 'Failed to load results', 'error')
  } finally {
    loadingResults.value = false
  }
}

async function loadSummaries() {
  loadingSummaries.value = true
  try {
    summaries.value = await adminAPI.healthProbe.getAllSummaries(summaryHours.value)
  } catch (e: any) {
    appStore.showToast(e.message || 'Failed to load summaries', 'error')
  } finally {
    loadingSummaries.value = false
  }
}

function statusLabel(status: number): string {
  switch (status) {
    case 0: return t('admin.healthProbe.status.unavailable')
    case 1: return t('admin.healthProbe.status.available')
    case 2: return t('admin.healthProbe.status.degraded')
    case 3: return t('admin.healthProbe.status.rateLimited')
    default: return t('admin.healthProbe.status.unknown')
  }
}

function statusBadgeClass(status: number): string {
  const base = 'rounded-full px-2 py-0.5 text-xs font-medium'
  switch (status) {
    case 0: return `${base} bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300`
    case 1: return `${base} bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300`
    case 2: return `${base} bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300`
    case 3: return `${base} bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300`
    default: return `${base} bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-400`
  }
}

function availabilityBadgeClass(pct: number): string {
  const base = 'rounded-full px-2 py-0.5 text-xs font-medium'
  if (pct >= 95) return `${base} bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300`
  if (pct >= 80) return `${base} bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300`
  return `${base} bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300`
}

function formatTime(ts: string): string {
  if (!ts) return '-'
  try {
    return new Date(ts).toLocaleString()
  } catch {
    return ts
  }
}

onMounted(async () => {
  await loadConfig()
  loadResults()
  loadSummaries()
})
</script>
