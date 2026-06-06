<script setup lang="ts">
const props = defineProps<{ modelValue: boolean; disabled?: boolean; label?: string }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>()
const toggle = () => { if (!props.disabled) emit('update:modelValue', !props.modelValue) }
</script>

<template>
  <button
    type="button"
    role="switch"
    :aria-checked="modelValue"
    :aria-label="label"
    :disabled="disabled"
    @click="toggle"
    :class="[
      'relative inline-flex h-5 w-9 rounded-full transition focus-ring',
      modelValue ? 'bg-ink-900' : 'bg-ink-200',
      disabled && 'opacity-50 cursor-not-allowed',
    ]"
  >
    <span
      :class="[
        'absolute top-0.5 left-0.5 h-4 w-4 rounded-full bg-white shadow transition-transform',
        modelValue && 'translate-x-4',
      ]"
    />
  </button>
</template>
