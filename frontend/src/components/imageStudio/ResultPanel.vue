<template>
  <div class="space-y-3">
    <div v-if="loading" class="flex flex-col items-center justify-center py-8 text-gray-500 dark:text-gray-400">
      <svg class="mb-2 h-8 w-8 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
      <span class="text-sm">生成中... {{ elapsed }}s</span>
    </div>
    <template v-else>
      <div v-if="!urls.length" class="py-8 text-center text-sm text-gray-400">结果将在这里显示</div>
      <template v-else>
        <div v-if="model || size" class="flex flex-wrap gap-2 text-xs text-gray-500 dark:text-gray-400">
          <span v-if="model" class="rounded-full bg-gray-100 px-2 py-0.5 dark:bg-dark-700">{{ model }}</span>
          <span v-if="size" class="rounded-full bg-gray-100 px-2 py-0.5 dark:bg-dark-700">{{ size }}</span>
        </div>
        <div v-for="(url, i) in urls" :key="i" class="group relative">
          <img :src="url" class="w-full rounded-xl" loading="lazy" />
          <button @click="downloadImage(url, i)" title="下载图片"
            class="absolute right-2 top-2 rounded-lg bg-black/50 p-1.5 text-white opacity-0 backdrop-blur-sm transition-opacity group-hover:opacity-100 hover:bg-black/70">
            <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"/></svg>
          </button>
        </div>
      </template>
    </template>
  </div>
</template>

<script setup lang="ts">
defineProps<{ urls: string[]; loading: boolean; elapsed: number; model?: string; size?: string }>()

async function downloadImage(url: string, index: number) {
  const ext = url.match(/\.(png|jpe?g|webp)/i)?.[1] || 'png'
  const filename = `image_${index + 1}.${ext}`
  try {
    const resp = await fetch(url)
    if (!resp.ok) throw new Error('fetch failed')
    const blob = await resp.blob()
    const blobUrl = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = blobUrl
    a.download = filename
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(blobUrl)
  } catch {
    window.open(url, '_blank')
  }
}
</script>
