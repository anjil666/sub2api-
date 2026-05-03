<template>
  <AppLayout>
    <div class="space-y-3">
      <!-- Header -->
      <div class="card flex flex-wrap items-center gap-3 p-3">
        <div class="flex items-center gap-2">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">分组</label>
          <select v-model="selectedGroupId" class="input !w-auto !py-1.5 !text-sm">
            <option v-for="g in groups" :key="g.group_id" :value="g.group_id">
              {{ cleanGroupName(g.group_name) }}
            </option>
          </select>
          <span v-if="!groupApiKey && selectedGroupId" class="text-xs text-amber-500">无可用密钥</span>
          <button @click="loadGroupsAndKeys" class="btn-secondary !rounded-lg !px-2 !py-1" title="刷新">
            <svg class="h-4 w-4" :class="{ 'animate-spin': loadingGroups }" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
          </button>
        </div>
        <div class="flex items-center gap-2">
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">模型</label>
          <select v-model="selectedModel" class="input !w-auto !py-1.5 !text-sm">
            <option v-for="m in videoModels" :key="m.id" :value="m.id">{{ m.name }}</option>
          </select>
        </div>
        <div v-if="price" class="text-xs text-gray-500 dark:text-gray-400">
          单次生成价格: <span class="font-medium text-emerald-600 dark:text-emerald-400">${{ price }}</span>
        </div>
        <div v-if="error" class="text-sm text-red-500">{{ error }}</div>
        <div v-else class="text-xs text-amber-600 dark:text-amber-400">提示：请先在「API密钥」页面创建密钥并绑定视频分组</div>
      </div>

      <!-- Generation Panel -->
      <div class="grid grid-cols-1 gap-3 lg:grid-cols-[1fr_1fr]">
        <!-- Left: Input -->
        <div class="card flex flex-col gap-3 p-4">
          <div class="text-sm font-semibold text-emerald-600 dark:text-emerald-400">文生视频</div>
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">画面比例</label>
            <div class="flex gap-2">
              <button v-for="r in aspectRatios" :key="r" @click="aspectRatio = r"
                :class="[aspectRatio === r ? 'bg-blue-600 text-white' : 'bg-gray-100 text-gray-700 hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300', 'rounded-lg px-3 py-1.5 text-sm font-medium transition-colors']">
                {{ r }}
              </button>
            </div>
          </div>
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">创意描述提示词</label>
            <textarea v-model="prompt" rows="5" placeholder="描述你想生成的视频内容..." class="input w-full" />
          </div>
          <button @click="submitGeneration" :disabled="!prompt.trim() || !groupApiKey || submitting" class="btn btn-primary text-sm">
            {{ submitting ? '提交中...' : '开始生成' }}
          </button>
        </div>

        <!-- Right: Task List -->
        <div class="card flex flex-col gap-3 p-4">
          <div class="text-sm font-semibold text-blue-600 dark:text-blue-400">生成任务</div>
          <div v-if="!tasks.length" class="flex flex-1 items-center justify-center text-sm text-gray-400 dark:text-gray-500">
            暂无任务，提交生成后将在此显示
          </div>
          <div v-else class="space-y-3 overflow-y-auto" style="max-height: 500px;">
            <div v-for="task in tasks" :key="task.id" class="rounded-lg border border-gray-200 p-3 dark:border-dark-600">
              <div class="mb-2 flex items-center justify-between">
                <span :class="statusClass(task.status)" class="rounded-full px-2 py-0.5 text-[10px] font-medium">
                  {{ statusLabel(task.status) }}
                </span>
                <div class="flex items-center gap-2">
                  <span class="text-[10px] text-gray-400">{{ task.model }}</span>
                  <span class="text-[10px] text-gray-400">{{ task.aspect_ratio }}</span>
                  <button @click="removeTask(task.id)" class="text-[10px] text-red-500 hover:text-red-700">删除</button>
                </div>
              </div>
              <p class="mb-2 line-clamp-2 text-xs text-gray-600 dark:text-gray-400">{{ task.prompt }}</p>
              <!-- Video Player -->
              <div v-if="task.status === 'completed' && task.video_url" class="space-y-2">
                <video :src="task.video_url" controls class="w-full rounded-lg" preload="metadata" />
                <a :href="task.video_url" download class="btn btn-secondary inline-flex items-center gap-1 !py-1 text-xs">
                  <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"/></svg>
                  下载视频
                </a>
              </div>
              <!-- Processing indicator -->
              <div v-else-if="task.status === 'pending' || task.status === 'processing'" class="flex items-center gap-2 rounded-lg bg-blue-50 p-3 dark:bg-blue-900/20">
                <svg class="h-4 w-4 animate-spin text-blue-500" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
                <span class="text-xs text-blue-600 dark:text-blue-400">视频生成中，请耐心等待...</span>
              </div>
              <!-- Error -->
              <div v-if="task.error" class="text-[10px] text-red-500">{{ task.error }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { onMounted } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useVideoGeneration } from '@/composables/useVideoGeneration'
import { cleanGroupName } from '@/utils/format'

const {
  loadingGroups, submitting, error,
  groups, selectedGroupId, groupApiKey,
  videoModels, selectedModel, price,
  prompt, aspectRatio,
  tasks,
  loadGroupsAndKeys, submitGeneration, removeTask,
} = useVideoGeneration()

const aspectRatios = ['16:9', '9:16', '1:1']

function statusClass(s: string) {
  return {
    pending: 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-400',
    processing: 'bg-blue-100 text-blue-700 animate-pulse dark:bg-blue-900/30',
    completed: 'bg-green-100 text-green-700 dark:bg-green-900/30',
    failed: 'bg-red-100 text-red-700 dark:bg-red-900/30',
  }[s] || ''
}

function statusLabel(s: string) {
  return { pending: '等待中', processing: '生成中', completed: '已完成', failed: '失败' }[s] || s
}

onMounted(() => loadGroupsAndKeys())
</script>
