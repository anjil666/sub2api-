<template>
  <div class="space-y-3">
    <div v-if="loading" class="flex flex-col items-center justify-center py-8 text-gray-500">
      <svg class="mb-2 h-8 w-8 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
      <span class="text-sm">生成中... {{ elapsed }}s</span>
    </div>
    <template v-else>
      <div v-if="!urls.length" class="py-8 text-center text-sm text-gray-400">结果将在这里显示</div>
      <div v-for="(url, i) in urls" :key="i" class="group relative">
        <img :src="url" class="w-full rounded-lg" loading="lazy" />
        <a :href="url" download :title="'下载图片'" class="absolute right-2 top-2 rounded bg-black/50 p-1.5 text-white opacity-0 transition-opacity group-hover:opacity-100">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"/></svg>
        </a>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
defineProps<{ urls: string[]; loading: boolean; elapsed: number }>()
</script>
