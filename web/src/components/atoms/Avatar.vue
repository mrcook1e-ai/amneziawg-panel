<script setup lang="ts">
import { computed } from 'vue'
import StatusDot from './StatusDot.vue'

type State = 'online' | 'stale' | 'offline'
const props = withDefaults(defineProps<{
  name: string
  state?: State | null
  size?: number
}>(), { size: 40, state: null })

// Three grayscale shades drawn from the ink palette. Because the palette is
// CSS-variable-backed, ink-700..900 inverts brightness between themes — so
// avatars stay readable against text-ink-50 (which inverts the other way).
const palette = [700, 800, 900]

const initial = computed(() => {
  const t = props.name.trim()
  if (!t) return '?'
  const parts = t.split(/\s+/)
  return (parts[0][0] + (parts[1]?.[0] || '')).toUpperCase().slice(0, 2)
})

const shade = computed(() => {
  let h = 0
  for (const ch of props.name) h = (h * 31 + ch.charCodeAt(0)) >>> 0
  return palette[h % palette.length]
})
</script>

<template>
  <div class="relative shrink-0">
    <div
      class="rounded-full flex items-center justify-center text-ink-50 font-semibold tracking-tight"
      :style="{
        width: size + 'px',
        height: size + 'px',
        backgroundColor: `rgb(var(--ink-${shade}))`,
        fontSize: Math.round(size * 0.38) + 'px',
      }"
    >
      {{ initial }}
    </div>
    <div
      v-if="state"
      class="absolute -bottom-0.5 -right-0.5 rounded-full ring-2"
      :style="{ '--tw-ring-color': 'rgb(var(--surface))' }"
    >
      <StatusDot :state="state" />
    </div>
  </div>
</template>
