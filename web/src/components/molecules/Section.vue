<script setup lang="ts">
defineProps<{ title?: string; footer?: string }>()
</script>

<template>
  <section>
    <div v-if="title" class="flex items-center gap-4 mb-3 px-1">
      <div class="eyebrow">{{ title }}</div>
      <div class="hairline flex-1" />
    </div>
    <div class="card overflow-hidden">
      <slot />
    </div>
    <p v-if="footer" class="mt-2.5 px-1 text-[11.5px] text-ink-500 leading-relaxed">{{ footer }}</p>
  </section>
</template>
