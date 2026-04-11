<template>
  <AppLayout>
    <div class="mx-auto max-w-4xl space-y-6">
      <div class="grid grid-cols-1 gap-6 sm:grid-cols-3">
        <StatCard :title="t('profile.accountBalance')" :value="formatCurrency(user?.balance || 0)" :icon="WalletIcon" icon-variant="success" />
        <StatCard :title="t('profile.concurrencyLimit')" :value="user?.concurrency || 0" :icon="BoltIcon" icon-variant="warning" />
        <StatCard :title="t('profile.memberSince')" :value="formatDate(user?.created_at || '', { year: 'numeric', month: 'long' })" :icon="CalendarIcon" icon-variant="primary" />
      </div>

      <!-- 每日签到卡片 -->
      <div v-if="checkinEnabled" class="card overflow-hidden">
        <div class="flex items-center justify-between p-6">
          <div class="flex items-center gap-4">
            <div class="flex h-12 w-12 items-center justify-center rounded-xl" :class="hasCheckedInToday ? 'bg-green-100 text-green-600 dark:bg-green-900/30 dark:text-green-400' : 'bg-primary-100 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400'">
              <svg v-if="hasCheckedInToday" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <svg v-else class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('checkin.title') }}</h3>
              <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('checkin.description') }}</p>
            </div>
          </div>
          <button
            @click="handleCheckin"
            :disabled="hasCheckedInToday || checkinLoading"
            class="inline-flex items-center gap-2 rounded-lg px-5 py-2.5 text-sm font-semibold transition-all"
            :class="hasCheckedInToday
              ? 'bg-gray-100 text-gray-400 cursor-not-allowed dark:bg-dark-700 dark:text-gray-500'
              : 'bg-primary-600 text-white hover:bg-primary-700 shadow-sm hover:shadow active:scale-95 dark:bg-primary-500 dark:hover:bg-primary-600'"
          >
            <svg v-if="checkinLoading" class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
            </svg>
            {{ hasCheckedInToday ? t('checkin.alreadyDone') : t('checkin.button') }}
          </button>
        </div>
      </div>

      <!-- 推荐返利入口卡片 -->
      <div v-if="referralEnabled" class="card overflow-hidden">
        <div class="flex items-center justify-between p-6">
          <div class="flex items-center gap-4">
            <div class="flex h-12 w-12 items-center justify-center rounded-xl bg-primary-100 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400">
              <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M18 18.72a9.094 9.094 0 003.741-.479 3 3 0 00-4.682-2.72m.94 3.198l.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0112 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 016 18.719m12 0a5.971 5.971 0 00-.941-3.197m0 0A5.995 5.995 0 0012 12.75a5.995 5.995 0 00-5.058 2.772m0 0a3 3 0 00-4.681 2.72 8.986 8.986 0 003.74.477m.94-3.197a5.971 5.971 0 00-.94 3.197M15 6.75a3 3 0 11-6 0 3 3 0 016 0zm6 3a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0zm-13.5 0a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0z" />
              </svg>
            </div>
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('referral.title') }}</h3>
              <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('referral.description') }}</p>
            </div>
          </div>
          <router-link to="/referral" class="inline-flex items-center gap-2 rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-semibold text-white shadow-sm transition-all hover:bg-primary-700 hover:shadow active:scale-95 dark:bg-primary-500 dark:hover:bg-primary-600">
            {{ t('referral.title') }}
          </router-link>
        </div>
      </div>

      <ProfileInfoCard :user="user" />
      <div v-if="contactInfo" class="card border-primary-200 bg-primary-50 dark:bg-primary-900/20 p-6">
        <div class="flex items-center gap-4">
          <div class="p-3 bg-primary-100 rounded-xl text-primary-600"><Icon name="chat" size="lg" /></div>
          <div><h3 class="font-semibold text-primary-800 dark:text-primary-200">{{ t('common.contactSupport') }}</h3><p class="text-sm font-medium">{{ contactInfo }}</p></div>
        </div>
      </div>
      <ProfileEditForm :initial-username="user?.username || ''" />
      <ProfilePasswordForm />
      <ProfileTotpCard />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, h, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { formatDate } from '@/utils/format'
import { authAPI, userAPI } from '@/api'
import AppLayout from '@/components/layout/AppLayout.vue'
import StatCard from '@/components/common/StatCard.vue'
import ProfileInfoCard from '@/components/user/profile/ProfileInfoCard.vue'
import ProfileEditForm from '@/components/user/profile/ProfileEditForm.vue'
import ProfilePasswordForm from '@/components/user/profile/ProfilePasswordForm.vue'
import ProfileTotpCard from '@/components/user/profile/ProfileTotpCard.vue'
import { Icon } from '@/components/icons'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const user = computed(() => authStore.user)
const contactInfo = ref('')
const checkinLoading = ref(false)

const checkinEnabled = computed(() => appStore.cachedPublicSettings?.checkin_enabled ?? false)
const referralEnabled = computed(() => appStore.cachedPublicSettings?.referral_enabled ?? false)

const hasCheckedInToday = computed(() => {
  const lastCheckin = user.value?.last_checkin_at
  if (!lastCheckin) return false
  const beijing = new Date(lastCheckin)
  const now = new Date()
  // 按北京时间判断同一天 (UTC+8)
  const toBeijingDate = (d: Date) => {
    const utc = d.getTime() + d.getTimezoneOffset() * 60000
    const bj = new Date(utc + 8 * 3600000)
    return `${bj.getFullYear()}-${bj.getMonth()}-${bj.getDate()}`
  }
  return toBeijingDate(beijing) === toBeijingDate(now)
})

const WalletIcon = { render: () => h('svg', { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' }, [h('path', { d: 'M21 12a2.25 2.25 0 00-2.25-2.25H15a3 3 0 11-6 0H5.25A2.25 2.25 0 003 12' })]) }
const BoltIcon = { render: () => h('svg', { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' }, [h('path', { d: 'm3.75 13.5 10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z' })]) }
const CalendarIcon = { render: () => h('svg', { fill: 'none', viewBox: '0 0 24 24', stroke: 'currentColor', 'stroke-width': '1.5' }, [h('path', { d: 'M6.75 3v2.25M17.25 3v2.25' })]) }

async function handleCheckin() {
  checkinLoading.value = true
  try {
    const result = await userAPI.dailyCheckin()
    appStore.showSuccess(t('checkin.success', { amount: result.reward.toFixed(2) }))
    // 刷新用户数据以更新余额和签到时间
    await authStore.refreshUser()
  } catch (error: any) {
    const msg = error?.response?.data?.message || error?.message || t('common.unknownError')
    appStore.showError(msg)
  } finally {
    checkinLoading.value = false
  }
}

onMounted(async () => {
  try {
    const s = await authAPI.getPublicSettings()
    contactInfo.value = s.contact_info || ''
  } catch (error) {
    console.error('Failed to load contact info:', error)
  }
})

const formatCurrency = (v: number) => `$${v.toFixed(2)}`
</script>
