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

// iOS-style filled input: no border, subtle gray fill, soft inner focus tint.
// In light: fill = ink-100 (#e5e5ea) on white card. In dark: ink-100 = #2c2c2e
// on surface (#1c1c1e). Both surfaces are visually distinct without an outline.
const cls = computed(() => [
  'block w-full bg-ink-100 text-ink-900 placeholder-ink-500 rounded-xl border-0 outline-none transition',
  'focus:bg-ink-200 focus-visible:outline-none',
  props.size === 'sm' ? 'h-9 px-3.5 text-[13.5px]' : 'h-11 px-4 text-[15px]',
  props.invalid && 'ring-2 ring-danger/50',
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
