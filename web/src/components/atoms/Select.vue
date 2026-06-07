<script setup lang="ts">
import { computed } from 'vue'
import { ChevronDown } from 'lucide-vue-next'

type Option = { value: string | number; label: string; disabled?: boolean }

const props = withDefaults(defineProps<{
  modelValue: string | number
  options: Option[]
  size?: 'sm' | 'md'
  placeholder?: string
  disabled?: boolean
  invalid?: boolean
  ariaLabel?: string
}>(), { size: 'md' })

defineEmits<{ (e: 'update:modelValue', v: string): void }>()

// Same focus language as Input — amber inset bar on the left, no border.
const cls = computed(() => [
  'block w-full bg-ink-100 text-ink-900 rounded-2xl border-0 outline-none appearance-none pr-9',
  'transition-[background-color,box-shadow] duration-150',
  'focus:bg-ink-50',
  'focus:shadow-[inset_2px_0_0_0_theme(colors.amber.400)]',
  props.size === 'sm' ? 'h-9 pl-3.5 text-[13.5px]' : 'h-11 pl-4 text-[15px]',
  props.invalid && 'shadow-[inset_2px_0_0_0_theme(colors.danger.DEFAULT)]',
  props.disabled && 'opacity-50 cursor-not-allowed',
])
</script>

<template>
  <div class="relative">
    <select
      :value="modelValue"
      @change="(e) => $emit('update:modelValue', (e.target as HTMLSelectElement).value)"
      :disabled="disabled"
      :aria-label="ariaLabel"
      :class="cls"
    >
      <option v-if="placeholder" value="" disabled>{{ placeholder }}</option>
      <option v-for="o in options" :key="o.value" :value="o.value" :disabled="o.disabled">
        {{ o.label }}
      </option>
    </select>
    <ChevronDown
      :size="size === 'sm' ? 14 : 16"
      class="absolute right-3.5 top-1/2 -translate-y-1/2 text-ink-500 pointer-events-none"
    />
  </div>
</template>
