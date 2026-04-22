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

      <!-- Per-Group Probe Model Config -->
      <div class="card p-6">
        <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">
              分组探测模型配置
            </h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              为每个分组指定探测使用的模型，留空表示自动选择
            </p>
          </div>
          <button
            type="button"
            class="btn btn-primary btn-sm"
            :disabled="savingAllGroupConfigs"
            @click="saveAllGroupConfigs"
          >
            {{ savingAllGroupConfigs ? '保存中...' : '全部保存' }}
          </button>
        </div>

        <div v-if="loadingGroupConfigs" class="flex justify-center py-6">
          <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
        </div>

        <div v-else-if="allGroupRows.length === 0" class="py-6 text-center text-gray-500 dark:text-gray-400">
          暂无活跃分组
        </div>

        <div v-else class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-gray-200 dark:border-dark-600">
                <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">分组</th>
                <th class="px-3 py-2 text-center font-medium text-gray-600 dark:text-gray-400">启用探测</th>
                <th class="px-3 py-2 text-left font-medium text-gray-600 dark:text-gray-400">探测模型</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in allGroupRows"
                :key="row.group_id"
                class="border-b border-gray-100 dark:border-dark-700 hover:bg-gray-50 dark:hover:bg-dark-700/50"
              >
                <td class="px-3 py-2 font-medium text-gray-900 dark:text-white whitespace-nowrap">
                  {{ row.group_name }}
                </td>
                <td class="px-3 py-2 text-center">
                  <label class="relative inline-flex cursor-pointer items-center">
                    <input v-model="row.probe_enabled" type="checkbox" class="peer sr-only" />
                    <div class="h-5 w-9 rounded-full bg-gray-200 after:absolute after:left-[2px] after:top-[2px] after:h-4 after:w-4 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-[''] peer-checked:bg-primary-500 peer-checked:after:translate-x-full peer-checked:after:border-white dark:bg-dark-600 dark:peer-checked:bg-primary-600"></div>
                  </label>
                </td>
                <td class="px-3 py-2">
                  <select v-model="row.probe_model" class="input w-full" :disabled="!row.probe_enabled">
                    <option value="">自动选择</option>
                    <option
                      v-for="model in getModelsForGroup(row.group_id)"
                      :key="model"
                      :value="model"
                    >
                      {{ model }}
                    </option>
                  </select>
                </td>
              </tr>
            </tbody>
          </table>
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
                <td class="px-3 py-2 text-gray-900 dark:text-white">
                  {{ r.group_name || r.group_id }}
                  <span v-if="r.rate_multiplier && r.rate_multiplier !== 1" class="ml-1 text-xs text-gray-400">
                    ×{{ r.rate_multiplier }}
                  </span>
                </td>
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
    </template>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api'
import { useAppStore } from '@/stores'
import type { HealthProbeResult, HealthProbeGroupConfig, UpdateHealthProbeConfigRequest } from '@/api/admin/healthProbe'

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

// Group configs
const loadingGroupConfigs = ref(false)
const savingAllGroupConfigs = ref(false)
const groupConfigs = ref<HealthProbeGroupConfig[]>([])
const groupModels = ref<Record<number, string[]>>({})

// All groups name map (loaded from groups API)
const allGroupsMap = ref<Record<number, string>>({})

// Combined table rows: all groups that have models, with current config
interface GroupRow {
  group_id: number
  group_name: string
  probe_model: string
  probe_enabled: boolean
}
const allGroupRows = ref<GroupRow[]>([])

function getGroupName(groupId: number): string {
  return allGroupsMap.value[groupId] || `Group ${groupId}`
}

function getModelsForGroup(groupId: number): string[] {
  return groupModels.value[groupId] || []
}

async function loadGroupModels() {
  try {
    groupModels.value = await adminAPI.healthProbe.getGroupModels()
  } catch {
    // silent
  }
}

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
    appStore.showToast('error', e.message || 'Failed to load config')
  } finally {
    loadingConfig.value = false
  }
}

async function saveConfig() {
  savingConfig.value = true
  try {
    await adminAPI.healthProbe.updateConfig(configForm.value)
    appStore.showToast('success', t('admin.healthProbe.config.saveSuccess'))
  } catch (e: any) {
    appStore.showToast('error', e.message || 'Failed to save config')
  } finally {
    savingConfig.value = false
  }
}

async function handleTrigger() {
  triggering.value = true
  try {
    await adminAPI.healthProbe.triggerProbe()
    appStore.showToast('success', t('admin.healthProbe.triggerSuccess'))
    // Reload results after a short delay
    setTimeout(() => loadResults(), 3000)
  } catch (e: any) {
    appStore.showToast('error', e.message || 'Failed to trigger probe')
  } finally {
    triggering.value = false
  }
}

async function loadResults() {
  loadingResults.value = true
  try {
    results.value = await adminAPI.healthProbe.getLatestResults()
  } catch (e: any) {
    appStore.showToast('error', e.message || 'Failed to load results')
  } finally {
    loadingResults.value = false
  }
}

async function loadGroupConfigs() {
  loadingGroupConfigs.value = true
  try {
    groupConfigs.value = await adminAPI.healthProbe.listGroupConfigs()
  } catch (e: any) {
    appStore.showToast('error', e.message || 'Failed to load group configs')
  } finally {
    loadingGroupConfigs.value = false
  }
}

async function loadAllGroups() {
  try {
    const groups = await adminAPI.groups.getAll()
    const map: Record<number, string> = {}
    for (const g of groups) {
      map[g.id] = g.name
    }
    allGroupsMap.value = map
  } catch {
    // silent
  }
}

function buildGroupRows() {
  const configMap = new Map<number, { probe_model: string; probe_enabled: boolean }>()
  for (const gc of groupConfigs.value) {
    configMap.set(Number(gc.group_id), {
      probe_model: gc.probe_model,
      probe_enabled: gc.probe_enabled !== false,
    })
  }
  const rows: GroupRow[] = []
  const groupIds = Object.keys(groupModels.value).map(Number).sort((a, b) => a - b)
  for (const gid of groupIds) {
    const cfg = configMap.get(gid)
    rows.push({
      group_id: gid,
      group_name: getGroupName(gid),
      probe_model: cfg?.probe_model || '',
      probe_enabled: cfg?.probe_enabled !== false,
    })
  }
  allGroupRows.value = rows
}

async function saveAllGroupConfigs() {
  savingAllGroupConfigs.value = true
  try {
    const configs = allGroupRows.value
      .map(row => ({ group_id: row.group_id, probe_model: row.probe_model, probe_enabled: row.probe_enabled }))

    if (configs.length > 0) {
      await adminAPI.healthProbe.batchUpsertGroupConfigs(configs)
    }

    await loadGroupConfigs()
    buildGroupRows()
    appStore.showToast('success', '已保存全部分组配置')
  } catch (e: any) {
    appStore.showToast('error', e.message || 'Failed to save')
  } finally {
    savingAllGroupConfigs.value = false
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
  await Promise.all([loadResults(), loadGroupConfigs(), loadGroupModels(), loadAllGroups()])
  buildGroupRows()
})
</script>
