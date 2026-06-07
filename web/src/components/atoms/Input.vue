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
  Editorial filled input. No border, ever. The focus state is a 2px amber
  bar inside the left edge — like a margin-mark on a typesetter's proof —
  plus a subtle bg shift. Distinctive, not shadcn.

  Invalid state mirrors the focus bar in red, also as inset shadow, so the
  outer geometry never changes (no layout shift, no double-stroke ring).
*/
const cls = computed(() => [
  'block w-full bg-ink-100 text-ink-900 placeholder-ink-500 rounded-2xl border-0 outline-none',
  'transition-[background-color,box-shadow] duration-150',
  'focus:bg-ink-50',
  'focus:shadow-[inset_2px_0_0_0_theme(colors.amber.400)]',
  props.size === 'sm' ? 'h-9 px-3.5 text-[13.5px]' : 'h-11 px-4 text-[15px]',
  props.invalid && 'shadow-[inset_2px_0_0_0_theme(colors.danger.DEFAULT)]',
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
