<script setup lang="ts">
import { computed } from 'vue'

/*
  Полоса пульса под TopBar.

  Один пиксель: трек ink-900/10, поверх — индикатор шириной
  √(rate / peak). Когда трафика нет — приглушённый ink-сегмент,
  когда трафик идёт — окрашивается в success и мягко пульсирует.

  Это единственное место, где «онлайн» прорывается на весь экран —
  и оно работает как индикатор живости панели.
*/

const props = withDefaults(defineProps<{
  rate: number      // байт/с
  peak?: number     // калибровка — что считать 100%
}>(), { peak: 5_000_000 })

const width = computed(() => {
  if (!props.rate || props.rate <= 0) return 0.06
  const r = Math.min(1, Math.sqrt(props.rate / props.peak))
  return Math.max(0.06, r)
})

const idle = computed(() => props.rate < 1024)
</script>

<template>
  <div class="px-4">
    <div class="max-w-5xl mx-auto">
      <div class="relative h-px">
        <div class="absolute inset-0 bg-ink-900/10" />
        <div
          class="absolute inset-y-0 left-0 origin-left transition-all duration-700 ease-out"
          :class="[
            idle ? 'bg-ink-900/40' : 'bg-success animate-pulse',
          ]"
          :style="{ width: (width * 100).toFixed(2) + '%' }"
        />
      </div>
    </div>
  </div>
</template>
