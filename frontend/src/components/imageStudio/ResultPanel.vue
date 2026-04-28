<template>
  <div class="max-h-[70vh] space-y-2 overflow-y-auto">
    <div v-if="!tasks.length && !loading" class="py-4 text-center text-xs text-gray-400">结果将在这里显示</div>
    <div v-for="task in tasks" :key="task.id" class="space-y-1.5 rounded-lg border border-gray-100 p-2 dark:border-dark-600">
      <div class="flex items-center gap-1.5">
        <span :class="taskStatusClass(task.status)" class="rounded-full px-2 py-0.5 text-[10px] font-medium">{{ taskStatusLabel(task.status) }}</span>
        <button v-if="task.status === 'running' || task.status === 'pending'" @click="emit('abort-task', task.id)" class="rounded-full px-1.5 py-0.5 text-[10px] text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20">取消</button>
        <span v-if="task.elapsed != null" class="text-[10px] text-gray-500">{{ task.elapsed }}s</span>
        <span class="ml-auto text-[10px] text-gray-400">{{ task.model }} · {{ task.size }}</span>
      </div>
      <div class="truncate text-[10px] text-gray-500 dark:text-gray-400">{{ task.prompt }}</div>
<!-- PLACEHOLDER_TASK_IMAGES -->
      <div v-if="task.status === 'running'" class="flex items-center justify-center py-3">
        <svg class="h-5 w-5 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
      </div>
      <div v-if="task.urls.length" v-for="(url, i) in task.urls" :key="i" class="group relative cursor-zoom-in" @click="preview(url)">
        <img :src="url" class="max-h-[400px] w-full rounded-xl object-contain" loading="lazy" />
        <button @click.stop="downloadImage(task, i)" title="下载图片"
          class="absolute right-2 top-2 rounded-lg bg-black/50 p-1.5 text-white opacity-0 backdrop-blur-sm transition-opacity group-hover:opacity-100 hover:bg-black/70">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"/></svg>
        </button>
      </div>
      <div v-if="task.error" class="text-[10px] text-red-500">{{ task.error }}</div>
    </div>
    <ImagePreview v-model:visible="previewVisible" :src="previewSrc" />
  </div>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import ImagePreview from './ImagePreview.vue'
import type { GenerationTask } from '@/composables/useImageGeneration'

defineProps<{ tasks: GenerationTask[]; loading: boolean; elapsed: number }>()
const emit = defineEmits<{ 'abort-task': [id: string] }>()
const previewVisible = ref(false)
const previewSrc = ref('')
function preview(url: string) { previewSrc.value = url; previewVisible.value = true }

function taskStatusClass(s: string) {
  return { pending: 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-400', running: 'bg-blue-100 text-blue-700 animate-pulse dark:bg-blue-900/30', success: 'bg-green-100 text-green-700 dark:bg-green-900/30', failed: 'bg-red-100 text-red-700 dark:bg-red-900/30' }[s] || ''
}
function taskStatusLabel(s: string) {
  return { pending: '排队中', running: '生成中', success: '成功', failed: '失败' }[s] || s
}

function triggerAnchorDownload(blobUrl: string, filename: string) {
  const a = document.createElement('a')
  a.href = blobUrl
  a.download = filename
  a.style.display = 'none'
  document.body.appendChild(a)
  a.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, view: window }))
  document.body.removeChild(a)
  setTimeout(() => URL.revokeObjectURL(blobUrl), 5000)
}

function forceDownload(url: string, filename: string) {
  if (url.startsWith('data:')) {
    const commaIdx = url.indexOf(',')
    const b64 = url.slice(commaIdx + 1)
    const bin = atob(b64)
    const arr = new Uint8Array(bin.length)
    for (let i = 0; i < bin.length; i++) arr[i] = bin.charCodeAt(i)
    triggerAnchorDownload(URL.createObjectURL(new Blob([arr], { type: 'application/octet-stream' })), filename)
    return
  }
  fetch(url).then(r => { if (!r.ok) throw new Error(); return r.blob() }).then(blob => {
    triggerAnchorDownload(URL.createObjectURL(new Blob([blob], { type: 'application/octet-stream' })), filename)
  }).catch(() => {
    const token = localStorage.getItem('auth_token')
    fetch(`/v1/user/image-proxy?url=${encodeURIComponent(url)}`, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    }).then(r => { if (!r.ok) throw new Error(); return r.blob() }).then(blob => {
      triggerAnchorDownload(URL.createObjectURL(new Blob([blob], { type: 'application/octet-stream' })), filename)
    }).catch(() => {})
  })
}

function downloadImage(task: GenerationTask, index: number) {
  const url = task.urls[index]
  if (!url) return
  forceDownload(url, `image_${index + 1}.png`)
}
</script>
