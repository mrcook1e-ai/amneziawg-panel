<script setup lang="ts">
import { computed } from 'vue'
import StatusDot from './StatusDot.vue'

type State = 'online' | 'stale' | 'offline'
const props = withDefaults(defineProps<{
  name: string
  state?: State | null
  size?: number
}>(), { size: 40, state: null })

// Strict B/W: a handful of grayscale tones so different avatars stay visually
// distinct without introducing chroma.
const palette = ['#1c1c1e', '#2c2c2e', '#3a3a3c', '#48484a', '#6c6c70']

const initial = computed(() => {
  const t = props.name.trim()
  if (!t) return '?'
  const parts = t.split(/\s+/)
  return (parts[0][0] + (parts[1]?.[0] || '')).toUpperCase().slice(0, 2)
})

const bg = computed(() => {
  let h = 0
  for (const ch of props.name) h = (h * 31 + ch.charCodeAt(0)) >>> 0
  return palette[h % palette.length]
})
</script>

<template>
  <div class="relative shrink-0">
    <div
      class="rounded-full flex items-center justify-center text-white font-semibold tracking-tight"
      :style="{ width: size + 'px', height: size + 'px', backgroundColor: bg, fontSize: Math.round(size * 0.38) + 'px' }"
    >
      {{ initial }}
    </div>
    <div
      v-if="state"
      class="absolute -bottom-0.5 -right-0.5 rounded-full ring-2 ring-white"
    >
      <StatusDot :state="state" />
    </div>
  </div>
</template>
