<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.referral.description') }}
          </div>
          <div class="ml-auto">
            <button class="btn btn-secondary" @click="loadItems">
              {{ t('common.refresh') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="items" :loading="loading">
          <template #cell-referred_email="{ value }">
            <span class="text-gray-900 dark:text-white">{{ value }}</span>
          </template>
          <template #cell-order_amount="{ value }">
            ${{ Number(value).toFixed(2) }}
          </template>
          <template #cell-commission_rate="{ value }">
            {{ (Number(value) * 100).toFixed(0) }}%
          </template>
          <template #cell-commission_amount="{ value }">
            <span class="font-medium text-green-600 dark:text-green-400">+${{ Number(value).toFixed(2) }}</span>
          </template>
          <template #cell-status="{ value }">
            <span class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium"
              :class="value === 'completed'
                ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                : 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'">
              {{ value === 'completed' ? t('admin.referral.statusCompleted') : t('admin.referral.statusPending') }}
            </span>
          </template>
          <template #cell-created_at="{ value }">
            {{ formatDateTimeStr(value) }}
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
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { adminAPI } from '@/api/admin'
import type { AdminReferralCommission } from '@/api/admin/referral'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'

const { t } = useI18n()

const items = ref<AdminReferralCommission[]>([])
const loading = ref(false)
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0
})

const columns = computed<Column[]>(() => [
  { key: 'referrer_id', label: t('admin.referral.columns.referrerId') },
  { key: 'referred_email', label: t('admin.referral.columns.referredEmail') },
  { key: 'order_code', label: t('admin.referral.columns.orderCode') },
  { key: 'order_amount', label: t('admin.referral.columns.orderAmount') },
  { key: 'commission_rate', label: t('admin.referral.columns.commissionRate') },
  { key: 'commission_amount', label: t('admin.referral.columns.commissionAmount') },
  { key: 'status', label: t('admin.referral.columns.status') },
  { key: 'created_at', label: t('admin.referral.columns.createdAt') }
])

function formatDateTimeStr(value: string): string {
  if (!value) return ''
  return new Date(value).toLocaleString()
}

const loadItems = async () => {
  loading.value = true
  try {
    const response = await adminAPI.referral.getCommissions({
      page: pagination.page,
      page_size: pagination.page_size
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

onMounted(loadItems)
</script>
