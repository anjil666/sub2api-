import { ref, computed, watch, onUnmounted } from 'vue'
import axios, { type AxiosInstance } from 'axios'
import { keysAPI } from '@/api/keys'
import { modelsAPI, type GroupModels } from '@/api/models'

export interface VideoModel {
  id: string
  name: string
}

export interface VideoTask {
  id: string
  model: string
  prompt: string
  aspect_ratio: string
  status: 'pending' | 'processing' | 'completed' | 'failed'
  video_url: string
  error?: string
  created_at: number
  _pollTimer?: ReturnType<typeof setInterval>
}

function translateError(msg: string): string {
  if (!msg) return '未知错误'
  const map: [RegExp, string][] = [
    [/all available accounts exhausted/i, '所有可用账号已耗尽，请稍后重试'],
    [/rate limit/i, '请求频率过高，请稍后重试'],
    [/timeout/i, '请求超时，请重试'],
    [/network error/i, '网络错误，请检查连接'],
    [/content policy/i, '内容违反安全策略，请修改提示词'],
    [/billing/i, '账户余额不足'],
    [/unauthorized|401/i, '认证失败，请检查密钥'],
    [/forbidden|403/i, '无权限访问'],
    [/bad gateway|502/i, '当前分组维护中，请更换分组重试'],
    [/service unavailable|503/i, '当前分组维护中，请更换分组重试'],
    [/internal server error|500/i, '服务器内部错误'],
  ]
  for (const [re, zh] of map) {
    if (re.test(msg)) return zh
  }
  return msg
}
function extractApiError(e: any): string {
  const msg = e?.response?.data?.error?.message || e?.response?.data?.message || e?.message || ''
  return translateError(msg)
}

export function useVideoGeneration() {
  const loading = ref(false)
  const loadingGroups = ref(false)
  const loadingModels = ref(false)
  const submitting = ref(false)
  const error = ref('')

  // group & key
  const groups = ref<GroupModels[]>([])
  const apiKeys = ref<{ key: string; group_id: number | null }[]>([])
  const selectedGroupId = ref<number | null>(null)

  // video models & price
  const videoModels = ref<VideoModel[]>([])
  const selectedModel = ref('')
  const price = ref<number>(0)

  // generation params
  const prompt = ref('')
  const aspectRatio = ref('16:9')

  // tasks
  const tasks = ref<VideoTask[]>([])
  const pollTimers = new Map<string, ReturnType<typeof setInterval>>()

  const videoGroups = computed(() =>
    groups.value.filter(g => g.video_studio_enabled)
  )
  const selectedGroup = computed(() =>
    videoGroups.value.find(g => g.group_id === selectedGroupId.value) || null
  )

  const groupApiKey = computed(() => {
    if (!selectedGroupId.value) return ''
    const k = apiKeys.value.find(k => k.group_id === selectedGroupId.value)
    return k?.key || ''
  })

  function createAxios(): AxiosInstance {
    return axios.create({
      baseURL: window.location.origin,
      timeout: 600000,
      headers: { Authorization: `Bearer ${groupApiKey.value}` },
    })
  }

  async function loadGroupsAndKeys() {
    loadingGroups.value = true
    error.value = ''
    try {
      const [gData, kData] = await Promise.all([
        modelsAPI.getGroupedModels(),
        keysAPI.list(1, 200),
      ])
      groups.value = gData
      apiKeys.value = kData.items.filter(k => k.status === 'active').map(k => ({ key: k.key, group_id: k.group_id }))
      if (!selectedGroupId.value && videoGroups.value.length) {
        selectedGroupId.value = videoGroups.value[0].group_id
      }
    } catch (e: any) {
      error.value = extractApiError(e) || '加载分组失败'
    } finally {
      loadingGroups.value = false
    }
  }
  async function fetchVideoModels() {
    if (!groupApiKey.value) return
    loadingModels.value = true
    try {
      const api = createAxios()
      const { data } = await api.get('/api/v1/user/video/models')
      const resp = data.data || data
      videoModels.value = resp.models || []
      price.value = resp.price || 0
      if (videoModels.value.length && !videoModels.value.find(m => m.id === selectedModel.value)) {
        selectedModel.value = videoModels.value[0].id
      }
    } catch (e: any) {
      error.value = extractApiError(e) || '加载视频模型失败'
    } finally {
      loadingModels.value = false
    }
  }

  watch(selectedGroupId, () => {
    if (groupApiKey.value) {
      fetchVideoModels()
    }
  })

  async function submitGeneration() {
    if (!prompt.value.trim() || !groupApiKey.value || !selectedModel.value) return
    submitting.value = true
    error.value = ''
    try {
      const api = createAxios()
      const { data } = await api.post('/api/v1/user/video/generations', {
        model: selectedModel.value,
        prompt: prompt.value,
        aspect_ratio: aspectRatio.value,
      })
      const resp = data.data || data
      const task: VideoTask = {
        id: resp.id,
        model: selectedModel.value,
        prompt: prompt.value,
        aspect_ratio: aspectRatio.value,
        status: resp.status || 'pending',
        video_url: '',
        created_at: Date.now(),
      }
      tasks.value.unshift(task)
      startPolling(task.id)
    } catch (e: any) {
      error.value = extractApiError(e) || '提交视频生成失败'
    } finally {
      submitting.value = false
    }
  }
  function startPolling(taskId: string) {
    if (pollTimers.has(taskId)) return
    const timer = setInterval(async () => {
      try {
        const api = createAxios()
        const { data } = await api.get(`/api/v1/user/video/generations/${taskId}`)
        const resp = data.data || data
        const task = tasks.value.find(t => t.id === taskId)
        if (!task) { stopPolling(taskId); return }
        task.status = resp.status
        if (resp.video_url) task.video_url = resp.video_url
        if (resp.error) task.error = resp.error
        if (task.status === 'completed' || task.status === 'failed') {
          stopPolling(taskId)
        }
      } catch {
        // keep polling on transient errors
      }
    }, 10000)
    pollTimers.set(taskId, timer)
  }

  function stopPolling(taskId: string) {
    const timer = pollTimers.get(taskId)
    if (timer) {
      clearInterval(timer)
      pollTimers.delete(taskId)
    }
  }

  function stopAllPolling() {
    for (const [id] of pollTimers) {
      stopPolling(id)
    }
  }

  function removeTask(taskId: string) {
    stopPolling(taskId)
    tasks.value = tasks.value.filter(t => t.id !== taskId)
  }

  onUnmounted(() => {
    stopAllPolling()
  })

  return {
    loading, loadingGroups, loadingModels, submitting, error,
    groups: videoGroups, selectedGroupId, selectedGroup, groupApiKey,
    videoModels, selectedModel, price,
    prompt, aspectRatio,
    tasks,
    loadGroupsAndKeys, fetchVideoModels, submitGeneration,
    removeTask, stopAllPolling,
  }
}
