<template>
  <AppLayout>
    <div class="space-y-4">
      <!-- Header: Group + Model selectors -->
      <div class="flex flex-wrap items-center gap-3 rounded-lg bg-white p-4 shadow dark:bg-gray-800">
        <div class="flex items-center gap-2">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">分组</label>
          <select v-model="selectedGroupId" class="rounded border border-gray-300 bg-white px-3 py-1.5 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200">
            <option v-for="g in groups" :key="g.group_id" :value="g.group_id">{{ g.group_name }}</option>
          </select>
          <button @click="loadGroupsAndKeys" class="rounded p-1.5 text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700" title="刷新">
            <svg class="h-4 w-4" :class="{ 'animate-spin': loadingGroups }" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
          </button>
        </div>
        <div class="flex items-center gap-2">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">模型</label>
          <select v-model="selectedModel" class="rounded border border-gray-300 bg-white px-3 py-1.5 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200">
            <option v-for="m in imageModels" :key="m.model_name" :value="m.model_name">{{ m.model_name }}</option>
          </select>
        </div>
        <div v-if="error" class="ml-auto text-sm text-red-500">{{ error }}</div>
      </div>

      <!-- Tabs -->
      <div class="flex gap-1 rounded-lg bg-gray-100 p-1 dark:bg-gray-700">
        <button v-for="tab in tabs" :key="tab.key" @click="activeTab = tab.key"
          :class="[activeTab === tab.key ? 'bg-white shadow dark:bg-gray-600 dark:text-white' : 'text-gray-600 hover:text-gray-900 dark:text-gray-400', 'rounded-md px-3 py-1.5 text-sm font-medium transition-colors']">
          {{ tab.label }}
        </button>
      </div>

      <!-- Tab Content -->
      <div class="rounded-lg bg-white p-4 shadow dark:bg-gray-800">
        <!-- GENERATION TAB -->
        <div v-if="activeTab === 'generation'" class="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <!-- Left: params -->
          <div class="space-y-4">
            <ParamPanel v-bind="paramBindings" @update:resolutionTier="resolutionTier = $event" @update:selectedRatio="selectedRatio = $event"
              @update:customW="customW = $event" @update:customH="customH = $event" @update:outputFormat="outputFormat = $event as any"
              @update:outputCompression="outputCompression = $event" @update:stylePreset="stylePreset = $event" @update:imageCount="imageCount = $event" />
          </div>
          <!-- Center: prompt -->
          <div class="flex flex-col gap-3">
            <textarea v-model="prompt" rows="8" placeholder="描述你想生成的图片..." class="w-full rounded-lg border border-gray-300 p-3 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200" />
            <button @click="generate" :disabled="loading || !prompt.trim() || !groupApiKey" class="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">
              {{ loading ? `生成中... ${elapsed}s` : '生成图片' }}
            </button>
            <button v-if="loading" @click="abort" class="rounded-lg border border-red-300 px-4 py-1.5 text-sm text-red-600 hover:bg-red-50">取消</button>
          </div>
          <!-- Right: results -->
          <ResultPanel :urls="resultUrls" :loading="loading" :elapsed="elapsed" />
        </div>

        <!-- SINGLE EDIT TAB -->
        <div v-if="activeTab === 'single-edit'" class="grid grid-cols-1 gap-4 lg:grid-cols-4">
          <div class="space-y-3">
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">参考图</label>
            <FileDropZone :files="editFile ? [editFile] : []" :max="1" accept="image/*" @update="editFile = $event[0] || null" />
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">遮罩图 (可选)</label>
            <FileDropZone :files="maskFile ? [maskFile] : []" :max="1" accept="image/*" @update="maskFile = $event[0] || null" />
          </div>
          <div><ParamPanel v-bind="paramBindings" @update:resolutionTier="resolutionTier = $event" @update:selectedRatio="selectedRatio = $event" @update:customW="customW = $event" @update:customH="customH = $event" @update:outputFormat="outputFormat = $event as any" @update:outputCompression="outputCompression = $event" @update:stylePreset="stylePreset = $event" @update:imageCount="imageCount = $event" /></div>
          <div class="flex flex-col gap-3">
            <textarea v-model="prompt" rows="6" placeholder="描述编辑内容..." class="w-full rounded-lg border border-gray-300 p-3 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200" />
            <button @click="editImage" :disabled="loading || !prompt.trim() || !groupApiKey" class="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">{{ loading ? `编辑中... ${elapsed}s` : '开始编辑' }}</button>
            <button v-if="loading" @click="abort" class="rounded-lg border border-red-300 px-4 py-1.5 text-sm text-red-600 hover:bg-red-50">取消</button>
          </div>
          <ResultPanel :urls="resultUrls" :loading="loading" :elapsed="elapsed" />
        </div>

        <!-- MULTI EDIT TAB -->
        <div v-if="activeTab === 'multi-edit'" class="grid grid-cols-1 gap-4 lg:grid-cols-4">
          <div class="space-y-3">
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300">参考图 (最多5张)</label>
            <FileDropZone :files="multiFiles" :max="5" accept="image/*" @update="multiFiles = $event" />
          </div>
          <div><ParamPanel v-bind="paramBindings" @update:resolutionTier="resolutionTier = $event" @update:selectedRatio="selectedRatio = $event" @update:customW="customW = $event" @update:customH="customH = $event" @update:outputFormat="outputFormat = $event as any" @update:outputCompression="outputCompression = $event" @update:stylePreset="stylePreset = $event" @update:imageCount="imageCount = $event" /></div>
          <div class="flex flex-col gap-3">
            <textarea v-model="prompt" rows="6" placeholder="描述编辑内容..." class="w-full rounded-lg border border-gray-300 p-3 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200" />
            <button @click="editImage" :disabled="loading || !prompt.trim() || !groupApiKey" class="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">{{ loading ? `编辑中... ${elapsed}s` : '开始编辑' }}</button>
            <button v-if="loading" @click="abort" class="rounded-lg border border-red-300 px-4 py-1.5 text-sm text-red-600 hover:bg-red-50">取消</button>
          </div>
          <ResultPanel :urls="resultUrls" :loading="loading" :elapsed="elapsed" />
        </div>

        <!-- BATCH TAB -->
        <div v-if="activeTab === 'batch'" class="space-y-4">
          <div class="flex items-center justify-between">
            <div class="text-sm text-gray-600 dark:text-gray-400">{{ batchProgress.done }}/{{ batchProgress.total }} 完成</div>
            <div class="flex gap-2">
              <button @click="addBatchTask" class="rounded bg-gray-200 px-3 py-1 text-sm hover:bg-gray-300 dark:bg-gray-600 dark:hover:bg-gray-500">添加任务</button>
              <button @click="runBatchTasks" :disabled="!batchTasks.length || !groupApiKey" class="rounded bg-blue-600 px-3 py-1 text-sm text-white hover:bg-blue-700 disabled:opacity-50">开始全部</button>
            </div>
          </div>
          <div class="mb-3"><ParamPanel v-bind="paramBindings" @update:resolutionTier="resolutionTier = $event" @update:selectedRatio="selectedRatio = $event" @update:customW="customW = $event" @update:customH="customH = $event" @update:outputFormat="outputFormat = $event as any" @update:outputCompression="outputCompression = $event" @update:stylePreset="stylePreset = $event" @update:imageCount="imageCount = $event" :compact="true" /></div>
          <div v-for="task in batchTasks" :key="task.id" class="flex items-start gap-3 rounded-lg border border-gray-200 p-3 dark:border-gray-600">
            <div class="flex-1 space-y-2">
              <textarea v-model="task.prompt" rows="2" placeholder="任务提示词..." class="w-full rounded border border-gray-300 p-2 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200" />
              <FileDropZone :files="task.referenceFiles" :max="5" accept="image/*" @update="task.referenceFiles = $event" :compact="true" />
            </div>
            <div class="flex flex-col items-center gap-1">
              <span :class="statusClass(task.status)" class="rounded px-2 py-0.5 text-xs">{{ statusLabel(task.status) }}</span>
              <span v-if="task.elapsed" class="text-xs text-gray-500">{{ task.elapsed }}s</span>
              <button @click="removeBatchTask(task.id)" class="text-xs text-red-500 hover:text-red-700">删除</button>
            </div>
            <img v-if="task.result" :src="task.result" class="h-20 w-20 rounded object-cover" />
            <div v-if="task.error" class="text-xs text-red-500">{{ task.error }}</div>
          </div>
        </div>

        <!-- STORYBOARD TAB -->
        <div v-if="activeTab === 'storyboard'" class="space-y-4">
          <div class="flex items-center justify-between">
            <div class="text-sm text-gray-600 dark:text-gray-400">第 {{ storyProgress.done }}/{{ storyProgress.total }} 幕</div>
            <div class="flex gap-2">
              <button @click="addScene" class="rounded bg-gray-200 px-3 py-1 text-sm hover:bg-gray-300 dark:bg-gray-600 dark:hover:bg-gray-500">添加场景</button>
              <button @click="runStoryboard" :disabled="!storyScenes.length || !groupApiKey" class="rounded bg-blue-600 px-3 py-1 text-sm text-white hover:bg-blue-700 disabled:opacity-50">生成全部</button>
            </div>
          </div>
          <div class="mb-3"><ParamPanel v-bind="paramBindings" @update:resolutionTier="resolutionTier = $event" @update:selectedRatio="selectedRatio = $event" @update:customW="customW = $event" @update:customH="customH = $event" @update:outputFormat="outputFormat = $event as any" @update:outputCompression="outputCompression = $event" @update:stylePreset="stylePreset = $event" @update:imageCount="imageCount = $event" :compact="true" /></div>
          <div class="space-y-2">
            <label class="text-sm font-medium text-gray-700 dark:text-gray-300">角色参考图 (最多5张，所有场景共用)</label>
            <FileDropZone :files="storyCharacterFiles" :max="5" accept="image/*" @update="storyCharacterFiles = $event" />
          </div>
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            <div v-for="scene in storyScenes" :key="scene.id" class="rounded-lg border border-gray-200 p-3 dark:border-gray-600">
              <div class="mb-2 flex items-center justify-between">
                <span class="text-sm font-medium">第 {{ scene.index }} 幕</span>
                <div class="flex items-center gap-2">
                  <span :class="statusClass(scene.status)" class="rounded px-2 py-0.5 text-xs">{{ statusLabel(scene.status) }}</span>
                  <button @click="removeScene(scene.id)" class="text-xs text-red-500 hover:text-red-700">删除</button>
                </div>
              </div>
              <textarea v-model="scene.prompt" rows="2" placeholder="场景描述..." class="mb-2 w-full rounded border border-gray-300 p-2 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200" />
              <img v-if="scene.result" :src="scene.result" class="w-full rounded object-cover" />
              <div v-if="scene.error" class="text-xs text-red-500">{{ scene.error }}</div>
            </div>
          </div>
        </div>

        <!-- GALLERY TAB -->
        <div v-if="activeTab === 'gallery'">
          <GalleryPanel />
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useImageGeneration, type StudioTab } from '@/composables/useImageGeneration'
import ParamPanel from '@/components/imageStudio/ParamPanel.vue'
import ResultPanel from '@/components/imageStudio/ResultPanel.vue'
import FileDropZone from '@/components/imageStudio/FileDropZone.vue'
import GalleryPanel from '@/components/imageStudio/GalleryPanel.vue'

const {
  activeTab, loading, loadingGroups, error, elapsed,
  groups, selectedGroupId, selectedModel, imageModels, groupApiKey,
  resolutionTier, selectedRatio, customW, customH, outputFormat, outputCompression,
  stylePreset, imageCount, prompt,
  editFile, maskFile, multiFiles,
  resultUrls,
  batchTasks, batchProgress,
  storyCharacterFiles, storyScenes, storyProgress,
  loadGroupsAndKeys, generate, editImage, abort,
  addBatchTask, removeBatchTask, runBatchTasks,
  addScene, removeScene, runStoryboard,
} = useImageGeneration()

const tabs: { key: StudioTab; label: string }[] = [
  { key: 'generation', label: '基础生图' },
  { key: 'single-edit', label: '单图编辑' },
  { key: 'multi-edit', label: '多图编辑' },
  { key: 'batch', label: '批量任务' },
  { key: 'storyboard', label: '分镜创作' },
  { key: 'gallery', label: '图片管理' },
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
  return { pending: 'bg-gray-100 text-gray-600', running: 'bg-blue-100 text-blue-700 animate-pulse', success: 'bg-green-100 text-green-700', failed: 'bg-red-100 text-red-700' }[s] || ''
}
function statusLabel(s: string) {
  return { pending: '等待', running: '生成中', success: '成功', failed: '失败' }[s] || s
}

onMounted(() => loadGroupsAndKeys())
</script>
