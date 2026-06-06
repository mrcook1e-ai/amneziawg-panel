<script setup lang="ts">
import { computed } from 'vue'

type State = 'online' | 'stale' | 'offline'
const props = defineProps<{ state: State }>()

const color = computed(() => ({
  online:  'bg-success',
  stale:   'bg-warning',
  offline: 'bg-ink-300',
}[props.state]))
</script>

<template>
  <span class="relative inline-flex">
    <span :class="['inline-block h-2 w-2 rounded-full', color]" />
    <span v-if="state === 'online'" :class="['absolute inset-0 rounded-full animate-ping opacity-40', color]" />
  </span>
</template>
