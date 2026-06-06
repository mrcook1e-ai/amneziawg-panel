<script setup lang="ts">
import { computed } from 'vue'

type State = 'online' | 'stale' | 'offline'
const props = defineProps<{ state: State }>()

// Three grayscale shades — no chroma, status reads via fill density.
const color = computed(() => ({
  online:  'bg-ink-900',
  stale:   'bg-ink-500',
  offline: 'bg-ink-300',
}[props.state]))
</script>

<template>
  <span class="relative inline-flex">
    <span :class="['inline-block h-2 w-2 rounded-full', color]" />
    <span
      v-if="state === 'online'"
      :class="['absolute inset-0 rounded-full animate-ping opacity-30', color]"
    />
  </span>
</template>
