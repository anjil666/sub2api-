import { ref, onUnmounted } from 'vue'
import { apiClient } from '@/api/client'

export interface VideoTask {
  id: string
  prompt: string
  aspect_ratio: string
  mode: 'text' | 'image'
  status: 'pending' | 'processing' | 'completed' | 'failed'
  video_url: string
  error?: string
  created_at: number
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
  const submitting = ref(false)
  const enhancing = ref(false)
  const error = ref('')

  const prompt = ref('')
  const aspectRatio = ref('9:16')
  const generateCount = ref(1)
  const imageFile = ref<File | null>(null)
  const activeTab = ref<'text' | 'image'>('text')
  const price = ref<number>(0)
  const videoModels = ref<{ id: string; name: string }[]>([])
  const selectedModel = ref('')

  const tasks = ref<VideoTask[]>([])
  const pollTimers = new Map<string, ReturnType<typeof setInterval>>()

  async function fetchPrice() {
    try {
      const { data } = await apiClient.get('/video/models')
      const resp = data.data || data
      price.value = resp.price || 0
      videoModels.value = resp.models || []
      if (videoModels.value.length && !selectedModel.value) {
        selectedModel.value = videoModels.value[0].id
      }
    } catch { /* ignore */ }
  }

  async function enhancePrompt() {
    if (!prompt.value.trim()) return
    enhancing.value = true
    error.value = ''
    try {
      const { data } = await apiClient.post('/video/prompt/enhance', {
        prompt: prompt.value,
      })
      const resp = data.data || data
      if (resp.enhanced) prompt.value = resp.enhanced
    } catch (e: any) {
      error.value = extractApiError(e) || '提示词优化失败'
    } finally {
      enhancing.value = false
    }
  }

  async function submitTextGeneration() {
    if (!prompt.value.trim()) return
    submitting.value = true
    error.value = ''
    try {
      for (let i = 0; i < generateCount.value; i++) {
        const { data } = await apiClient.post('/video/generations', {
          model: selectedModel.value || 'veo-3.1',
          prompt: prompt.value,
          aspect_ratio: aspectRatio.value,
        })
        const resp = data.data || data
        const task: VideoTask = {
          id: resp.id,
          prompt: prompt.value,
          aspect_ratio: aspectRatio.value,
          mode: 'text',
          status: resp.status || 'pending',
          video_url: '',
          created_at: Date.now(),
        }
        tasks.value.unshift(task)
        startPolling(task.id)
      }
    } catch (e: any) {
      error.value = extractApiError(e) || '提交视频生成失败'
    } finally {
      submitting.value = false
    }
  }

  async function submitImg2Video() {
    if (!imageFile.value) return
    submitting.value = true
    error.value = ''
    try {
      const formData = new FormData()
      formData.append('image', imageFile.value)
      formData.append('prompt', prompt.value)
      formData.append('aspect_ratio', aspectRatio.value)
      const { data } = await apiClient.post('/video/img2video', formData, {
        timeout: 600000,
      })
      const resp = data.data || data
      const task: VideoTask = {
        id: resp.id,
        prompt: prompt.value || '图生视频',
        aspect_ratio: aspectRatio.value,
        mode: 'image',
        status: resp.status || 'pending',
        video_url: '',
        created_at: Date.now(),
      }
      tasks.value.unshift(task)
      startPolling(task.id)
    } catch (e: any) {
      error.value = extractApiError(e) || '提交图生视频失败'
    } finally {
      submitting.value = false
    }
  }

  function startPolling(taskId: string) {
    if (pollTimers.has(taskId)) return
    const timer = setInterval(async () => {
      try {
        const { data } = await apiClient.get(`/video/generations/${taskId}`)
        const resp = data.data || data
        const task = tasks.value.find(t => t.id === taskId)
        if (!task) { stopPolling(taskId); return }
        task.status = resp.status
        if (resp.video_url) task.video_url = resp.video_url
        if (resp.error) task.error = resp.error
        if (task.status === 'completed' || task.status === 'failed') {
          stopPolling(taskId)
        }
      } catch { /* keep polling */ }
    }, 10000)
    pollTimers.set(taskId, timer)
  }

  function stopPolling(taskId: string) {
    const timer = pollTimers.get(taskId)
    if (timer) { clearInterval(timer); pollTimers.delete(taskId) }
  }

  function removeTask(taskId: string) {
    stopPolling(taskId)
    tasks.value = tasks.value.filter(t => t.id !== taskId)
  }

  onUnmounted(() => {
    for (const [id] of pollTimers) stopPolling(id)
  })

  return {
    submitting, enhancing, error,
    price, videoModels, selectedModel,
    prompt, aspectRatio, generateCount, imageFile, activeTab,
    tasks,
    fetchPrice, enhancePrompt,
    submitTextGeneration, submitImg2Video,
    removeTask,
  }
}
