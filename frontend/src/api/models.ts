/**
 * Model Square API endpoints
 * Handles fetching available models with pricing for the current user
 */

import { apiClient } from './client'

export interface ModelInfo {
  model_name: string
  provider: string
  mode: string
  input_price_per_million: number
  output_price_per_million: number
  cache_write_price_per_million: number
  cache_read_price_per_million: number
  supports_prompt_caching: boolean
  has_pricing: boolean
  billing_mode?: string
  per_request_price?: number
}

export interface GroupModels {
  group_id: number
  group_name: string
  platform: string
  rate_multiplier: number
  billing_display?: string
  image_price_1k?: number | null
  image_price_2k?: number | null
  image_price_4k?: number | null
  image_studio_enabled?: boolean
  models: ModelInfo[]
}

/**
 * Get all available models grouped by the user's accessible groups
 * @returns List of groups with their available models and pricing
 */
export async function getGroupedModels(): Promise<GroupModels[]> {
  const { data } = await apiClient.get<GroupModels[]>('/models')
  return data
}

export const modelsAPI = {
  getGroupedModels
}

export default modelsAPI
