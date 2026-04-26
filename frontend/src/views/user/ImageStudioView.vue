<template>
  <AppLayout>
    <div class="space-y-4">
      <!-- Header -->
      <div class="card flex flex-wrap items-center gap-3 p-4">
        <div class="flex items-center gap-2">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">分组</label>
          <select v-model="selectedGroupId" class="input !w-auto">
            <option v-for="g in groups" :key="g.group_id" :value="g.group_id">
              {{ cleanGroupName(g.group_name) }}
              <template v-if="g.billing_display"> · {{ g.billing_display }}</template>
              <template v-else-if="g.image_price_1k"> · ${{ g.image_price_1k }}/次</template>
            </option>
          </select>
          <span v-if="!groupApiKey && selectedGroupId" class="text-xs text-amber-500">无可用密钥</span>
          <button @click="loadGroupsAndKeys" class="btn-secondary !rounded-lg !px-2 !py-1.5" title="刷新">
            <svg class="h-4 w-4" :class="{ 'animate-spin': loadingGroups }" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
          </button>
        </div>
        <div class="flex items-center gap-2">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">模型</label>
          <select v-model="selectedModel" class="input !w-auto">
            <option v-for="m in imageModels" :key="m.model_name" :value="m.model_name">{{ m.model_name }}</option>
          </select>
        </div>
        <div v-if="error" class="ml-auto text-sm text-red-500">{{ error }}</div>
      </div>
<!-- PLACEHOLDER_HINT_TABS -->
      <!-- Hint -->
      <div v-if="!loadingGroups && !groups.length" class="rounded-xl border border-amber-200 bg-amber-50 px-4 py-2.5 text-sm text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400">
        提示：请先在「API密钥」页面创建密钥并绑定做图分组，分组才会出现在这里
      </div>

      <!-- Tabs -->
      <div class="flex gap-1 rounded-xl bg-gray-100 p-1 dark:bg-dark-700">
        <button v-for="tab in tabs" :key="tab.key" @click="activeTab = tab.key"
          :class="[activeTab === tab.key ? 'bg-white shadow dark:bg-dark-600 dark:text-white' : 'text-gray-600 hover:text-gray-900 dark:text-gray-400', 'rounded-lg px-4 py-2 text-sm font-medium transition-colors']">
          {{ tab.label }}
        </button>
      </div>

      <!-- GENERATION TAB -->
      <div v-if="activeTab === 'generation'" class="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <div class="card p-4">
          <ParamPanel v-bind="paramBindings" @update:resolutionTier="resolutionTier = $event" @update:selectedRatio="selectedRatio = $event"
            @update:customW="customW = $event" @update:customH="customH = $event" @update:outputFormat="outputFormat = $event as any"
            @update:outputCompression="outputCompression = $event" @update:stylePreset="stylePreset = $event" @update:imageCount="imageCount = $event" />
        </div>
        <div class="card flex flex-col gap-3 p-4">
          <div class="text-base font-semibold text-emerald-600 dark:text-emerald-400">创意描述</div>
          <textarea v-model="prompt" rows="8" placeholder="描述你想生成的图片..." class="input flex-1" />
          <div class="flex items-center gap-2">
            <label class="text-xs font-medium text-gray-600 dark:text-gray-400">数量</label>
            <input type="number" v-model.number="imageCount" min="1" max="4" class="input !w-16" />
          </div>
          <button @click="generate" :disabled="loading || !prompt.trim() || !groupApiKey" class="btn btn-primary">
            {{ loading ? `生成中... ${elapsed}s` : '开始生成' }}
          </button>
          <button v-if="loading" @click="abort" class="btn btn-secondary !border-red-300 !text-red-600 hover:!bg-red-50">取消</button>
        </div>
        <div class="card p-4">
          <div class="mb-3 text-base font-semibold text-blue-600 dark:text-blue-400">生成结果</div>
          <ResultPanel :urls="resultUrls" :loading="loading" :elapsed="elapsed" :model="selectedModel" :size="sizeString" />
        </div>
      </div>
<!-- PLACEHOLDER_EDIT_TAB -->
      <!-- IMAGE EDIT TAB (merged single+multi) -->
      <div v-if="activeTab === 'edit'" class="grid grid-cols-1 gap-4 lg:grid-cols-4">
        <div class="card space-y-3 p-4">
          <div class="text-base font-semibold text-orange-600 dark:text-orange-400">上传图片</div>
          <label class="block text-xs font-medium text-gray-600 dark:text-gray-400">参考图 (0~5张)</label>
          <FileDropZone :files="multiFiles" :max="5" accept="image/*" @update="multiFiles = $event" />
          <label class="block text-xs font-medium text-gray-600 dark:text-gray-400">遮罩图 (可选)</label>
          <FileDropZone :files="maskFile ? [maskFile] : []" :max="1" accept="image/*" @update="maskFile = $event[0] || null" />
        </div>
        <div class="card p-4">
          <ParamPanel v-bind="paramBindings" @update:resolutionTier="resolutionTier = $event" @update:selectedRatio="selectedRatio = $event"
            @update:customW="customW = $event" @update:customH="customH = $event" @update:outputFormat="outputFormat = $event as any"
            @update:outputCompression="outputCompression = $event" @update:stylePreset="stylePreset = $event" @update:imageCount="imageCount = $event" />
        </div>
        <div class="card flex flex-col gap-3 p-4">
          <div class="text-base font-semibold text-emerald-600 dark:text-emerald-400">编辑描述</div>
          <textarea v-model="prompt" rows="6" placeholder="描述编辑内容..." class="input flex-1" />
          <div class="flex items-center gap-2">
            <label class="text-xs font-medium text-gray-600 dark:text-gray-400">数量</label>
            <input type="number" v-model.number="imageCount" min="1" max="4" class="input !w-16" />
          </div>
          <button @click="editImage" :disabled="loading || !prompt.trim() || !groupApiKey" class="btn btn-primary">
            {{ loading ? `编辑中... ${elapsed}s` : '开始编辑' }}
          </button>
          <button v-if="loading" @click="abort" class="btn btn-secondary !border-red-300 !text-red-600 hover:!bg-red-50">取消</button>
        </div>
        <div class="card p-4">
          <div class="mb-3 text-base font-semibold text-blue-600 dark:text-blue-400">编辑结果</div>
          <ResultPanel :urls="resultUrls" :loading="loading" :elapsed="elapsed" :model="selectedModel" :size="sizeString" />
        </div>
      </div>
<!-- PLACEHOLDER_BATCH_TAB -->
      <!-- BATCH TAB -->
      <div v-if="activeTab === 'batch'" class="space-y-4">
        <div class="card p-4">
          <div class="mb-3 flex items-center justify-between">
            <div class="text-base font-semibold text-purple-600 dark:text-purple-400">批量任务 <span class="text-sm font-normal text-gray-500">{{ batchProgress.done }}/{{ batchProgress.total }}</span></div>
            <div class="flex gap-2">
              <button @click="addBatchTask" class="btn btn-secondary text-sm">添加任务</button>
              <button @click="runBatchTasks" :disabled="!batchTasks.length || !groupApiKey" class="btn btn-primary text-sm">开始全部</button>
            </div>
          </div>
          <ParamPanel v-bind="paramBindings" @update:resolutionTier="resolutionTier = $event" @update:selectedRatio="selectedRatio = $event"
            @update:customW="customW = $event" @update:customH="customH = $event" @update:outputFormat="outputFormat = $event as any"
            @update:outputCompression="outputCompression = $event" @update:stylePreset="stylePreset = $event" @update:imageCount="imageCount = $event" />
        </div>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          <div v-for="task in batchTasks" :key="task.id" class="card space-y-2 p-4">
            <div class="flex items-center justify-between">
              <span :class="statusClass(task.status)" class="rounded-full px-2.5 py-0.5 text-xs font-medium">{{ statusLabel(task.status) }}</span>
              <div class="flex items-center gap-2">
                <span v-if="task.elapsed" class="text-xs text-gray-500">{{ task.elapsed }}s</span>
                <button @click="removeBatchTask(task.id)" class="text-xs text-red-500 hover:text-red-700">删除</button>
              </div>
            </div>
            <textarea v-model="task.prompt" rows="2" placeholder="任务提示词..." class="input" />
            <FileDropZone :files="task.referenceFiles" :max="5" accept="image/*" @update="task.referenceFiles = $event" :compact="true" />
            <img v-if="task.result" :src="task.result" class="w-full rounded-xl object-cover" />
            <div v-if="task.error" class="text-xs text-red-500">{{ task.error }}</div>
          </div>
        </div>
      </div>
<!-- PLACEHOLDER_STORY_TAB -->
      <!-- STORYBOARD TAB -->
      <div v-if="activeTab === 'storyboard'" class="space-y-4">
        <div class="card p-4">
          <div class="mb-3 flex items-center justify-between">
            <div class="text-base font-semibold text-rose-600 dark:text-rose-400">分镜创作 <span class="text-sm font-normal text-gray-500">第 {{ storyProgress.done }}/{{ storyProgress.total }} 幕</span></div>
            <div class="flex gap-2">
              <button @click="addScene" class="btn btn-secondary text-sm">添加场景</button>
              <button @click="runStoryboard" :disabled="!storyScenes.length || !groupApiKey" class="btn btn-primary text-sm">生成全部</button>
            </div>
          </div>
          <ParamPanel v-bind="paramBindings" @update:resolutionTier="resolutionTier = $event" @update:selectedRatio="selectedRatio = $event"
            @update:customW="customW = $event" @update:customH="customH = $event" @update:outputFormat="outputFormat = $event as any"
            @update:outputCompression="outputCompression = $event" @update:stylePreset="stylePreset = $event" @update:imageCount="imageCount = $event" />
        </div>
        <div class="card space-y-2 p-4">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">角色参考图 (最多5张，所有场景共用)</label>
          <FileDropZone :files="storyCharacterFiles" :max="5" accept="image/*" @update="storyCharacterFiles = $event" />
        </div>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          <div v-for="scene in storyScenes" :key="scene.id" class="card space-y-2 p-4">
            <div class="flex items-center justify-between">
              <span class="text-sm font-medium">第 {{ scene.index }} 幕</span>
              <div class="flex items-center gap-2">
                <span :class="statusClass(scene.status)" class="rounded-full px-2.5 py-0.5 text-xs font-medium">{{ statusLabel(scene.status) }}</span>
                <button @click="removeScene(scene.id)" class="text-xs text-red-500 hover:text-red-700">删除</button>
              </div>
            </div>
            <textarea v-model="scene.prompt" rows="2" placeholder="场景描述..." class="input" />
            <img v-if="scene.result" :src="scene.result" class="w-full rounded-xl object-cover" />
            <div v-if="scene.error" class="text-xs text-red-500">{{ scene.error }}</div>
          </div>
        </div>
      </div>

      <!-- RECENT TASKS (bottom) -->
      <GalleryPanel />
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { computed, onMounted } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useImageGeneration, type StudioTab } from '@/composables/useImageGeneration'
import { cleanGroupName } from '@/utils/format'
import ParamPanel from '@/components/imageStudio/ParamPanel.vue'
import ResultPanel from '@/components/imageStudio/ResultPanel.vue'
import FileDropZone from '@/components/imageStudio/FileDropZone.vue'
import GalleryPanel from '@/components/imageStudio/GalleryPanel.vue'

const {
  activeTab, loading, loadingGroups, error, elapsed,
  groups, selectedGroupId, selectedModel, imageModels, groupApiKey,
  resolutionTier, selectedRatio, customW, customH, outputFormat, outputCompression,
  stylePreset, imageCount, prompt, sizeString,
  maskFile, multiFiles,
  resultUrls,
  batchTasks, batchProgress,
  storyCharacterFiles, storyScenes, storyProgress,
  loadGroupsAndKeys, generate, editImage, abort,
  addBatchTask, removeBatchTask, runBatchTasks,
  addScene, removeScene, runStoryboard,
} = useImageGeneration()

const tabs: { key: StudioTab; label: string }[] = [
  { key: 'generation', label: '文生图' },
  { key: 'edit', label: '图生图' },
  { key: 'batch', label: '批量任务' },
  { key: 'storyboard', label: '分镜创作' },
]

const paramBindings = computed(() => ({
  resolutionTier: resolutionTier.value,
  selectedRatio: selectedRatio.value,
  customW: customW.value,
  customH: customH.value,
  outputFormat: outputFormat.value,
  outputCompression: outputCompression.value,
  stylePreset: stylePreset.value,
  imageCount: imageCount.value,
}))

function statusClass(s: string) {
  return { pending: 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-400', running: 'bg-blue-100 text-blue-700 animate-pulse dark:bg-blue-900/30', success: 'bg-green-100 text-green-700 dark:bg-green-900/30', failed: 'bg-red-100 text-red-700 dark:bg-red-900/30' }[s] || ''
}
function statusLabel(s: string) {
  return { pending: '等待', running: '生成中', success: '成功', failed: '失败' }[s] || s
}

onMounted(() => loadGroupsAndKeys())
</script>




