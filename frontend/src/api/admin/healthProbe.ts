import { apiClient } from '../client'

export interface HealthProbeConfig {
  id: number
  enabled: boolean
  interval_minutes: number
  timeout_seconds: number
  retention_hours: number
  slow_threshold_ms: number
  webhook_enabled: boolean
  webhook_url: string
  webhook_debounce_count: number
  webhook_cooldown_minutes: number
  created_at: string
  updated_at: string
}

export interface HealthProbeResult {
  id: number
  account_id: number
  group_id: number
  probe_model: string
  status: number // 0=unavailable, 1=available, 2=degraded, 3=rate_limited
  latency_ms: number
  error_type: string
  http_status_code: number
  error_message: string
  checked_at: string
  group_name?: string
  rate_multiplier?: number
  platform?: string
}

export interface HealthProbeSummary {
  id: number
  group_id: number
  bucket_time: string
  total_probes: number
  success_count: number
  avg_latency_ms: number
  availability_pct: number
  created_at: string
}

export interface UpdateHealthProbeConfigRequest {
  enabled?: boolean
  interval_minutes?: number
  timeout_seconds?: number
  retention_hours?: number
  slow_threshold_ms?: number
  webhook_enabled?: boolean
  webhook_url?: string
  webhook_debounce_count?: number
  webhook_cooldown_minutes?: number
}

export interface HealthProbeGroupConfig {
  id: number
  group_id: number
  probe_model: string
  created_at: string
  updated_at: string
}

export async function getConfig(): Promise<HealthProbeConfig> {
  const { data } = await apiClient.get<HealthProbeConfig>('/admin/health-probe/config')
  return data
}

export async function updateConfig(req: UpdateHealthProbeConfigRequest): Promise<HealthProbeConfig> {
  const { data } = await apiClient.put<HealthProbeConfig>('/admin/health-probe/config', req)
  return data
}

export async function triggerProbe(): Promise<void> {
  await apiClient.post('/admin/health-probe/trigger')
}

export async function getLatestResults(): Promise<HealthProbeResult[]> {
  const { data } = await apiClient.get<HealthProbeResult[]>('/admin/health-probe/latest')
  return data ?? []
}

export async function getAllSummaries(hours?: number): Promise<HealthProbeSummary[]> {
  const { data } = await apiClient.get<HealthProbeSummary[]>('/admin/health-probe/summaries', {
    params: hours ? { hours } : undefined
  })
  return data ?? []
}

export async function getGroupResults(groupId: number, hours?: number, limit?: number): Promise<HealthProbeResult[]> {
  const { data } = await apiClient.get<HealthProbeResult[]>(`/admin/health-probe/groups/${groupId}/results`, {
    params: { ...(hours ? { hours } : {}), ...(limit ? { limit } : {}) }
  })
  return data ?? []
}

export async function getGroupSummaries(groupId: number, hours?: number): Promise<HealthProbeSummary[]> {
  const { data } = await apiClient.get<HealthProbeSummary[]>(`/admin/health-probe/groups/${groupId}/summaries`, {
    params: hours ? { hours } : undefined
  })
  return data ?? []
}

export async function listGroupConfigs(): Promise<HealthProbeGroupConfig[]> {
  const { data } = await apiClient.get<HealthProbeGroupConfig[]>('/admin/health-probe/group-configs')
  return data ?? []
}

export async function upsertGroupConfig(groupId: number, probeModel: string): Promise<void> {
  await apiClient.put('/admin/health-probe/group-configs', { group_id: groupId, probe_model: probeModel })
}

export async function deleteGroupConfig(groupId: number): Promise<void> {
  await apiClient.delete(`/admin/health-probe/group-configs/${groupId}`)
}

export const healthProbeAPI = {
  getConfig,
  updateConfig,
  triggerProbe,
  getLatestResults,
  getAllSummaries,
  getGroupResults,
  getGroupSummaries,
  listGroupConfigs,
  upsertGroupConfig,
  deleteGroupConfig
}

export default healthProbeAPI
