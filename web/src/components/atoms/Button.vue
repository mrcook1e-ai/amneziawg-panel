<script setup lang="ts">
import { computed } from 'vue'

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger'
type Size = 'sm' | 'md'

const props = withDefaults(defineProps<{
  variant?: Variant
  size?: Size
  type?: 'button' | 'submit' | 'reset'
  disabled?: boolean
  loading?: boolean
  block?: boolean
}>(), { variant: 'secondary', size: 'md', type: 'button' })

const base = 'inline-flex items-center justify-center gap-2 font-medium rounded-lg select-none transition focus-ring disabled:opacity-50 disabled:cursor-not-allowed'

const sizes: Record<Size, string> = {
  sm: 'h-8 px-3 text-[13px]',
  md: 'h-10 px-4 text-sm',
}

const variants: Record<Variant, string> = {
  primary:   'bg-ink-900 text-ink-50 hover:bg-ink-800 active:bg-ink-950',
  secondary: 'bg-surface text-ink-900 border border-ink-200 hover:bg-ink-100 active:bg-ink-200',
  ghost:     'bg-transparent text-ink-700 hover:bg-ink-100 active:bg-ink-200',
  danger:    'bg-danger text-ink-50 hover:opacity-90 active:opacity-100',
}

const cls = computed(() => [base, sizes[props.size], variants[props.variant], props.block && 'w-full'])
</script>

<template>
  <button :type="type" :class="cls" :disabled="disabled || loading">
    <span v-if="loading" class="h-3.5 w-3.5 rounded-full border-2 border-current border-r-transparent animate-spin" />
    <slot />
  </button>
</template>
