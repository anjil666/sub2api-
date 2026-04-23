import { apiClient } from '../client'

// ── TypeScript 接口 ──

export interface UpstreamSite {
  id: number
  name: string
  platform: string
  base_url: string
  credential_mode: 'api_key' | 'login'
  api_key_masked: string
  email_masked: string
  has_password: boolean
  price_multiplier: number
  sync_enabled: boolean
  sync_interval_minutes: number
  last_sync_at: string | null
  last_sync_status: string
  last_sync_error: string
  last_sync_model_count: number
  status: string
  site_type: string
  managed_resource_count: number
  created_at: string
  updated_at: string
}

export interface CreateUpstreamSiteRequest {
  name: string
  base_url: string
  credential_mode: 'api_key' | 'login'
  api_key?: string
  email?: string
  password?: string
  price_multiplier: number
  sync_enabled: boolean
  sync_interval_minutes: number
  site_type?: string
}

export interface UpdateUpstreamSiteRequest {
  name: string
  base_url: string
  credential_mode: 'api_key' | 'login'
  api_key?: string
  email?: string
  password?: string
  price_multiplier: number
  sync_enabled: boolean
  sync_interval_minutes: number
  status?: string
  site_type?: string
}

export interface UpstreamBalanceInfo {
  balance_usd: number
  used_usd: number
  remaining_usd: number
}

export interface UpstreamModelInfo {
  id: string
  type: string
  display_name: string
}

export interface UpstreamManagedResource {
  id: number
  upstream_key_id: string
  upstream_key_prefix: string
  upstream_key_name: string
  upstream_group_id: number | null
  managed_group_id: number | null
  managed_account_id: number | null
  managed_channel_id: number | null
  price_multiplier: number
  upstream_rate_multiplier: number
  model_count: number
  model_filter: string
  status: string
  last_synced_at: string | null
  created_at: string
  updated_at: string
}

export interface SyncResult {
  models_discovered: number
  keys_discovered?: number
  group_id?: number
  account_id?: number
  channel_id?: number
  error?: string
}

interface PaginatedResponse<T> {
  items: T[]
  total: number
  page: number
  page_size: number
  pages: number
}

// ── API 方法 ──

async function list(params?: { page?: number; page_size?: number; status?: string; search?: string }): Promise<PaginatedResponse<UpstreamSite>> {
  const { data } = await apiClient.get('/admin/upstream-sites', { params })
  return data
}

async function getById(id: number): Promise<UpstreamSite> {
  const { data } = await apiClient.get(`/admin/upstream-sites/${id}`)
  return data
}

async function create(req: CreateUpstreamSiteRequest): Promise<UpstreamSite> {
  const { data } = await apiClient.post('/admin/upstream-sites', req)
  return data
}

async function update(id: number, req: UpdateUpstreamSiteRequest): Promise<UpstreamSite> {
  const { data } = await apiClient.put(`/admin/upstream-sites/${id}`, req)
  return data
}

async function remove(id: number): Promise<void> {
  await apiClient.delete(`/admin/upstream-sites/${id}`)
}

async function syncNow(id: number): Promise<SyncResult> {
  const { data } = await apiClient.post(`/admin/upstream-sites/${id}/sync`)
  return data
}

async function getBalance(id: number): Promise<UpstreamBalanceInfo> {
  const { data } = await apiClient.get(`/admin/upstream-sites/${id}/balance`)
  return data
}

async function getModels(id: number): Promise<UpstreamModelInfo[]> {
  const { data } = await apiClient.get(`/admin/upstream-sites/${id}/models`)
  return data
}

async function listResources(id: number): Promise<UpstreamManagedResource[]> {
  const { data } = await apiClient.get(`/admin/upstream-sites/${id}/resources`)
  return data
}

async function updateResource(siteId: number, resourceId: number, req: { price_multiplier: number; model_filter?: string }): Promise<UpstreamManagedResource> {
  const { data } = await apiClient.put(`/admin/upstream-sites/${siteId}/resources/${resourceId}`, req)
  return data
}

async function toggleResource(siteId: number, resourceId: number): Promise<UpstreamManagedResource> {
  const { data } = await apiClient.post(`/admin/upstream-sites/${siteId}/resources/${resourceId}/toggle`)
  return data
}

async function deleteResource(siteId: number, resourceId: number): Promise<void> {
  await apiClient.delete(`/admin/upstream-sites/${siteId}/resources/${resourceId}`)
}

async function toggle(id: number): Promise<UpstreamSite> {
  const { data } = await apiClient.post(`/admin/upstream-sites/${id}/toggle`)
  return data
}

export const upstreamAPI = {
  list,
  getById,
  create,
  update,
  remove,
  syncNow,
  getBalance,
  getModels,
  listResources,
  updateResource,
  toggleResource,
  deleteResource,
  toggle
}

export default upstreamAPI
