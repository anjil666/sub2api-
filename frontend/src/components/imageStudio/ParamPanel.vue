<template>
  <div :class="compact ? 'flex flex-wrap gap-3' : 'space-y-3'">
    <div :class="compact ? 'w-auto' : ''">
      <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">分辨率</label>
      <div class="flex flex-wrap gap-1">
        <button v-for="t in tiers" :key="t" @click="$emit('update:resolutionTier', t)"
          :class="[resolutionTier === t ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-700 dark:bg-gray-600 dark:text-gray-300', 'rounded px-2 py-1 text-xs']">{{ t }}</button>
      </div>
    </div>
    <div v-if="resolutionTier !== 'custom'" :class="compact ? 'w-auto' : ''">
      <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">宽高比</label>
      <div class="flex flex-wrap gap-1">
        <button v-for="r in ratios" :key="r.label" @click="$emit('update:selectedRatio', r)"
          :class="[selectedRatio.label === r.label ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-700 dark:bg-gray-600 dark:text-gray-300', 'rounded px-2 py-1 text-xs']">{{ r.label }}</button>
      </div>
    </div>
    <div v-if="resolutionTier === 'custom'" class="flex gap-2">
      <div>
        <label class="mb-1 block text-xs text-gray-600 dark:text-gray-400">宽</label>
        <input type="number" :value="customW" @input="$emit('update:customW', clamp($event))" step="16" min="256" max="3840" class="w-20 rounded border border-gray-300 px-2 py-1 text-xs dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200" />
      </div>
      <div>
        <label class="mb-1 block text-xs text-gray-600 dark:text-gray-400">高</label>
        <input type="number" :value="customH" @input="$emit('update:customH', clamp($event))" step="16" min="256" max="3840" class="w-20 rounded border border-gray-300 px-2 py-1 text-xs dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200" />
      </div>
    </div>
    <div :class="compact ? 'w-auto' : ''">
      <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">输出格式</label>
      <select :value="outputFormat" @change="$emit('update:outputFormat', ($event.target as HTMLSelectElement).value)" class="rounded border border-gray-300 px-2 py-1 text-xs dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200">
        <option value="png">PNG</option><option value="jpeg">JPEG</option><option value="webp">WebP</option>
      </select>
    </div>
    <div v-if="outputFormat !== 'png'" :class="compact ? 'w-auto' : ''">
      <label class="mb-1 block text-xs text-gray-600 dark:text-gray-400">压缩率 {{ outputCompression }}%</label>
      <input type="range" :value="outputCompression" @input="$emit('update:outputCompression', +($event.target as HTMLInputElement).value)" min="10" max="100" class="w-24" />
    </div>
    <div :class="compact ? 'w-auto' : ''">
      <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">风格</label>
      <select :value="stylePreset.label" @change="onStyleChange" class="rounded border border-gray-300 px-2 py-1 text-xs dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200">
        <option v-for="s in styles" :key="s.label" :value="s.label">{{ s.label }}</option>
      </select>
    </div>
    <div v-if="!compact" :class="compact ? 'w-auto' : ''">
      <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">数量</label>
      <input type="number" :value="imageCount" @input="$emit('update:imageCount', Math.max(1, Math.min(4, +($event.target as HTMLInputElement).value)))" min="1" max="4" class="w-16 rounded border border-gray-300 px-2 py-1 text-xs dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { RESOLUTION_TIERS, ASPECT_RATIOS, STYLE_PRESETS, type ResolutionTier, type StylePreset, type AspectRatio } from '@/composables/useImageGeneration'

defineProps<{
  resolutionTier: ResolutionTier
  selectedRatio: AspectRatio
  customW: number
  customH: number
  outputFormat: string
  outputCompression: number
  stylePreset: StylePreset
  imageCount: number
  compact?: boolean
}>()

const emit = defineEmits<{
  'update:resolutionTier': [v: ResolutionTier]
  'update:selectedRatio': [v: AspectRatio]
  'update:customW': [v: number]
  'update:customH': [v: number]
  'update:outputFormat': [v: string]
  'update:outputCompression': [v: number]
  'update:stylePreset': [v: StylePreset]
  'update:imageCount': [v: number]
}>()

const tiers = RESOLUTION_TIERS
const ratios = ASPECT_RATIOS
const styles = STYLE_PRESETS

function clamp(e: Event) {
  let v = +(e.target as HTMLInputElement).value
  v = Math.round(v / 16) * 16
  return Math.max(256, Math.min(3840, v))
}

function onStyleChange(e: Event) {
  const label = (e.target as HTMLSelectElement).value
  const s = styles.find(s => s.label === label)
  if (s) emit('update:stylePreset', s)
}
</script>
