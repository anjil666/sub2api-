import { apiClient } from './client'

export interface HealthStatusResult {
  group_id: number
  status: number
  latency_ms: number
  checked_at: string
}

export interface HealthStatusSummary {
  id: number
  group_id: number
  bucket_time: string
  total_probes: number
  success_count: number
  avg_latency_ms: number
  availability_pct: number
  created_at: string
}

export async function getLatest(): Promise<HealthStatusResult[]> {
  const { data } = await apiClient.get<HealthStatusResult[]>('/health-status/latest')
  return data ?? []
}

export async function getAllSummaries(hours?: number): Promise<HealthStatusSummary[]> {
  const { data } = await apiClient.get<HealthStatusSummary[]>('/health-status/summaries', {
    params: hours ? { hours } : undefined
  })
  return data ?? []
}

export async function getGroupSummaries(groupId: number, hours?: number): Promise<HealthStatusSummary[]> {
  const { data } = await apiClient.get<HealthStatusSummary[]>(`/health-status/groups/${groupId}/summaries`, {
    params: hours ? { hours } : undefined
  })
  return data ?? []
}

export const healthStatusAPI = {
  getLatest,
  getAllSummaries,
  getGroupSummaries
}

export default healthStatusAPI
