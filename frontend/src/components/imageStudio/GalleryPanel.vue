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
        class="group relative cursor-pointer overflow-hidden rounded-xl border border-gray-100 transition-shadow hover:shadow-card-hover dark:border-dark-700/50"
        @click="toggleSelect(img.id)">
        <div class="absolute left-2 top-2 z-10">
          <input type="checkbox" :checked="selectedIds.has(img.id)" @click.stop class="rounded" @change="toggleSelect(img.id)" />
        </div>
        <img :src="img.imageUrl" class="aspect-square w-full object-cover" loading="lazy" />
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
  </div>
</template>
<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { getImages, deleteImages, type ImageRecord } from '@/utils/imageDB'

const filterMode = ref<string>('')
const images = ref<ImageRecord[]>([])
const selectedIds = ref<Set<string>>(new Set())
const page = ref(0)
const pageSize = 50
const hasMore = ref(true)

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

async function downloadSelected() {
  const sel = images.value.filter(i => selectedIds.value.has(i.id))
  if (sel.length === 1) {
    try {
      const resp = await fetch(sel[0].imageUrl)
      const blob = await resp.blob()
      const blobUrl = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = blobUrl; a.download = `image_${sel[0].id}.png`; a.click()
      URL.revokeObjectURL(blobUrl)
    } catch { window.open(sel[0].imageUrl, '_blank') }
    return
  }
  try {
    const { default: JSZip } = await import('jszip')
    const zip = new JSZip()
    await Promise.all(sel.map(async (img, i) => {
      try {
        const resp = await fetch(img.imageUrl)
        const blob = await resp.blob()
        zip.file(`image_${i + 1}.${blob.type.split('/')[1] || 'png'}`, blob)
      } catch { /* skip */ }
    }))
    const content = await zip.generateAsync({ type: 'blob' })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(content); a.download = 'images.zip'; a.click()
    URL.revokeObjectURL(a.href)
  } catch { alert('下载失败，请重试') }
}

function formatDate(ts: number) {
  const d = new Date(ts)
  return `${d.getMonth() + 1}/${d.getDate()} ${d.getHours()}:${String(d.getMinutes()).padStart(2, '0')}`
}

watch(filterMode, () => load(true))
onMounted(() => load(true))
</script>
