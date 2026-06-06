<script setup lang="ts">
defineProps<{ label?: string; hint?: string; error?: string }>()
</script>

<template>
  <label class="block">
    <span v-if="label" class="block mb-1.5 text-[12px] font-medium text-ink-700">{{ label }}</span>
    <slot />
    <span v-if="error" class="block mt-1.5 text-[12px] text-danger">{{ error }}</span>
    <span v-else-if="hint" class="block mt-1.5 text-[12px] text-ink-500">{{ hint }}</span>
  </label>
</template>
