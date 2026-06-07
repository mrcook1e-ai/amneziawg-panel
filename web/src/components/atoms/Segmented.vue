<script setup lang="ts" generic="T extends string">
defineProps<{
  modelValue: T
  options: { value: T; label: string }[]
}>()
defineEmits<{ (e: 'update:modelValue', v: T): void }>()
</script>

<template>
  <!--
    Editorial segmented control. Active option is a heavy ink-on-paper
    inversion — no pill, no shadow, no fake-luxury 2px border. The container
    fill defines the bounds; the active block defines the choice.
  -->
  <div class="inline-flex p-1 bg-ink-100 rounded-xl">
    <button
      v-for="opt in options" :key="opt.value" type="button"
      @click="$emit('update:modelValue', opt.value)"
      :class="[
        'px-3.5 h-8 text-[12.5px] font-semibold rounded-lg transition-colors duration-150 focus-ring tracking-chrome',
        modelValue === opt.value
          ? 'bg-ink-900 text-ink-50'
          : 'text-ink-500 hover:text-ink-900',
      ]"
    >{{ opt.label }}</button>
  </div>
</template>
