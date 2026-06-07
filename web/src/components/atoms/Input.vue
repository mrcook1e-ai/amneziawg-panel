<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  modelValue: string | number
  size?: 'sm' | 'md'
  placeholder?: string
  type?: string
  invalid?: boolean
  mono?: boolean
  autocomplete?: string
  autofocus?: boolean
  disabled?: boolean
}>(), { size: 'md', type: 'text' })

defineEmits<{ (e: 'update:modelValue', v: string): void }>()

/*
  Editorial filled input. No border, no inset bar. Focus = the whole input
  lifts: brighter fill (ink-50) with a faint warm amber wash. Invalid =
  the whole field tints red. Geometry never changes between states.
*/
const cls = computed(() => [
  'block w-full bg-ink-100 text-ink-900 placeholder-ink-500 rounded-2xl border-0 outline-none',
  'transition-colors duration-150',
  'focus:bg-amber-50 dark:focus:bg-amber-400/10 focus:text-ink-900',
  props.size === 'sm' ? 'h-9 px-3.5 text-[13.5px]' : 'h-11 px-4 text-[15px]',
  props.invalid && 'bg-danger/10 text-danger placeholder-danger/60',
  props.mono && 'font-mono',
  props.disabled && 'opacity-50 cursor-not-allowed',
])
</script>

<template>
  <input
    :value="modelValue"
    @input="(e) => $emit('update:modelValue', (e.target as HTMLInputElement).value)"
    :type="type"
    :placeholder="placeholder"
    :autocomplete="autocomplete"
    :autofocus="autofocus"
    :disabled="disabled"
    :class="cls"
  />
</template>
