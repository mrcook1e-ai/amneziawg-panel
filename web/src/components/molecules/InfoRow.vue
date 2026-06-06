<script setup lang="ts">
defineProps<{
  label: string
  value?: string
  mono?: boolean
  showDivider?: boolean
  trailing?: boolean // when used purely as a slot row with a control on the right
}>()
</script>

<template>
  <div class="relative">
    <div class="flex items-center gap-4 px-4 py-3 min-h-[52px]">
      <div class="text-[14px] text-ink-700 font-medium shrink-0">{{ label }}</div>
      <div class="flex-1 min-w-0 flex justify-end items-center gap-2">
        <span
          v-if="value !== undefined"
          :class="['text-[13px] text-ink-500 truncate', mono && 'font-mono']"
          :title="value"
        >{{ value }}</span>
        <slot />
      </div>
    </div>
    <div v-if="showDivider" class="ml-4 mr-4 border-t border-ink-100/80" />
  </div>
</template>
