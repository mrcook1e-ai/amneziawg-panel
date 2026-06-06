<script setup lang="ts">
defineProps<{ title?: string; footer?: string }>()
</script>

<template>
  <section>
    <div v-if="title" class="section-title">{{ title }}</div>
    <div class="card overflow-hidden">
      <slot />
    </div>
    <p v-if="footer" class="mt-2 px-1 text-[11.5px] text-ink-500 leading-snug">{{ footer }}</p>
  </section>
</template>
