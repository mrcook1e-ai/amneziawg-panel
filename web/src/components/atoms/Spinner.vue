<script setup lang="ts">
withDefaults(defineProps<{ size?: number }>(), { size: 16 })
</script>

<template>
  <span
    class="inline-block rounded-full border-2 border-ink-300 border-r-transparent animate-spin"
    :style="{ width: size + 'px', height: size + 'px' }"
    role="status"
    aria-label="Loading"
  />
</template>
