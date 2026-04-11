<template>
  <AppLayout>
    <div class="mx-auto max-w-4xl space-y-6">
      <!-- Loading -->
      <div v-if="loading" class="flex items-center justify-center py-20">
        <svg class="h-8 w-8 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
        </svg>
        <span class="ml-3 text-gray-500 dark:text-gray-400">{{ t('referral.loading') }}</span>
      </div>

      <!-- Error -->
      <div v-else-if="error" class="rounded-xl border border-red-200 bg-red-50 p-6 text-center dark:border-red-800/50 dark:bg-red-900/20">
        <p class="text-red-700 dark:text-red-400">{{ t('referral.error') }}</p>
      </div>

      <!-- Content -->
      <template v-else-if="stats">
        <!-- Stats Cards -->
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <div class="card p-5">
            <div class="text-sm text-gray-500 dark:text-gray-400">{{ t('referral.stats.totalReferrals') }}</div>
            <div class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ stats.total_referred }}</div>
          </div>
          <div class="card p-5">
            <div class="text-sm text-gray-500 dark:text-gray-400">{{ t('referral.stats.totalEarnings') }}</div>
            <div class="mt-1 text-2xl font-bold text-green-600 dark:text-green-400">${{ stats.total_commission.toFixed(2) }}</div>
          </div>
          <div class="card p-5">
            <div class="text-sm text-gray-500 dark:text-gray-400">{{ t('referral.stats.commissionRate') }}</div>
            <div class="mt-1 text-2xl font-bold text-primary-600 dark:text-primary-400">{{ (stats.commission_rate * 100).toFixed(0) }}%</div>
          </div>
        </div>

        <!-- Referral Code & Link -->
        <div class="card p-6">
          <div class="space-y-4">
            <div>
              <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('referral.myReferralCode') }}</label>
              <div class="flex items-center gap-2">
                <code class="flex-1 rounded-lg bg-gray-100 px-4 py-2.5 font-mono text-lg tracking-wider text-gray-900 dark:bg-dark-700 dark:text-white">{{ stats.referral_code }}</code>
                <button @click="copyToClipboard(stats.referral_code, 'code')" class="btn btn-secondary whitespace-nowrap">
                  {{ codeCopied ? t('referral.copied') : t('referral.copyCode') }}
                </button>
              </div>
            </div>
            <div>
              <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('referral.referralLink') }}</label>
              <div class="flex items-center gap-2">
                <input type="text" readonly :value="stats.referral_link" class="input flex-1 font-mono text-sm" />
                <button @click="copyToClipboard(stats.referral_link, 'link')" class="btn btn-secondary whitespace-nowrap">
                  {{ linkCopied ? t('referral.copied') : t('referral.copyLink') }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Referred Users -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('referral.referredUsers') }}</h2>
          </div>
          <div v-if="referredUsers.length === 0" class="p-6 text-center text-gray-500 dark:text-gray-400">
            {{ t('referral.referredUsersEmpty') }}
          </div>
          <div v-else class="overflow-x-auto">
            <table class="w-full text-left text-sm">
              <thead>
                <tr class="border-b border-gray-100 dark:border-dark-700">
                  <th class="px-6 py-3 font-medium text-gray-500 dark:text-gray-400">{{ t('referral.referredEmail') }}</th>
                  <th class="px-6 py-3 font-medium text-gray-500 dark:text-gray-400">{{ t('referral.registeredAt') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="user in referredUsers" :key="user.id" class="border-b border-gray-50 dark:border-dark-700/50">
                  <td class="px-6 py-3 text-gray-900 dark:text-white">{{ user.email }}</td>
                  <td class="px-6 py-3 text-gray-500 dark:text-gray-400">{{ formatDateTime(user.created_at) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- Commission Records -->
        <div class="card">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('referral.commissions') }}</h2>
          </div>
          <div v-if="commissions.length === 0" class="p-6 text-center text-gray-500 dark:text-gray-400">
            {{ t('referral.commissionsEmpty') }}
          </div>
          <div v-else class="overflow-x-auto">
            <table class="w-full text-left text-sm">
              <thead>
                <tr class="border-b border-gray-100 dark:border-dark-700">
                  <th class="px-6 py-3 font-medium text-gray-500 dark:text-gray-400">{{ t('referral.referredEmail') }}</th>
                  <th class="px-6 py-3 font-medium text-gray-500 dark:text-gray-400">{{ t('referral.orderAmount') }}</th>
                  <th class="px-6 py-3 font-medium text-gray-500 dark:text-gray-400">{{ t('referral.commissionAmount') }}</th>
                  <th class="px-6 py-3 font-medium text-gray-500 dark:text-gray-400">{{ t('referral.status') }}</th>
                  <th class="px-6 py-3 font-medium text-gray-500 dark:text-gray-400">{{ t('referral.time') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="commission in commissions" :key="commission.id" class="border-b border-gray-50 dark:border-dark-700/50">
                  <td class="px-6 py-3 text-gray-900 dark:text-white">{{ commission.referred_email }}</td>
                  <td class="px-6 py-3 text-gray-900 dark:text-white">${{ commission.order_amount.toFixed(2) }}</td>
                  <td class="px-6 py-3 font-medium text-green-600 dark:text-green-400">+${{ commission.commission_amount.toFixed(2) }}</td>
                  <td class="px-6 py-3">
                    <span class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium"
                      :class="commission.status === 'credited'
                        ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                        : 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'">
                      {{ commission.status === 'credited' ? t('referral.statusCredited') : t('referral.statusPending') }}
                    </span>
                  </td>
                  <td class="px-6 py-3 text-gray-500 dark:text-gray-400">{{ formatDateTime(commission.created_at) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { referralAPI } from '@/api/referral'
import type { ReferralStats, ReferredUser, ReferralCommission } from '@/api/referral'

const { t } = useI18n()

const loading = ref(true)
const error = ref(false)
const stats = ref<ReferralStats | null>(null)
const referredUsers = ref<ReferredUser[]>([])
const commissions = ref<ReferralCommission[]>([])
const codeCopied = ref(false)
const linkCopied = ref(false)

function formatDateTime(dateStr: string): string {
  const d = new Date(dateStr)
  return d.toLocaleString()
}

async function copyToClipboard(text: string, type: 'code' | 'link') {
  try {
    await navigator.clipboard.writeText(text)
    if (type === 'code') {
      codeCopied.value = true
      setTimeout(() => { codeCopied.value = false }, 2000)
    } else {
      linkCopied.value = true
      setTimeout(() => { linkCopied.value = false }, 2000)
    }
  } catch {
    // fallback
    const textarea = document.createElement('textarea')
    textarea.value = text
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
    if (type === 'code') {
      codeCopied.value = true
      setTimeout(() => { codeCopied.value = false }, 2000)
    } else {
      linkCopied.value = true
      setTimeout(() => { linkCopied.value = false }, 2000)
    }
  }
}

async function loadData() {
  loading.value = true
  error.value = false
  try {
    const [infoRes, usersRes, commissionsRes] = await Promise.all([
      referralAPI.getReferralInfo(),
      referralAPI.getReferredUsers({ page: 1, page_size: 50 }),
      referralAPI.getCommissions({ page: 1, page_size: 50 })
    ])
    stats.value = infoRes
    // 如果后端返回的 referral_link 没有域名，用当前站点域名补全
    if (stats.value && stats.value.referral_link && !stats.value.referral_link.startsWith('http')) {
      stats.value.referral_link = window.location.origin + stats.value.referral_link
    }
    referredUsers.value = usersRes.items || []
    commissions.value = commissionsRes.items || []
  } catch {
    error.value = true
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadData()
})
</script>
