<script setup lang="ts">
defineProps<{ label?: string; hint?: string; error?: string }>()
</script>

<template>
  <label class="block">
    <!--
      Editorial label: uppercase, tracked, ink-500. Reads as a print-spec
      heading instead of a generic form label. Matches the .eyebrow voice
      used everywhere else in the panel.
    -->
    <span
      v-if="label"
      class="block mb-2 text-[10.5px] uppercase tracking-[0.12em] font-semibold text-ink-500"
    >{{ label }}</span>
    <slot />
    <span v-if="error" class="block mt-2 text-[12px] text-danger">{{ error }}</span>
    <span v-else-if="hint" class="block mt-2 text-[12px] text-ink-500">{{ hint }}</span>
  </label>
</template>
