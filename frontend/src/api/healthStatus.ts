import { apiClient } from './client'

export interface HealthStatusConfig {
  timeout_seconds: number
}

export interface HealthStatusResult {
  group_id: number
  group_name: string
  rate_multiplier: number
  platform: string
  probe_model: string
  status: number
  latency_ms: number
  checked_at: string
  billing_display?: string
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

export async function getConfig(): Promise<HealthStatusConfig> {
  const { data } = await apiClient.get<HealthStatusConfig>('/health-status/config')
  return data
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
  getConfig,
  getLatest,
  getAllSummaries,
  getGroupSummaries
}

export default healthStatusAPI
