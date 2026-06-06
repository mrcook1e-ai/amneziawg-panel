<script setup lang="ts">
defineProps<{ title: string; description?: string }>()
</script>

<template>
  <div class="flex flex-col items-center justify-center text-center py-16 px-6">
    <div class="h-10 w-10 rounded-full bg-ink-100 mb-4" />
    <h3 class="text-[15px] font-semibold text-ink-900">{{ title }}</h3>
    <p v-if="description" class="mt-1.5 max-w-md text-[13px] text-ink-500">{{ description }}</p>
    <div v-if="$slots.action" class="mt-5"><slot name="action" /></div>
  </div>
</template>
