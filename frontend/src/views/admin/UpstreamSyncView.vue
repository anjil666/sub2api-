<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.upstream.description') }}
          </div>
          <div class="ml-auto flex items-center gap-2">
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('common.search')"
              class="input input-sm w-48"
              @keyup.enter="loadItems"
            />
            <button class="btn btn-secondary btn-sm" @click="loadItems">
              {{ t('common.refresh') }}
            </button>
            <button class="btn btn-primary btn-sm" @click="openCreateDialog">
              {{ t('admin.upstream.addSite') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="items" :loading="loading">
          <template #cell-name="{ row }">
            <div>
              <div class="font-medium text-gray-900 dark:text-white">{{ row.name }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400 truncate max-w-xs">{{ row.base_url }}</div>
            </div>
          </template>
          <template #cell-credential_mode="{ row }">
            <span
              class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium"
              :class="row.credential_mode === 'login'
                ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
                : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400'"
            >
              {{ row.credential_mode === 'login' ? t('admin.upstream.modeLogin') : t('admin.upstream.modeApiKey') }}
            </span>
          </template>
          <template #cell-status="{ row }">
            <span
              class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium cursor-pointer"
              :class="row.status === 'active'
                ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400'"
              @click="handleToggle(row)"
            >
              {{ row.status === 'active' ? t('admin.upstream.statusActive') : t('admin.upstream.statusDisabled') }}
            </span>
          </template>
          <template #cell-sync_status="{ row }">
            <div class="flex items-center gap-1.5">
              <span
                class="inline-block h-2 w-2 rounded-full"
                :class="{
                  'bg-green-500': row.last_sync_status === 'success',
                  'bg-red-500': row.last_sync_status === 'error',
                  'bg-gray-400': row.last_sync_status === 'pending'
                }"
              />
              <span class="text-xs">
                {{ row.last_sync_status === 'success' ? t('admin.upstream.syncSuccess') :
                   row.last_sync_status === 'error' ? t('admin.upstream.syncError') :
                   t('admin.upstream.syncPending') }}
              </span>
            </div>
            <div v-if="row.last_sync_at" class="text-xs text-gray-400 mt-0.5">
              {{ formatDateTime(row.last_sync_at) }}
            </div>
          </template>
          <template #cell-last_sync_model_count="{ value }">
            <span class="font-medium">{{ value }}</span>
          </template>
          <template #cell-price_multiplier="{ value }">
            <span class="font-medium">{{ Number(value).toFixed(2) }}x</span>
          </template>
          <template #cell-managed="{ row }">
            <button
              v-if="row.managed_resource_count > 0"
              class="text-sm text-blue-600 dark:text-blue-400 hover:underline"
              @click="handleViewResources(row)"
            >
              {{ t('admin.upstream.resourceCount', { count: row.managed_resource_count }) }}
            </button>
            <span v-else class="text-xs text-gray-400">
              {{ t('admin.upstream.notSynced') }}
            </span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <button class="btn btn-xs btn-primary" :disabled="syncing[row.id]" @click="handleSync(row)">
                {{ syncing[row.id] ? t('admin.upstream.syncing') : t('admin.upstream.syncNow') }}
              </button>
              <button class="btn btn-xs btn-secondary" @click="handleViewModels(row)">
                {{ t('admin.upstream.models') }}
              </button>
              <button class="btn btn-xs btn-secondary" @click="handleViewBalance(row)">
                {{ t('admin.upstream.balance') }}
              </button>
              <button class="btn btn-xs btn-secondary" @click="openEditDialog(row)">
                {{ t('common.edit') }}
              </button>
              <button class="btn btn-xs btn-danger" @click="handleDelete(row)">
                {{ t('common.delete') }}
              </button>
            </div>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- 新建/编辑弹窗 -->
    <BaseDialog :show="dialogVisible" :title="dialogMode === 'create' ? t('admin.upstream.addSite') : t('admin.upstream.editSite')" @close="dialogVisible = false">
      <form class="space-y-4" @submit.prevent="handleSubmit">
        <div>
          <label class="label">{{ t('admin.upstream.form.name') }}</label>
          <input v-model="form.name" type="text" class="input" required maxlength="200" />
        </div>
        <div>
          <label class="label">{{ t('admin.upstream.form.baseUrl') }}</label>
          <input v-model="form.base_url" type="url" class="input" required maxlength="500" :placeholder="'https://example.com'" />
        </div>

        <!-- 认证方式选择 -->
        <div>
          <label class="label">{{ t('admin.upstream.credentialMode') }}</label>
          <div class="flex items-center gap-4 mt-1">
            <label class="flex items-center gap-2 cursor-pointer">
              <input v-model="form.credential_mode" type="radio" value="api_key" class="radio" />
              <span class="text-sm">{{ t('admin.upstream.modeApiKey') }}</span>
            </label>
            <label class="flex items-center gap-2 cursor-pointer">
              <input v-model="form.credential_mode" type="radio" value="login" class="radio" />
              <span class="text-sm">{{ t('admin.upstream.modeLogin') }}</span>
            </label>
          </div>
        </div>

        <!-- API Key 模式 -->
        <div v-if="form.credential_mode === 'api_key'">
          <label class="label">{{ t('admin.upstream.form.apiKey') }}</label>
          <div class="relative">
            <input v-model="form.api_key" :type="showApiKey ? 'text' : 'password'" class="input pr-10" :required="dialogMode === 'create'" :placeholder="dialogMode === 'edit' ? t('admin.upstream.form.apiKeyPlaceholder') : ''" />
            <button type="button" class="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600" @click="showApiKey = !showApiKey">
              {{ showApiKey ? '***' : 'Aa' }}
            </button>
          </div>
        </div>

        <!-- 邮箱密码登录模式 -->
        <template v-if="form.credential_mode === 'login'">
          <div>
            <label class="label">{{ t('admin.upstream.form.email') }}</label>
            <input v-model="form.email" type="email" class="input" :required="dialogMode === 'create'" :placeholder="dialogMode === 'edit' ? t('admin.upstream.form.emailPlaceholder') : ''" />
          </div>
          <div>
            <label class="label">{{ t('admin.upstream.form.password') }}</label>
            <div class="relative">
              <input v-model="form.password" :type="showPassword ? 'text' : 'password'" class="input pr-10" :required="dialogMode === 'create'" :placeholder="dialogMode === 'edit' ? t('admin.upstream.form.passwordPlaceholder') : ''" />
              <button type="button" class="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600" @click="showPassword = !showPassword">
                {{ showPassword ? '***' : 'Aa' }}
              </button>
            </div>
          </div>
          <div class="rounded-lg bg-blue-50 dark:bg-blue-900/20 p-3 text-xs text-blue-700 dark:text-blue-300">
            {{ t('admin.upstream.loginModeHint') }}
          </div>
        </template>

        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="label">{{ t('admin.upstream.form.priceMultiplier') }}</label>
            <input v-model.number="form.price_multiplier" type="number" class="input" step="0.01" min="0.01" />
          </div>
          <div>
            <label class="label">{{ t('admin.upstream.form.syncInterval') }}</label>
            <input v-model.number="form.sync_interval_minutes" type="number" class="input" min="1" />
            <span class="text-xs text-gray-400">{{ t('admin.upstream.form.minutes') }}</span>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <input v-model="form.sync_enabled" type="checkbox" class="checkbox" id="sync-enabled" />
          <label for="sync-enabled" class="text-sm">{{ t('admin.upstream.form.syncEnabled') }}</label>
        </div>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="dialogVisible = false">{{ t('common.cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="submitting">{{ t('common.save') }}</button>
        </div>
      </form>
    </BaseDialog>

    <!-- 模型预览弹窗 -->
    <BaseDialog :show="modelsDialogVisible" :title="t('admin.upstream.modelsTitle')" @close="modelsDialogVisible = false">
      <div v-if="modelsLoading" class="py-8 text-center text-gray-500">{{ t('common.loading') }}...</div>
      <div v-else-if="modelsList.length === 0" class="py-8 text-center text-gray-500">{{ t('admin.upstream.noModels') }}</div>
      <div v-else class="max-h-96 overflow-y-auto">
        <div class="grid grid-cols-1 gap-1">
          <div v-for="model in modelsList" :key="model.id" class="flex items-center justify-between px-3 py-2 rounded bg-gray-50 dark:bg-gray-800">
            <span class="text-sm font-mono">{{ model.id }}</span>
            <span v-if="model.type" class="text-xs text-gray-400">{{ model.type }}</span>
          </div>
        </div>
        <div class="mt-3 text-sm text-gray-500 text-center">{{ t('admin.upstream.totalModels', { count: modelsList.length }) }}</div>
      </div>
    </BaseDialog>

    <!-- 余额弹窗 -->
    <BaseDialog :show="balanceDialogVisible" :title="t('admin.upstream.balanceTitle')" @close="balanceDialogVisible = false">
      <div v-if="balanceLoading" class="py-8 text-center text-gray-500">{{ t('common.loading') }}...</div>
      <div v-else-if="balanceInfo" class="space-y-3 py-4">
        <div class="flex justify-between items-center">
          <span class="text-gray-600 dark:text-gray-400">{{ t('admin.upstream.balanceRemaining') }}</span>
          <span class="text-2xl font-bold text-green-600 dark:text-green-400">${{ balanceInfo.remaining_usd.toFixed(2) }}</span>
        </div>
        <div class="flex justify-between items-center text-sm">
          <span class="text-gray-500">{{ t('admin.upstream.balanceTotal') }}</span>
          <span>${{ balanceInfo.balance_usd.toFixed(2) }}</span>
        </div>
        <div class="flex justify-between items-center text-sm">
          <span class="text-gray-500">{{ t('admin.upstream.balanceUsed') }}</span>
          <span>${{ balanceInfo.used_usd.toFixed(2) }}</span>
        </div>
      </div>
    </BaseDialog>

    <!-- 托管资源弹窗 -->
    <BaseDialog :show="resourcesDialogVisible" :title="t('admin.upstream.resourcesTitle')" @close="resourcesDialogVisible = false">
      <div v-if="resourcesLoading" class="py-8 text-center text-gray-500">{{ t('common.loading') }}...</div>
      <div v-else-if="resourcesList.length === 0" class="py-8 text-center text-gray-500">{{ t('admin.upstream.noResources') }}</div>
      <div v-else class="max-h-96 overflow-y-auto">
        <div class="space-y-2">
          <div v-for="res in resourcesList" :key="res.id" class="rounded-lg border border-gray-200 dark:border-gray-700 p-3">
            <div class="flex items-center justify-between mb-2">
              <div class="flex items-center gap-2">
                <span class="font-medium text-sm">{{ res.upstream_key_name || 'Unnamed Key' }}</span>
                <span
                  class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium"
                  :class="res.status === 'active'
                    ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                    : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400'"
                >
                  {{ res.status }}
                </span>
              </div>
              <span class="text-xs text-gray-400">{{ res.model_count }} {{ t('admin.upstream.models') }}</span>
            </div>
            <div class="text-xs text-gray-500 dark:text-gray-400 font-mono mb-1">
              {{ t('admin.upstream.keyPrefix') }}: {{ res.upstream_key_prefix }}
            </div>
            <div class="flex flex-wrap gap-2 text-xs">
              <span v-if="res.managed_group_id" class="text-blue-600 dark:text-blue-400">
                {{ t('admin.upstream.group') }}#{{ res.managed_group_id }}
              </span>
              <span v-if="res.managed_account_id" class="text-purple-600 dark:text-purple-400">
                {{ t('admin.upstream.account') }}#{{ res.managed_account_id }}
              </span>
              <span v-if="res.managed_channel_id" class="text-orange-600 dark:text-orange-400">
                {{ t('admin.upstream.channel') }}#{{ res.managed_channel_id }}
              </span>
            </div>
            <div v-if="res.last_synced_at" class="text-xs text-gray-400 mt-1">
              {{ t('admin.upstream.lastSynced') }}: {{ formatDateTime(res.last_synced_at) }}
            </div>
          </div>
        </div>
      </div>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { adminAPI } from '@/api/admin'
import type { UpstreamSite, UpstreamModelInfo, UpstreamBalanceInfo, UpstreamManagedResource } from '@/api/admin/upstream'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'

const { t } = useI18n()

// ── 列表状态 ──
const items = ref<UpstreamSite[]>([])
const loading = ref(false)
const searchQuery = ref('')
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0
})

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.upstream.columns.name'), width: '180px' },
  { key: 'credential_mode', label: t('admin.upstream.columns.credentialMode'), width: '100px' },
  { key: 'status', label: t('admin.upstream.columns.status'), width: '90px' },
  { key: 'sync_status', label: t('admin.upstream.columns.syncStatus'), width: '120px' },
  { key: 'last_sync_model_count', label: t('admin.upstream.columns.modelCount'), width: '80px' },
  { key: 'price_multiplier', label: t('admin.upstream.columns.multiplier'), width: '80px' },
  { key: 'managed', label: t('admin.upstream.columns.resources'), width: '120px' },
  { key: 'actions', label: t('common.actions'), width: '320px' }
])

// ── 同步状态 ──
const syncing = reactive<Record<number, boolean>>({})

// ── 弹窗状态 ──
const dialogVisible = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const editingId = ref<number | null>(null)
const submitting = ref(false)
const showApiKey = ref(false)
const showPassword = ref(false)
const form = reactive({
  name: '',
  base_url: '',
  credential_mode: 'api_key' as 'api_key' | 'login',
  api_key: '',
  email: '',
  password: '',
  price_multiplier: 1.0,
  sync_enabled: true,
  sync_interval_minutes: 60
})

const modelsDialogVisible = ref(false)
const modelsLoading = ref(false)
const modelsList = ref<UpstreamModelInfo[]>([])

const balanceDialogVisible = ref(false)
const balanceLoading = ref(false)
const balanceInfo = ref<UpstreamBalanceInfo | null>(null)

const resourcesDialogVisible = ref(false)
const resourcesLoading = ref(false)
const resourcesList = ref<UpstreamManagedResource[]>([])

// ── 数据加载 ──
const loadItems = async () => {
  loading.value = true
  try {
    const response = await adminAPI.upstream.list({
      page: pagination.page,
      page_size: pagination.page_size,
      search: searchQuery.value || undefined
    })
    items.value = response.items || []
    pagination.total = response.total
  } catch {
    items.value = []
  } finally {
    loading.value = false
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  loadItems()
}

function handlePageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  loadItems()
}

// ── 新建/编辑 ──
function openCreateDialog() {
  dialogMode.value = 'create'
  editingId.value = null
  showApiKey.value = false
  showPassword.value = false
  Object.assign(form, {
    name: '',
    base_url: '',
    credential_mode: 'api_key',
    api_key: '',
    email: '',
    password: '',
    price_multiplier: 1.0,
    sync_enabled: true,
    sync_interval_minutes: 60
  })
  dialogVisible.value = true
}

function openEditDialog(site: UpstreamSite) {
  dialogMode.value = 'edit'
  editingId.value = site.id
  showApiKey.value = false
  showPassword.value = false
  Object.assign(form, {
    name: site.name,
    base_url: site.base_url,
    credential_mode: site.credential_mode || 'api_key',
    api_key: '',
    email: '',
    password: '',
    price_multiplier: site.price_multiplier,
    sync_enabled: site.sync_enabled,
    sync_interval_minutes: site.sync_interval_minutes
  })
  dialogVisible.value = true
}

async function handleSubmit() {
  submitting.value = true
  try {
    if (dialogMode.value === 'create') {
      await adminAPI.upstream.create({
        name: form.name,
        base_url: form.base_url,
        credential_mode: form.credential_mode,
        api_key: form.credential_mode === 'api_key' ? form.api_key : undefined,
        email: form.credential_mode === 'login' ? form.email : undefined,
        password: form.credential_mode === 'login' ? form.password : undefined,
        price_multiplier: form.price_multiplier,
        sync_enabled: form.sync_enabled,
        sync_interval_minutes: form.sync_interval_minutes
      })
    } else if (editingId.value) {
      await adminAPI.upstream.update(editingId.value, {
        name: form.name,
        base_url: form.base_url,
        credential_mode: form.credential_mode,
        api_key: form.credential_mode === 'api_key' && form.api_key ? form.api_key : undefined,
        email: form.credential_mode === 'login' && form.email ? form.email : undefined,
        password: form.credential_mode === 'login' && form.password ? form.password : undefined,
        price_multiplier: form.price_multiplier,
        sync_enabled: form.sync_enabled,
        sync_interval_minutes: form.sync_interval_minutes
      })
    }
    dialogVisible.value = false
    loadItems()
  } catch (err: any) {
    alert(err?.message || 'Failed')
  } finally {
    submitting.value = false
  }
}

// ── 操作 ──
async function handleSync(site: UpstreamSite) {
  syncing[site.id] = true
  try {
    const result = await adminAPI.upstream.syncNow(site.id)
    const keysMsg = result.keys_discovered ? `, ${result.keys_discovered} keys` : ''
    alert(t('admin.upstream.syncComplete', { count: result.models_discovered }) + keysMsg)
    loadItems()
  } catch (err: any) {
    alert(err?.message || 'Sync failed')
  } finally {
    syncing[site.id] = false
  }
}

async function handleToggle(site: UpstreamSite) {
  try {
    await adminAPI.upstream.toggle(site.id)
    loadItems()
  } catch (err: any) {
    alert(err?.message || 'Toggle failed')
  }
}

async function handleDelete(site: UpstreamSite) {
  if (!confirm(t('admin.upstream.confirmDelete', { name: site.name }))) return
  try {
    await adminAPI.upstream.remove(site.id)
    loadItems()
  } catch (err: any) {
    alert(err?.message || 'Delete failed')
  }
}

async function handleViewModels(site: UpstreamSite) {
  modelsDialogVisible.value = true
  modelsLoading.value = true
  modelsList.value = []
  try {
    modelsList.value = await adminAPI.upstream.getModels(site.id)
  } catch (err: any) {
    alert(err?.message || 'Failed to fetch models')
    modelsDialogVisible.value = false
  } finally {
    modelsLoading.value = false
  }
}

async function handleViewBalance(site: UpstreamSite) {
  balanceDialogVisible.value = true
  balanceLoading.value = true
  balanceInfo.value = null
  try {
    balanceInfo.value = await adminAPI.upstream.getBalance(site.id)
  } catch (err: any) {
    alert(err?.message || 'Failed to fetch balance')
    balanceDialogVisible.value = false
  } finally {
    balanceLoading.value = false
  }
}

async function handleViewResources(site: UpstreamSite) {
  resourcesDialogVisible.value = true
  resourcesLoading.value = true
  resourcesList.value = []
  try {
    resourcesList.value = await adminAPI.upstream.listResources(site.id)
  } catch (err: any) {
    alert(err?.message || 'Failed to fetch resources')
    resourcesDialogVisible.value = false
  } finally {
    resourcesLoading.value = false
  }
}

// ── 工具 ──
function formatDateTime(value: string): string {
  if (!value) return ''
  return new Date(value).toLocaleString()
}

onMounted(loadItems)
</script>
