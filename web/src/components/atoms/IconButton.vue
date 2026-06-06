<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  size?: 'sm' | 'md'
  variant?: 'ghost' | 'soft'
  disabled?: boolean
  title?: string
  tone?: 'neutral' | 'danger'
}>(), { size: 'md', variant: 'ghost', tone: 'neutral' })

const cls = computed(() => [
  'inline-flex items-center justify-center rounded-lg transition focus-ring disabled:opacity-40',
  props.size === 'sm' ? 'h-8 w-8' : 'h-9 w-9',
  props.variant === 'ghost' ? 'hover:bg-ink-100' : 'bg-ink-100 hover:bg-ink-200',
  props.tone === 'danger' ? 'text-danger hover:bg-danger-soft' : 'text-ink-700',
])
</script>

<template>
  <button type="button" :disabled="disabled" :class="cls" :title="title" :aria-label="title">
    <slot />
  </button>
</template>
