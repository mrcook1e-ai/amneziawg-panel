<script setup lang="ts">
import { computed } from 'vue'
import { bytesParts } from '@/lib/format'

/*
  StatBlock — eyebrow + большая цифра + ед.

  Принимает либо `value` (байты, форматируется через bytesParts), либо `raw`
  (уже отформатированная строка типа "85%"). Вынесено из ClientDetailView
  чтобы не пересоздавать defineComponent на каждый setup-run.
*/
const props = withDefaults(defineProps<{
  eyebrow: string
  value?: number
  raw?: string
}>(), { value: 0, raw: '' })

const parts = computed(() =>
  props.raw ? { value: props.raw, unit: '' } : bytesParts(props.value || 0),
)
</script>

<template>
  <div class="space-y-1">
    <div class="eyebrow truncate">{{ eyebrow }}</div>
    <div class="flex items-baseline gap-1.5">
      <span class="num-display-soft tnum text-ink-900 text-[34px] sm:text-[40px]">{{ parts.value }}</span>
      <span v-if="parts.unit" class="mono text-[10.5px] text-ink-500 uppercase tracking-wider">{{ parts.unit }}</span>
    </div>
  </div>
</template>
