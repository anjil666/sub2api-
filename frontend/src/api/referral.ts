/**
 * Referral API endpoints
 * Handles referral info, referred users, and commission records for users
 */

import { apiClient } from './client'

export interface ReferralStats {
  referral_code: string
  referral_link: string
  total_referred: number
  total_commission: number
  commission_rate: number
}

export interface ReferredUser {
  id: number
  email: string
  created_at: string
}

export interface ReferralCommission {
  id: number
  referred_email: string
  order_code: string
  order_amount: number
  commission_rate: number
  commission_amount: number
  status: string
  created_at: string
}

export interface ReferralCommissionListResponse {
  items: ReferralCommission[]
  total: number
  page: number
  page_size: number
  pages: number
}

/**
 * Get referral info (stats + code + link)
 */
export async function getReferralInfo(): Promise<ReferralStats> {
  const { data } = await apiClient.get<ReferralStats>('/user/referral/info')
  return data
}

/**
 * Get referred users list
 */
export async function getReferredUsers(params?: {
  page?: number
  page_size?: number
}): Promise<{ items: ReferredUser[]; total: number; page: number; page_size: number; pages: number }> {
  const { data } = await apiClient.get('/user/referral/users', { params })
  return data
}

/**
 * Get commission records
 */
export async function getCommissions(params?: {
  page?: number
  page_size?: number
}): Promise<ReferralCommissionListResponse> {
  const { data } = await apiClient.get<ReferralCommissionListResponse>('/user/referral/commissions', { params })
  return data
}

export const referralAPI = {
  getReferralInfo,
  getReferredUsers,
  getCommissions
}

export default referralAPI
