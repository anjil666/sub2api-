<template>
  <div class="card space-y-4 p-4">
    <div class="flex flex-wrap items-center gap-3">
      <span class="text-base font-semibold text-gray-700 dark:text-gray-300">最近任务</span>
      <select v-model="filterMode" class="input !w-auto text-sm">
        <option value="">全部</option>
        <option value="generation">文生图</option>
        <option value="multi-edit">图生图</option>
        <option value="batch">批量</option>
        <option value="storyboard">分镜</option>
      </select>
      <label class="flex items-center gap-1 text-sm text-gray-600 dark:text-gray-400">
        <input type="checkbox" :checked="allSelected" @change="toggleAll" class="rounded" /> 全选
      </label>
      <button @click="downloadSelected" :disabled="!selectedIds.size" class="btn btn-primary text-sm">下载选中 ({{ selectedIds.size }})</button>
      <button @click="deleteSelected" :disabled="!selectedIds.size" class="btn btn-secondary !border-red-300 !text-red-600 text-sm">删除选中</button>
      <button @click="load(true)" class="btn btn-secondary text-sm">刷新</button>
      <span class="ml-auto text-xs text-gray-500">共 {{ images.length }} 张</span>
    </div>
    <div v-if="!images.length" class="py-8 text-center text-sm text-gray-400">暂无历史记录</div>
    <div v-else class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
      <div v-for="img in images" :key="img.id"
        class="group relative overflow-hidden rounded-xl border border-gray-100 transition-shadow hover:shadow-card-hover dark:border-dark-700/50">
        <div class="absolute left-2 top-2 z-10">
          <input type="checkbox" :checked="selectedIds.has(img.id)" class="cursor-pointer rounded" @click.stop="toggleSelect(img.id)" />
        </div>
        <div class="relative">
          <img :src="img.imageUrl" class="aspect-square w-full cursor-pointer object-cover" loading="lazy" @click="previewSrc = img.imageUrl; previewVisible = true" />
          <div class="pointer-events-none absolute inset-0 flex items-center justify-center bg-black/0 transition-colors group-hover:bg-black/30">
            <svg class="h-8 w-8 text-white opacity-0 drop-shadow transition-opacity group-hover:opacity-80" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0zM10 7v3m0 0v3m0-3h3m-3 0H7"/></svg>
          </div>
        </div>
        <div class="p-2">
          <div class="truncate text-xs text-gray-700 dark:text-gray-300">{{ img.prompt }}</div>
          <div class="flex items-center justify-between text-[10px] text-gray-400">
            <span>{{ img.model }}</span>
            <span>{{ formatDate(img.createdAt) }}</span>
          </div>
        </div>
      </div>
    </div>
    <button v-if="hasMore" @click="loadMore" class="btn btn-secondary mx-auto block text-sm">加载更多</button>
    <ImagePreview v-model:visible="previewVisible" :src="previewSrc" />
  </div>
</template>
<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { getImages, deleteImages, type ImageRecord } from '@/utils/imageDB'
import ImagePreview from './ImagePreview.vue'

const filterMode = ref<string>('')
const images = ref<ImageRecord[]>([])
const selectedIds = ref<Set<string>>(new Set())
const page = ref(0)
const pageSize = 50
const hasMore = ref(true)
const previewVisible = ref(false)
const previewSrc = ref('')

const allSelected = computed(() => images.value.length > 0 && images.value.every(i => selectedIds.value.has(i.id)))

async function load(reset = false) {
  if (reset) { page.value = 0; images.value = []; hasMore.value = true }
  const mode = filterMode.value || undefined
  const items = await getImages(mode as any, pageSize, page.value * pageSize)
  if (reset) images.value = items; else images.value.push(...items)
  hasMore.value = items.length === pageSize
}

function loadMore() { page.value++; load() }

function toggleSelect(id: string) {
  const s = new Set(selectedIds.value)
  s.has(id) ? s.delete(id) : s.add(id)
  selectedIds.value = s
}

function toggleAll() {
  if (allSelected.value) selectedIds.value = new Set()
  else selectedIds.value = new Set(images.value.map(i => i.id))
}

async function deleteSelected() {
  if (!selectedIds.value.size) return
  await deleteImages([...selectedIds.value])
  selectedIds.value = new Set()
  load(true)
}

function toDownloadBlobUrl(url: string): string {
  if (url.startsWith('data:')) {
    const [, b64] = url.split(',')
    const bin = atob(b64)
    const arr = new Uint8Array(bin.length)
    for (let i = 0; i < bin.length; i++) arr[i] = bin.charCodeAt(i)
    return URL.createObjectURL(new Blob([arr], { type: 'application/octet-stream' }))
  }
  if (url.startsWith('blob:')) return url
  return url
}

function sanitizeFilename(prompt: string): string {
  const clean = prompt.replace(/[\\/:*?"<>|\n\r]/g, '').trim()
  return clean.slice(0, 30) || 'image'
}

function downloadSelected() {
  const sel = images.value.filter(i => selectedIds.value.has(i.id))
  sel.forEach((img, idx) => {
    setTimeout(() => {
      const blobUrl = toDownloadBlobUrl(img.imageUrl)
      const a = document.createElement('a')
      a.href = blobUrl; a.download = `${sanitizeFilename(img.prompt)}.png`
      a.style.display = 'none'
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      if (blobUrl !== img.imageUrl) setTimeout(() => URL.revokeObjectURL(blobUrl), 2000)
    }, idx * 300)
  })
}

function formatDate(ts: number) {
  const d = new Date(ts)
  return `${d.getMonth() + 1}/${d.getDate()} ${d.getHours()}:${String(d.getMinutes()).padStart(2, '0')}`
}

watch(filterMode, () => load(true))
onMounted(() => {
  load(true)
  window.addEventListener('image-studio-saved', onSaved)
})
onUnmounted(() => window.removeEventListener('image-studio-saved', onSaved))
function onSaved() { load(true) }
</script>
