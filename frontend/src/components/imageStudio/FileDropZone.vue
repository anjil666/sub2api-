<template>
  <div @dragover.prevent="dragging = true" @dragleave="dragging = false" @drop.prevent="onDrop"
    :class="[dragging ? 'border-primary-400 bg-primary-50 dark:bg-primary-900/20' : 'border-gray-200 dark:border-dark-600', compact ? 'p-2' : 'p-4', 'rounded-xl border-2 border-dashed transition-colors']">
    <div v-if="!files.length" class="text-center">
      <label class="cursor-pointer text-sm text-gray-500 hover:text-primary-600 dark:text-gray-400 dark:hover:text-primary-400">
        拖拽或点击上传
        <input type="file" :accept="accept" :multiple="max > 1" @change="onSelect" class="hidden" />
      </label>
    </div>
    <div v-else class="flex flex-wrap gap-1.5">
      <div v-for="(_f, i) in files" :key="i" class="group relative">
        <img :src="previewUrls[i]" class="h-12 w-12 rounded-lg object-cover" />
        <button @click="remove(i)" class="absolute -right-1 -top-1 rounded-full bg-red-500 p-0.5 text-white opacity-0 transition-opacity group-hover:opacity-100">
          <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
        </button>
      </div>
      <label v-if="files.length < max" class="flex h-12 w-12 cursor-pointer items-center justify-center rounded-lg border-2 border-dashed border-gray-200 text-gray-400 hover:border-primary-400 dark:border-dark-600">
        <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
        <input type="file" :accept="accept" :multiple="max > 1" @change="onSelect" class="hidden" />
      </label>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'

const props = defineProps<{ files: File[]; max: number; accept: string; compact?: boolean }>()
const emit = defineEmits<{ update: [files: File[]] }>()
const dragging = ref(false)

const previewUrls = ref<string[]>([])

watch(() => props.files, (newFiles) => {
  previewUrls.value.forEach(u => URL.revokeObjectURL(u))
  previewUrls.value = newFiles.map(f => URL.createObjectURL(f))
}, { immediate: true })

function onDrop(e: DragEvent) {
  dragging.value = false
  const dt = e.dataTransfer
  if (!dt) return
  const added = Array.from(dt.files).filter(f => f.type.startsWith('image/'))
  emit('update', [...props.files, ...added].slice(0, props.max))
}

function onSelect(e: Event) {
  const input = e.target as HTMLInputElement
  if (!input.files) return
  const added = Array.from(input.files)
  emit('update', [...props.files, ...added].slice(0, props.max))
  input.value = ''
}

function remove(i: number) {
  const copy = [...props.files]
  copy.splice(i, 1)
  emit('update', copy)
}
</script>
