/**
 * Admin Referral API endpoints
 * Handles referral commission management for administrators
 */

import { apiClient } from '../client'

export interface AdminReferralCommission {
  id: number
  referrer_id: number
  referred_id: number
  referred_email: string
  order_code: string
  order_amount: number
  commission_rate: number
  commission_amount: number
  status: string
  created_at: string
}

/**
 * Get all referral commission records (admin)
 */
export async function getCommissions(params?: {
  page?: number
  page_size?: number
}): Promise<{ items: AdminReferralCommission[]; total: number; page: number; page_size: number; pages: number }> {
  const { data } = await apiClient.get('/admin/referral/commissions', { params })
  return data
}

export const referralAPI = {
  getCommissions
}

export default referralAPI
