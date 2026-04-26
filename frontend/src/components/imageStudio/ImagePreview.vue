<template>
  <Teleport to="body">
    <div v-if="visible" class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm" @click.self="close">
      <button @click="close" class="absolute right-4 top-4 rounded-full bg-white/10 p-2 text-white hover:bg-white/20">
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
      </button>
      <img :src="src" class="max-h-[90vh] max-w-[90vw] rounded-lg object-contain" @wheel.prevent="onWheel" :style="{ transform: `scale(${scale})` }" />
    </div>
  </Teleport>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue'
const props = defineProps<{ visible: boolean; src: string }>()
const emit = defineEmits<{ 'update:visible': [v: boolean] }>()
const scale = ref(1)
function close() { emit('update:visible', false) }
function onWheel(e: WheelEvent) {
  scale.value = Math.max(0.5, Math.min(5, scale.value + (e.deltaY > 0 ? -0.15 : 0.15)))
}
watch(() => props.visible, v => { if (v) scale.value = 1 })
function onKey(e: KeyboardEvent) { if (e.key === 'Escape') close() }
if (typeof window !== 'undefined') window.addEventListener('keydown', onKey)
</script>
