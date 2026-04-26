<template>
  <div class="space-y-2">
    <div class="flex items-center justify-between">
      <span class="text-sm font-semibold text-primary-600 dark:text-primary-400">尺寸和质量</span>
      <span class="text-[11px] text-gray-500 dark:text-gray-400">{{ currentLabel }}</span>
    </div>
    <div class="grid grid-cols-3 gap-1.5">
      <button v-for="p in presets" :key="p.tier + p.ratio.label" @click="selectPreset(p)"
        :class="[isActive(p) ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300' : 'border-gray-200 bg-white text-gray-700 hover:border-gray-300 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300', 'flex flex-col items-center rounded-lg border px-1.5 py-1 text-xs transition-colors']">
        <span class="text-[9px] font-semibold uppercase opacity-50">{{ p.tier }}</span>
        <span class="font-medium leading-tight">{{ p.pixels }}</span>
        <span class="text-[10px] opacity-70">{{ p.ratio.label }}</span>
      </button>
    </div>
    <div class="grid grid-cols-2 gap-1.5">
      <button @click="$emit('update:resolutionTier', 'AUTO')"
        :class="[resolutionTier === 'AUTO' ? 'bg-gray-800 text-white dark:bg-gray-200 dark:text-gray-900' : 'bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-300', 'rounded-lg px-2 py-1.5 text-xs font-medium transition-colors']">
        自动
      </button>
      <button @click="toggleCustom"
        :class="[resolutionTier === 'custom' ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/30' : 'border-gray-200 dark:border-dark-600', 'rounded-lg border px-2 py-1.5 text-xs text-gray-700 transition-colors dark:text-gray-300']">
        自定义
      </button>
    </div>
    <div v-if="resolutionTier === 'custom'" class="grid grid-cols-2 gap-2">
      <div>
        <label class="text-[10px] text-gray-500 dark:text-gray-400">宽</label>
        <input type="number" :value="customW" @input="$emit('update:customW', clamp($event))" step="16" min="256" max="3840" class="input !py-1 !text-xs" />
      </div>
      <div>
        <label class="text-[10px] text-gray-500 dark:text-gray-400">高</label>
        <input type="number" :value="customH" @input="$emit('update:customH', clamp($event))" step="16" min="256" max="3840" class="input !py-1 !text-xs" />
      </div>
    </div>
    <div class="grid grid-cols-2 gap-2">
      <div>
        <label class="text-[10px] font-medium text-gray-500 dark:text-gray-400">格式</label>
        <select :value="outputFormat" @change="$emit('update:outputFormat', ($event.target as HTMLSelectElement).value)" class="input !py-1 !text-xs">
          <option value="png">PNG</option><option value="jpeg">JPEG</option><option value="webp">WebP</option>
        </select>
      </div>
      <div>
        <label class="text-[10px] font-medium text-gray-500 dark:text-gray-400">风格</label>
        <select :value="stylePreset.label" @change="onStyleChange" class="input !py-1 !text-xs">
          <option v-for="s in styles" :key="s.label" :value="s.label">{{ s.label }}</option>
        </select>
      </div>
    </div>
    <div v-if="outputFormat !== 'png'">
      <label class="text-[10px] text-gray-500 dark:text-gray-400">压缩 {{ outputCompression }}%</label>
      <input type="range" :value="outputCompression" @input="$emit('update:outputCompression', +($event.target as HTMLInputElement).value)" min="10" max="100" class="w-full accent-primary-500" />
    </div>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { ASPECT_RATIOS, STYLE_PRESETS, computeSize, type ResolutionTier, type StylePreset, type AspectRatio } from '@/composables/useImageGeneration'

const props = defineProps<{
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

interface Preset { tier: ResolutionTier; ratio: AspectRatio; pixels: string }

const presets = computed<Preset[]>(() => {
  const items: Preset[] = []
  for (const tier of ['1K', '2K', '4K'] as ResolutionTier[]) {
    for (const ratio of [ASPECT_RATIOS[0], ASPECT_RATIOS[1]]) {
      items.push({ tier, ratio, pixels: computeSize(tier, ratio.w, ratio.h) })
    }
  }
  return items
})

const styles = STYLE_PRESETS

const currentLabel = computed(() => {
  if (props.resolutionTier === 'AUTO') return '自动'
  if (props.resolutionTier === 'custom') return `${props.customW}x${props.customH}`
  return computeSize(props.resolutionTier, props.selectedRatio.w, props.selectedRatio.h)
})

function isActive(p: Preset) {
  return props.resolutionTier === p.tier && props.selectedRatio.label === p.ratio.label
}

function selectPreset(p: Preset) {
  emit('update:resolutionTier', p.tier)
  emit('update:selectedRatio', p.ratio)
}

function toggleCustom() {
  emit('update:resolutionTier', props.resolutionTier === 'custom' ? 'AUTO' : 'custom')
}

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
