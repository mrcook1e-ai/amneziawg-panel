<script setup lang="ts">
import { computed } from 'vue'

type Tone = 'neutral' | 'success' | 'warning' | 'danger'
const props = withDefaults(defineProps<{ tone?: Tone }>(), { tone: 'neutral' })

const cls = computed(() => {
  const tones: Record<Tone, string> = {
    neutral: 'bg-ink-100 text-ink-700',
    success: 'bg-success-soft text-success',
    warning: 'bg-warning-soft text-warning',
    danger:  'bg-danger-soft text-danger',
  }
  return ['inline-flex items-center gap-1.5 h-6 px-2 rounded-full text-[11px] font-medium tracking-wide', tones[props.tone]]
})
</script>

<template>
  <span :class="cls"><slot /></span>
</template>
