<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  modelValue: string
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

const cls = computed(() => [
  'block w-full bg-white text-ink-900 placeholder-ink-400 rounded-lg border transition focus-ring',
  props.size === 'sm' ? 'h-8 px-3 text-[13px]' : 'h-10 px-3 text-sm',
  props.invalid ? 'border-danger' : 'border-ink-200 hover:border-ink-300 focus:border-ink-400',
  props.mono && 'font-mono',
  props.disabled && 'bg-ink-50 text-ink-500 cursor-not-allowed',
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
