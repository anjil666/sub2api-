import { ref, computed, watch } from 'vue'
import axios, { type AxiosInstance } from 'axios'
import { keysAPI } from '@/api/keys'
import { modelsAPI, type GroupModels } from '@/api/models'
import { compressImageIfNeeded } from '@/utils/imageCompression'
import { saveImage } from '@/utils/imageDB'

export type StudioTab = 'generation' | 'edit' | 'batch' | 'storyboard'

export interface SizePreset {
  label: string
  w: number
  h: number
  ratio: string
}

export interface StylePreset {
  label: string
  prefix: string
}

export interface BatchTask {
  id: string
  prompt: string
  referenceFiles: File[]
  status: 'pending' | 'running' | 'success' | 'failed'
  result?: string
  error?: string
  elapsed?: number
}

export interface StoryboardScene {
  id: string
  index: number
  prompt: string
  referenceFile?: File
  status: 'pending' | 'running' | 'success' | 'failed'
  result?: string
  error?: string
}

export const RESOLUTION_TIERS = ['AUTO', '1K', '2K', '4K', 'custom'] as const
export type ResolutionTier = typeof RESOLUTION_TIERS[number]

export type AspectRatio = { label: string; w: number; h: number }

export const ASPECT_RATIOS: readonly AspectRatio[] = [
  { label: '1:1', w: 1, h: 1 },
  { label: '16:9', w: 16, h: 9 },
  { label: '9:16', w: 9, h: 16 },
  { label: '2:3', w: 2, h: 3 },
  { label: '3:2', w: 3, h: 2 },
  { label: '4:3', w: 4, h: 3 },
  { label: '21:9', w: 21, h: 9 },
  { label: '4:5', w: 4, h: 5 },
]

export const STYLE_PRESETS: StylePreset[] = [
  { label: '无风格', prefix: '' },
  { label: '写实摄影', prefix: 'realistic photography style, ' },
  { label: '动漫风格', prefix: 'anime style, ' },
  { label: '油画艺术', prefix: 'oil painting style, ' },
  { label: '水彩插画', prefix: 'watercolor illustration style, ' },
  { label: '像素艺术', prefix: 'pixel art style, ' },
  { label: '3D渲染', prefix: '3D rendering style, ' },
  { label: '极简设计', prefix: 'minimalist design style, ' },
  { label: '赛博朋克', prefix: 'cyberpunk style, ' },
  { label: '短剧写实', prefix: 'cinematic still, dramatic lighting, ' },
  { label: '日系动漫', prefix: 'Japanese anime style, detailed, ' },
  { label: '漫画分镜', prefix: 'manga panel style, black and white, ' },
  { label: '电影海报', prefix: 'movie poster style, dramatic composition, ' },
]

const TIER_BASE: Record<string, number> = { '1K': 1024, '2K': 2048, '4K': 3840 }
const TIER_QUALITY: Record<string, string> = { '1K': 'standard', '2K': 'hd', '4K': '4k' }

export function computeSize(tier: ResolutionTier, ratioW: number, ratioH: number, customW?: number, customH?: number): string {
  if (tier === 'AUTO') return 'auto'
  if (tier === 'custom') {
    const w = customW || 1024
    const h = customH || 1024
    return `${w}x${h}`
  }
  const base = TIER_BASE[tier]
  const maxSide = base
  let w: number, h: number
  if (ratioW >= ratioH) {
    w = maxSide
    h = Math.round((maxSide * ratioH) / ratioW)
  } else {
    h = maxSide
    w = Math.round((maxSide * ratioW) / ratioH)
  }
  w = Math.round(w / 16) * 16
  h = Math.round(h / 16) * 16
  return `${w}x${h}`
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

function extractImageUrl(item: any, format?: string): string {
  if (item.url) return item.url
  if (item.b64_json) {
    const mime = format === 'webp' ? 'image/webp' : format === 'jpeg' ? 'image/jpeg' : 'image/png'
    return `data:${mime};base64,${item.b64_json}`
  }
  if (item.b64) {
    const mime = format === 'webp' ? 'image/webp' : format === 'jpeg' ? 'image/jpeg' : 'image/png'
    return `data:${mime};base64,${item.b64}`
  }
  if (typeof item === 'string' && item.length > 100) {
    const mime = format === 'webp' ? 'image/webp' : format === 'jpeg' ? 'image/jpeg' : 'image/png'
    return `data:${mime};base64,${item}`
  }
  if (typeof item === 'string' && item.startsWith('http')) return item
  return ''
}

export function useImageGeneration() {
  const activeTab = ref<StudioTab>('generation')
  const loading = ref(false)
  const loadingGroups = ref(false)
  const error = ref('')
  const elapsed = ref(0)
  let elapsedTimer: ReturnType<typeof setInterval> | null = null
  let abortController: AbortController | null = null

  // group & model
  const groups = ref<GroupModels[]>([])
  const apiKeys = ref<{ key: string; group_id: number | null }[]>([])
  const selectedGroupId = ref<number | null>(null)
  const selectedModel = ref('')

  // generation params
  const resolutionTier = ref<ResolutionTier>('AUTO')
  const selectedRatio = ref(ASPECT_RATIOS[0])
  const customW = ref(1024)
  const customH = ref(1024)
  const outputFormat = ref<'png' | 'jpeg' | 'webp'>('png')
  const outputCompression = ref(90)
  const stylePreset = ref(STYLE_PRESETS[0])
  const imageCount = ref(1)
  const prompt = ref('')

  // single edit (legacy)
  const maskFile = ref<File | null>(null)

  // multi edit
  const multiFiles = ref<File[]>([])

  // results
  const resultUrls = ref<string[]>([])

  // batch
  const batchTasks = ref<BatchTask[]>([])

  // storyboard
  const storyCharacterFiles = ref<File[]>([])
  const storyScenes = ref<StoryboardScene[]>([])

  const imageGroups = computed(() =>
    groups.value.filter(g => g.image_studio_enabled && g.models.some(m => /^(gpt-image|dall-e|image)/i.test(m.model_name)))
  )

  const selectedGroup = computed(() =>
    imageGroups.value.find(g => g.group_id === selectedGroupId.value) || null
  )

  const imageModels = computed(() =>
    selectedGroup.value?.models.filter(m => /^(gpt-image|dall-e|image)/i.test(m.model_name)) || []
  )

  const groupApiKey = computed(() => {
    if (!selectedGroupId.value) return ''
    const k = apiKeys.value.find(k => k.group_id === selectedGroupId.value)
    return k?.key || ''
  })

  const sizeString = computed(() =>
    computeSize(resolutionTier.value, selectedRatio.value.w, selectedRatio.value.h, customW.value, customH.value)
  )

  const qualityString = computed(() => TIER_QUALITY[resolutionTier.value] || undefined)

  const fullPrompt = computed(() => stylePreset.value.prefix + prompt.value)

  function createAxios(): AxiosInstance {
    return axios.create({
      baseURL: window.location.origin,
      timeout: 600000,
      headers: { Authorization: `Bearer ${groupApiKey.value}` },
    })
  }

  watch(selectedGroupId, () => {
    const models = imageModels.value
    if (models.length && !models.find(m => m.model_name === selectedModel.value)) {
      selectedModel.value = models[0].model_name
    }
  })

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
      if (!selectedGroupId.value && imageGroups.value.length) {
        selectedGroupId.value = imageGroups.value[0].group_id
      }
    } catch (e: any) {
      error.value = extractApiError(e) || '加载分组失败'
    } finally {
      loadingGroups.value = false
    }
  }

  function startTimer() {
    elapsed.value = 0
    elapsedTimer = setInterval(() => elapsed.value++, 1000)
  }
  function stopTimer() {
    if (elapsedTimer) { clearInterval(elapsedTimer); elapsedTimer = null }
  }

  function abort() {
    abortController?.abort()
    abortController = null
    stopTimer()
    loading.value = false
  }

  async function generate() {
    if (!fullPrompt.value.trim() || !groupApiKey.value) return
    loading.value = true
    error.value = ''
    resultUrls.value = []
    abortController = new AbortController()
    startTimer()
    try {
      const api = createAxios()
      const body: Record<string, any> = {
        model: selectedModel.value,
        prompt: fullPrompt.value,
        n: imageCount.value,
        output_format: outputFormat.value,
      }
      if (sizeString.value !== 'auto') body.size = sizeString.value
      if (qualityString.value) body.quality = qualityString.value
      if ((outputFormat.value === 'jpeg' || outputFormat.value === 'webp') && outputCompression.value < 100) {
        body.output_compression = outputCompression.value
      }
      const { data } = await api.post('/v1/images/generations', body, { signal: abortController!.signal })
      console.log('[ImageStudio] generate response keys:', Object.keys(data), 'data.data length:', data.data?.length)
      const items = data.data || data.images || (Array.isArray(data) ? data : [])
      resultUrls.value = items.map((d: any) => extractImageUrl(d, outputFormat.value)).filter(Boolean)
      console.log('[ImageStudio] resultUrls count:', resultUrls.value.length, 'first url length:', resultUrls.value[0]?.length)
      if (!resultUrls.value.length) {
        error.value = '生成完成但未返回有效图片数据'
        console.warn('[ImageStudio] empty result, raw response:', JSON.stringify(data).slice(0, 500))
      }
      for (const url of resultUrls.value) {
        try {
          await saveImage({
            id: crypto.randomUUID(),
            prompt: fullPrompt.value,
            model: selectedModel.value,
            size: sizeString.value,
            mode: 'generation',
            imageUrl: url,
            groupName: selectedGroup.value?.group_name || '',
            style: stylePreset.value.label,
            createdAt: Date.now(),
          })
        } catch (saveErr) {
          console.warn('[ImageStudio] saveImage failed:', saveErr)
        }
      }
    } catch (e: any) {
      if (e.name !== 'CanceledError') error.value = extractApiError(e) || '生成失败'
    } finally {
      stopTimer()
      loading.value = false
    }
  }

  async function editImage() {
    if (!fullPrompt.value.trim() || !groupApiKey.value) return
    loading.value = true
    error.value = ''
    resultUrls.value = []
    abortController = new AbortController()
    startTimer()
    try {
      const api = createAxios()
      const fd = new FormData()
      fd.append('model', selectedModel.value)
      fd.append('prompt', fullPrompt.value)
      if (sizeString.value !== 'auto') fd.append('size', sizeString.value)
      if (qualityString.value) fd.append('quality', qualityString.value)
      fd.append('output_format', outputFormat.value)
      if ((outputFormat.value === 'jpeg' || outputFormat.value === 'webp') && outputCompression.value < 100) {
        fd.append('output_compression', String(outputCompression.value))
      }
      const files = multiFiles.value
      for (const f of files) {
        const compressed = await compressImageIfNeeded(f)
        fd.append('image[]', compressed, compressed.name)
      }
      if (maskFile.value) {
        fd.append('mask', maskFile.value, maskFile.value.name)
      }
      const { data } = await api.post('/v1/images/edits', fd, {
        signal: abortController!.signal,
        headers: { 'Content-Type': 'multipart/form-data' },
      })
      const items = data.data || data.images || (Array.isArray(data) ? data : [])
      resultUrls.value = items.map((d: any) => extractImageUrl(d, outputFormat.value)).filter(Boolean)
      if (!resultUrls.value.length) {
        error.value = '编辑完成但未返回有效图片数据'
        console.warn('[ImageStudio] empty edit result, raw:', JSON.stringify(data).slice(0, 500))
      }
      for (const url of resultUrls.value) {
        try {
          await saveImage({
            id: crypto.randomUUID(),
            prompt: fullPrompt.value,
            model: selectedModel.value,
            size: sizeString.value,
            mode: 'multi-edit',
            imageUrl: url,
            groupName: selectedGroup.value?.group_name || '',
            style: stylePreset.value.label,
            createdAt: Date.now(),
          })
        } catch (saveErr) {
          console.warn('[ImageStudio] saveImage failed:', saveErr)
        }
      }
    } catch (e: any) {
      if (e.name !== 'CanceledError') error.value = extractApiError(e) || '编辑失败'
    } finally {
      stopTimer()
      loading.value = false
    }
  }

  // concurrency limiter
  async function runWithConcurrency<T>(tasks: (() => Promise<T>)[], limit: number): Promise<T[]> {
    const results: T[] = new Array(tasks.length)
    let idx = 0
    async function worker() {
      while (idx < tasks.length) {
        const i = idx++
        results[i] = await tasks[i]()
      }
    }
    await Promise.all(Array.from({ length: Math.min(limit, tasks.length) }, () => worker()))
    return results
  }

  function addBatchTask() {
    batchTasks.value.push({
      id: crypto.randomUUID(),
      prompt: '',
      referenceFiles: [],
      status: 'pending',
    })
  }

  function removeBatchTask(id: string) {
    batchTasks.value = batchTasks.value.filter(t => t.id !== id)
  }

  async function runBatchTasks() {
    if (!groupApiKey.value) return
    const pending = batchTasks.value.filter(t => t.status !== 'success')
    pending.forEach(t => { t.status = 'pending'; t.error = undefined; t.result = undefined })
    abortController = new AbortController()
    const tasks = pending.map(task => async () => {
      task.status = 'running'
      const start = Date.now()
      try {
        const api = createAxios()
        let data: any
        if (task.referenceFiles.length) {
          const fd = new FormData()
          fd.append('model', selectedModel.value)
          fd.append('prompt', (stylePreset.value.prefix + task.prompt))
          if (sizeString.value !== 'auto') fd.append('size', sizeString.value)
          if (qualityString.value) fd.append('quality', qualityString.value)
          fd.append('output_format', outputFormat.value)
          for (const f of task.referenceFiles) {
            const c = await compressImageIfNeeded(f)
            fd.append('image[]', c, c.name)
          }
          const resp = await api.post('/v1/images/edits', fd, {
            signal: abortController!.signal,
            headers: { 'Content-Type': 'multipart/form-data' },
          })
          data = resp.data
        } else {
          const body: Record<string, any> = {
            model: selectedModel.value,
            prompt: (stylePreset.value.prefix + task.prompt),
            n: 1,
            output_format: outputFormat.value,
          }
          if (sizeString.value !== 'auto') body.size = sizeString.value
          if (qualityString.value) body.quality = qualityString.value
          const resp = await api.post('/v1/images/generations', body, { signal: abortController!.signal })
          data = resp.data
        }
        const url = extractImageUrl(data.data?.[0] || {}, outputFormat.value)
        task.result = url
        task.status = 'success'
        task.elapsed = Math.round((Date.now() - start) / 1000)
        await saveImage({
          id: crypto.randomUUID(),
          prompt: stylePreset.value.prefix + task.prompt,
          model: selectedModel.value,
          size: sizeString.value,
          mode: 'batch',
          imageUrl: url,
          groupName: selectedGroup.value?.group_name || '',
          style: stylePreset.value.label,
          createdAt: Date.now(),
          batchId: task.id,
        })
      } catch (e: any) {
        task.status = 'failed'
        task.error = extractApiError(e) || '失败'
        task.elapsed = Math.round((Date.now() - start) / 1000)
      }
    })
    await runWithConcurrency(tasks, 3)
  }

  function addScene() {
    const idx = storyScenes.value.length
    storyScenes.value.push({
      id: crypto.randomUUID(),
      index: idx + 1,
      prompt: '',
      status: 'pending',
    })
  }

  function removeScene(id: string) {
    storyScenes.value = storyScenes.value.filter(s => s.id !== id)
    storyScenes.value.forEach((s, i) => { s.index = i + 1 })
  }

  async function runStoryboard() {
    if (!groupApiKey.value) return
    const pending = storyScenes.value.filter(s => s.status !== 'success')
    pending.forEach(s => { s.status = 'pending'; s.error = undefined; s.result = undefined })
    abortController = new AbortController()
    const tasks = pending.map(scene => async () => {
      scene.status = 'running'
      try {
        const api = createAxios()
        const fd = new FormData()
        fd.append('model', selectedModel.value)
        fd.append('prompt', (stylePreset.value.prefix + scene.prompt))
        if (sizeString.value !== 'auto') fd.append('size', sizeString.value)
        if (qualityString.value) fd.append('quality', qualityString.value)
        fd.append('output_format', outputFormat.value)
        for (const f of storyCharacterFiles.value) {
          const c = await compressImageIfNeeded(f)
          fd.append('image[]', c, c.name)
        }
        if (scene.referenceFile) {
          const c = await compressImageIfNeeded(scene.referenceFile)
          fd.append('image[]', c, c.name)
        }
        const useEdits = storyCharacterFiles.value.length > 0 || scene.referenceFile
        const endpoint = useEdits ? '/v1/images/edits' : '/v1/images/generations'
        let resp: any
        if (useEdits) {
          resp = await api.post(endpoint, fd, {
            signal: abortController!.signal,
            headers: { 'Content-Type': 'multipart/form-data' },
          })
        } else {
          resp = await api.post(endpoint, {
            model: selectedModel.value,
            prompt: (stylePreset.value.prefix + scene.prompt),
            n: 1,
            output_format: outputFormat.value,
            ...(sizeString.value !== 'auto' ? { size: sizeString.value } : {}),
            ...(qualityString.value ? { quality: qualityString.value } : {}),
          }, { signal: abortController!.signal })
        }
        const url = extractImageUrl(resp.data.data?.[0] || {}, outputFormat.value)
        scene.result = url
        scene.status = 'success'
        await saveImage({
          id: crypto.randomUUID(),
          prompt: stylePreset.value.prefix + scene.prompt,
          model: selectedModel.value,
          size: sizeString.value,
          mode: 'storyboard',
          imageUrl: url,
          groupName: selectedGroup.value?.group_name || '',
          style: stylePreset.value.label,
          createdAt: Date.now(),
          storyboardId: storyScenes.value[0]?.id,
          sceneIndex: scene.index,
        })
      } catch (e: any) {
        scene.status = 'failed'
        scene.error = extractApiError(e) || '失败'
      }
    })
    await runWithConcurrency(tasks, 3)
  }

  const batchProgress = computed(() => {
    const total = batchTasks.value.length
    const done = batchTasks.value.filter(t => t.status === 'success' || t.status === 'failed').length
    return { total, done }
  })

  const storyProgress = computed(() => {
    const total = storyScenes.value.length
    const done = storyScenes.value.filter(s => s.status === 'success' || s.status === 'failed').length
    return { total, done }
  })

  return {
    activeTab, loading, loadingGroups, error, elapsed,
    groups: imageGroups, selectedGroupId, selectedGroup, selectedModel, imageModels, groupApiKey,
    resolutionTier, selectedRatio, customW, customH, outputFormat, outputCompression,
    stylePreset, imageCount, prompt, fullPrompt, sizeString,
    maskFile, multiFiles,
    resultUrls,
    batchTasks, batchProgress,
    storyCharacterFiles, storyScenes, storyProgress,
    loadGroupsAndKeys, generate, editImage, abort,
    addBatchTask, removeBatchTask, runBatchTasks,
    addScene, removeScene, runStoryboard,
  }
}
