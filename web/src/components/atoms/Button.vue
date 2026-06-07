<script setup lang="ts">
import { computed } from 'vue'

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'accent'
type Size = 'sm' | 'md' | 'lg' | 'xl'

const props = withDefaults(defineProps<{
  variant?: Variant
  size?: Size
  type?: 'button' | 'submit' | 'reset'
  disabled?: boolean
  loading?: boolean
  block?: boolean
}>(), { variant: 'secondary', size: 'md', type: 'button' })

const base = 'inline-flex items-center justify-center gap-2 font-medium select-none transition focus-ring disabled:opacity-50 disabled:cursor-not-allowed tracking-chrome active:scale-[0.98]'

// Larger CTA sizes get larger corner radius to match the Amnezia client visual.
const sizes: Record<Size, string> = {
  sm: 'h-8 px-3 text-[12.5px] rounded-lg',
  md: 'h-10 px-4 text-[13.5px] rounded-lg',
  lg: 'h-12 px-5 text-[14px] font-semibold rounded-2xl',
  xl: 'h-14 px-6 text-[15px] font-semibold rounded-2xl',
}

const variants: Record<Variant, string> = {
  primary:   'bg-ink-900 text-ink-50 hover:bg-ink-800 active:bg-ink-950',
  secondary: 'bg-surface text-ink-900 border border-ink-900/10 hover:bg-ink-100 active:bg-ink-200',
  ghost:     'bg-transparent text-ink-700 hover:bg-ink-100 active:bg-ink-200',
  danger:    'bg-danger text-white hover:bg-danger/90 active:bg-danger',
  // Amnezia client accent — phosphor amber, dark ink text. Works on both themes.
  accent:    'bg-amber-400 text-amber-900 hover:bg-amber-500 dark:bg-amber-400 dark:text-[#0E0900] dark:hover:bg-amber-300',
}

const cls = computed(() => [base, sizes[props.size], variants[props.variant], props.block && 'w-full'])
</script>

<template>
  <button :type="type" :class="cls" :disabled="disabled || loading">
    <span v-if="loading" class="h-3.5 w-3.5 rounded-full border-2 border-current border-r-transparent animate-spin" />
    <slot />
  </button>
</template>
