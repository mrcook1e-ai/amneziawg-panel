<script setup lang="ts">
/*
  Chunked QR carousel for AmneziaWG configs.

  AmneziaWG configs with full obfuscation (S1+S2+I-tags + junk + browser-FP)
  routinely exceed the byte budget of a single QR at scannable density.
  The backend splits the payload into N chunks and we cycle through them so
  the Amnezia mobile app can stitch them back together.

  Used in two places: the inline fullscreen viewer on the device card, and
  the "device created" success step of the add-device wizard.
*/
import { ref, watch, onBeforeUnmount, computed } from 'vue'
import { Loader2, ChevronLeft, ChevronRight, Download } from 'lucide-vue-next'
import { api } from '@/lib/api'

const props = defineProps<{
  token: string
  deviceId: string
  deviceName?: string
  /** Visual size of the QR canvas in px. */
  size?: number
  /** Show prev/next chevrons + dots when there are multiple chunks. */
  controls?: boolean
  /** Show inline .vpn download link under the QR. */
  showDownload?: boolean
}>()

const chunks  = ref<string[]>([])
const idx     = ref(0)
const loading = ref(false)
const error   = ref(false)
let timer: ReturnType<typeof setInterval> | null = null

const sizeStyle = computed(() => {
  const s = props.size ?? 240
  return `width:${s}px;height:${s}px`
})

function stopTimer() {
  if (timer) { clearInterval(timer); timer = null }
}
function startTimer() {
  stopTimer()
  if (chunks.value.length <= 1) return
  // 2.5s per chunk — matches the Amnezia scanner's stitch window.
  timer = setInterval(() => {
    idx.value = (idx.value + 1) % chunks.value.length
  }, 2500)
}

async function load() {
  stopTimer()
  chunks.value = []
  idx.value    = 0
  error.value  = false
  loading.value = true
  try {
    const res = await api.cabinetDeviceAmneziaQrChunks(props.token, props.deviceId)
    chunks.value = res.chunks
    startTimer()
  } catch {
    error.value = true
  } finally {
    loading.value = false
  }
}

watch(() => [props.token, props.deviceId], load, { immediate: true })
onBeforeUnmount(stopTimer)

function prev() {
  stopTimer()
  idx.value = (idx.value - 1 + chunks.value.length) % chunks.value.length
  startTimer()
}
function next() {
  stopTimer()
  idx.value = (idx.value + 1) % chunks.value.length
  startTimer()
}

const vpnUrl = computed(() => api.cabinetDeviceAmneziaVpnUrl(props.token, props.deviceId))
</script>

<template>
  <div class="inline-flex flex-col items-center gap-3">
    <div class="relative" :style="sizeStyle">

      <!-- Loading -->
      <div v-if="loading"
           class="absolute inset-0 flex items-center justify-center bg-white rounded-2xl">
        <Loader2 :size="32" class="text-ink-300 animate-spin" />
      </div>

      <!-- Error -->
      <div v-else-if="error"
           class="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-white rounded-2xl p-6 text-center">
        <p class="text-[13px] font-semibold text-ink-700">QR недоступен</p>
        <p class="text-[11.5px] text-ink-500">Используйте файл .vpn ниже</p>
      </div>

      <!-- Chunks -->
      <template v-else-if="chunks.length">
        <Transition
          enter-active-class="transition-opacity duration-200"
          leave-active-class="transition-opacity duration-150"
          enter-from-class="opacity-0"
          leave-to-class="opacity-0"
          mode="out-in">
          <div
            :key="idx"
            class="w-full h-full bg-white rounded-2xl p-3 shadow-sm">
            <img
              :src="`data:image/png;base64,${chunks[idx]}`"
              :alt="`QR часть ${idx + 1} из ${chunks.length}`"
              class="w-full h-full block"
              style="image-rendering: pixelated"
            />
          </div>
        </Transition>

        <template v-if="controls && chunks.length > 1">
          <button
            class="absolute left-0 -translate-x-10 top-1/2 -translate-y-1/2 w-8 h-8 flex items-center justify-center rounded-full bg-ink-900/10 hover:bg-ink-900/20 text-ink-700 transition-colors"
            aria-label="Предыдущая часть"
            @click="prev">
            <ChevronLeft :size="16" />
          </button>
          <button
            class="absolute right-0 translate-x-10 top-1/2 -translate-y-1/2 w-8 h-8 flex items-center justify-center rounded-full bg-ink-900/10 hover:bg-ink-900/20 text-ink-700 transition-colors"
            aria-label="Следующая часть"
            @click="next">
            <ChevronRight :size="16" />
          </button>
        </template>
      </template>
    </div>

    <!-- Chunk counter + dots -->
    <div v-if="chunks.length > 1" class="flex items-center gap-2.5 h-5">
      <span class="text-ink-500 text-[11px] mono tnum">{{ idx + 1 }} / {{ chunks.length }}</span>
      <div class="flex items-center gap-1">
        <span
          v-for="(_, i) in chunks"
          :key="i"
          class="rounded-full transition-all duration-300"
          :class="i === idx
            ? 'w-3.5 h-1.5 bg-amber-400'
            : 'w-1.5 h-1.5 bg-ink-300'"
        />
      </div>
    </div>

    <a
      v-if="showDownload && deviceId"
      :href="vpnUrl"
      :download="`${deviceName || 'device'}.vpn`"
      class="inline-flex items-center gap-1.5 text-[12px] text-ink-500 hover:text-ink-900 transition-colors">
      <Download :size="13" />
      Скачать .vpn
    </a>
  </div>
</template>
