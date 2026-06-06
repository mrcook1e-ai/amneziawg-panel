<script setup lang="ts" generic="T extends string">
defineProps<{
  modelValue: T
  options: { value: T; label: string }[]
}>()
defineEmits<{ (e: 'update:modelValue', v: T): void }>()
</script>

<template>
  <div class="inline-flex p-0.5 bg-ink-100 rounded-lg">
    <button
      v-for="opt in options" :key="opt.value" type="button"
      @click="$emit('update:modelValue', opt.value)"
      :class="[
        'px-3 h-7 text-[12.5px] font-medium rounded-md transition focus-ring',
        modelValue === opt.value
          ? 'bg-surface text-ink-900 shadow-card'
          : 'text-ink-600 hover:text-ink-900',
      ]"
    >{{ opt.label }}</button>
  </div>
</template>
