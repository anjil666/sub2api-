<template>
  <AppLayout>
    <div class="space-y-3">
      <!-- Header -->
      <div class="card flex flex-wrap items-center gap-3 p-3">
        <div v-if="price" class="text-xs text-gray-500 dark:text-gray-400">
          单次: <span class="font-medium text-emerald-600 dark:text-emerald-400">${{ price }}</span>
        </div>
        <div v-if="error" class="text-sm text-red-500">{{ error }}</div>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-5 gap-3">
        <!-- Left: Input -->
        <div class="lg:col-span-2 card p-4 space-y-4">
          <!-- Tab -->
          <div class="flex gap-2">
            <button @click="activeTab = 'text'" class="px-4 py-1.5 rounded-lg text-sm font-medium transition-colors"
              :class="activeTab === 'text' ? 'bg-primary-500 text-white' : 'bg-gray-100 dark:bg-dark-700 text-gray-600 dark:text-gray-400'">
              文生视频
            </button>
            <button @click="activeTab = 'image'" class="px-4 py-1.5 rounded-lg text-sm font-medium transition-colors"
              :class="activeTab === 'image' ? 'bg-primary-500 text-white' : 'bg-gray-100 dark:bg-dark-700 text-gray-600 dark:text-gray-400'">
              图生视频
            </button>
          </div>

          <!-- Image upload (img2video) -->
          <div v-if="activeTab === 'image'" class="space-y-2">
            <label class="text-sm font-medium text-gray-700 dark:text-gray-300">上传图片</label>
            <div class="relative">
              <input type="file" accept="image/*" @change="onImageChange" class="hidden" ref="fileInput" />
              <div @click="($refs.fileInput as HTMLInputElement)?.click()"
                class="border-2 border-dashed rounded-lg p-4 text-center cursor-pointer transition-colors
                  border-gray-300 dark:border-dark-600 hover:border-primary-400 dark:hover:border-primary-500">
                <div v-if="imageFile" class="space-y-1">
                  <p class="text-sm text-gray-600 dark:text-gray-400">{{ imageFile.name }}</p>
                  <p class="text-xs text-gray-400">点击重新选择</p>
                </div>
                <div v-else>
                  <svg class="mx-auto h-8 w-8 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"/></svg>
                  <p class="mt-1 text-sm text-gray-500">点击选择图片</p>
                </div>
              </div>
            </div>
          </div>

          <!-- Options row -->
          <div class="flex flex-wrap items-center gap-4">
            <div v-if="videoModels.length > 1" class="flex items-center gap-2">
              <label class="text-sm text-gray-600 dark:text-gray-400">模型</label>
              <select v-model="selectedModel" class="input !w-auto !py-1 !text-sm">
                <option v-for="m in videoModels" :key="m.id" :value="m.id">{{ m.name }}</option>
              </select>
            </div>
            <div v-else-if="videoModels.length === 1" class="flex items-center gap-2">
              <label class="text-sm text-gray-600 dark:text-gray-400">模型</label>
              <span class="text-sm font-medium text-primary-600 dark:text-primary-400">{{ videoModels[0].name }}</span>
            </div>
            <div class="flex items-center gap-2">
              <label class="text-sm text-gray-600 dark:text-gray-400">画面比例</label>
              <select v-model="aspectRatio" class="input !w-auto !py-1 !text-sm">
                <option value="9:16">9:16 竖屏</option>
                <option value="16:9">16:9 横屏</option>
              </select>
            </div>
            <div v-if="activeTab === 'text'" class="flex items-center gap-2">
              <label class="text-sm text-gray-600 dark:text-gray-400">生成数量</label>
              <select v-model="generateCount" class="input !w-auto !py-1 !text-sm">
                <option :value="1">1</option>
                <option :value="2">2</option>
              </select>
            </div>
          </div>

          <!-- Prompt -->
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <label class="text-sm font-medium text-gray-700 dark:text-gray-300">提示词</label>
              <button @click="enhancePrompt" :disabled="enhancing || !prompt.trim()"
                class="text-xs px-3 py-1 rounded-md font-medium transition-colors bg-gradient-to-r from-amber-500 to-orange-500 text-white hover:from-amber-600 hover:to-orange-600 disabled:opacity-40 disabled:cursor-not-allowed shadow-sm">
                <span v-if="enhancing">优化中...</span>
                <span v-else>AI 优化提示词</span>
              </button>
            </div>
            <textarea v-model="prompt" rows="4" :placeholder="activeTab === 'text' ? '描述你想生成的视频内容...' : '描述视频动作（可选）...'"
              class="input !text-sm w-full resize-y"></textarea>
          </div>

          <!-- Submit -->
          <button @click="activeTab === 'text' ? submitTextGeneration() : submitImg2Video()"
            :disabled="submitting || (activeTab === 'text' ? !prompt.trim() : !imageFile)"
            class="btn-primary w-full !py-2.5">
            <span v-if="submitting">提交中...</span>
            <span v-else>{{ activeTab === 'text' ? '生成视频' : '图片生成视频' }}</span>
          </button>
        </div>

        <!-- Right: Tasks Grid -->
        <div class="lg:col-span-3 card p-4 space-y-3">
          <div class="flex items-center justify-between">
            <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">生成任务</h3>
            <button v-if="tasks.length" @click="clearCompleted"
              class="text-xs text-gray-400 hover:text-red-500 transition-colors">清空已完成</button>
          </div>
          <p class="text-xs text-amber-600 dark:text-amber-400">视频及时下载，链接会过期</p>
          <div v-if="!tasks.length" class="text-center py-8 text-sm text-gray-400">暂无任务</div>
          <div class="grid grid-cols-2 gap-3">
            <div v-for="task in tasks" :key="task.id"
              class="border rounded-lg p-2 space-y-1.5 border-gray-200 dark:border-dark-600">
              <div class="flex items-center justify-between">
                <span class="text-[10px] px-1.5 py-0.5 rounded-full"
                  :class="{
                    'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400': task.status === 'pending',
                    'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400': task.status === 'processing',
                    'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400': task.status === 'completed',
                    'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400': task.status === 'failed',
                  }">
                  {{ { pending: '排队中', processing: '生成中', completed: '已完成', failed: '失败' }[task.status] }}
                </span>
                <button @click="removeTask(task.id)" class="text-gray-400 hover:text-red-500 text-[10px]">删除</button>
              </div>
              <p class="text-[10px] text-gray-500 dark:text-gray-400 line-clamp-1">{{ task.prompt }}</p>
              <div v-if="task.status === 'completed' && task.video_url" class="space-y-1">
                <div class="relative cursor-pointer group" @click="openPlayer(task)">
                  <video :src="proxyVideoUrl(task.video_url)" class="w-full aspect-video rounded object-cover" preload="metadata" muted></video>
                  <div class="absolute inset-0 flex items-center justify-center bg-black/20 opacity-0 group-hover:opacity-100 transition-opacity rounded">
                    <svg class="w-8 h-8 text-white" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>
                  </div>
                </div>
                <button @click="downloadVideo(task)" :disabled="isDownloading(task.id)"
                  class="block w-full text-center text-[10px] text-primary-500 hover:underline cursor-pointer disabled:opacity-50 disabled:cursor-wait">
                  {{ isDownloading(task.id) ? '下载中...' : '下载视频' }}
                </button>
              </div>
              <p v-if="task.status === 'failed' && task.error" class="text-[10px] text-red-500 line-clamp-2">{{ task.error }}</p>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Fullscreen Player Modal -->
    <Teleport to="body">
      <div v-if="playerTask" class="fixed inset-0 z-50 flex items-center justify-center bg-black/80" @click.self="playerTask = null">
        <div class="relative w-full max-w-3xl mx-4">
          <button @click="playerTask = null" class="absolute -top-10 right-0 text-white text-sm hover:text-gray-300">关闭</button>
          <video :src="proxyVideoUrl(playerTask.video_url)" controls autoplay class="w-full rounded-lg shadow-2xl"></video>
          <div class="mt-2 flex items-center justify-between">
            <p class="text-xs text-gray-300 line-clamp-1 flex-1 mr-4">{{ playerTask.prompt }}</p>
            <button @click="downloadVideo(playerTask!)" :disabled="isDownloading(playerTask!.id)"
              class="text-xs text-primary-400 hover:underline whitespace-nowrap disabled:opacity-50">
              {{ isDownloading(playerTask!.id) ? '下载中...' : '下载视频' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useVideoGeneration, type VideoTask } from '@/composables/useVideoGeneration'

const {
  submitting, enhancing, error,
  price, videoModels, selectedModel,
  prompt, aspectRatio, generateCount, imageFile, activeTab,
  tasks,
  fetchPrice, enhancePrompt,
  submitTextGeneration, submitImg2Video,
  removeTask, clearCompleted, downloadVideo, isDownloading, resumePolling,
} = useVideoGeneration()

const fileInput = ref<HTMLInputElement | null>(null)
const playerTask = ref<VideoTask | null>(null)

function proxyVideoUrl(url: string): string {
  if (!url) return ''
  const token = localStorage.getItem('auth_token') || ''
  return `/api/v1/video/proxy?url=${encodeURIComponent(url)}&token=${encodeURIComponent(token)}`
}

function onImageChange(e: Event) {
  const input = e.target as HTMLInputElement
  if (input.files?.length) {
    imageFile.value = input.files[0]
  }
}

function openPlayer(task: VideoTask) {
  playerTask.value = task
}

onMounted(() => {
  fetchPrice()
  resumePolling()
})
</script>
