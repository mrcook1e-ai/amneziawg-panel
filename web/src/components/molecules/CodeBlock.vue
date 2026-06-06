<script setup lang="ts">
import CopyButton from './CopyButton.vue'
defineProps<{ code: string; lang?: string }>()
</script>

<template>
  <div class="relative group rounded-lg border border-ink-200 bg-ink-50 overflow-hidden">
    <div class="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition">
      <CopyButton :value="code" />
    </div>
    <pre class="overflow-x-auto p-3 text-[12px] font-mono text-ink-900 leading-relaxed"><code>{{ code }}</code></pre>
  </div>
</template>
