<script setup lang="ts">
import { computed } from 'vue'

type Tone = 'neutral' | 'success' | 'warning' | 'danger'
type Size = 'xs' | 'sm'

const props = withDefaults(defineProps<{
  tone?: Tone
  size?: Size
}>(), { tone: 'neutral', size: 'sm' })

const cls = computed(() => {
  const tones: Record<Tone, string> = {
    neutral: 'bg-ink-100 text-ink-700',
    success: 'bg-success-soft text-success',
    warning: 'bg-warning-soft text-warning',
    danger:  'bg-danger/10 text-danger',
  }
  const sizes: Record<Size, string> = {
    xs: 'h-5 px-1.5 text-[10px] uppercase tracking-wide font-semibold rounded-full',
    sm: 'h-6 px-2 text-[11px] font-medium tracking-wide rounded-full',
  }
  return ['inline-flex items-center gap-1.5', sizes[props.size], tones[props.tone]]
})
</script>

<template>
  <span :class="cls"><slot /></span>
</template>
