<script setup lang="ts">
import { computed } from 'vue'

/*
  StatusDot — единственный «светофор» в основном UI:
    online   → success зелёный  + мягкий ping-ореол
    stale    → warning янтарный + ping
    offline  → нейтральный      без пульса

  Размеры:
    sm  — 6px ядро, inline-индикатор внутри текста
    md  — 8px ядро (по умолчанию), для строк списка
    lg  — 10px ядро + статический outer ring 20%, для status-чипов

  Использовать вместо инлайновых `bg-success animate-pulse` спанов
  и legacy CSS-класса .live-dot. Один источник правды для онлайн-статуса
  на главной, в кабинете, на странице клиента/подписки.
*/

type State = 'online' | 'stale' | 'offline'
type Size  = 'sm' | 'md' | 'lg'

const props = withDefaults(defineProps<{
  state: State
  size?: Size
}>(), { size: 'md' })

const sizes: Record<Size, { box: string; core: string; ping: string }> = {
  sm: { box: 'h-1.5 w-1.5', core: 'h-1.5 w-1.5', ping: '' },
  md: { box: 'h-2   w-2',   core: 'h-2   w-2',   ping: '' },
  lg: {
    // lg ядро 10px с тонкой 2px подложкой того же цвета — это и есть
    // знакомый «live-dot» с двумя слоями.
    box:  'h-2.5 w-2.5',
    core: 'h-2.5 w-2.5',
    ping: '',
  },
}

const color = computed(() => ({
  online:  'bg-success',
  stale:   'bg-warning',
  offline: 'bg-ink-300',
}[props.state]))

const sz = computed(() => sizes[props.size])
</script>

<template>
  <span :class="['relative inline-flex shrink-0', sz.box]">
    <!-- Outer expanding ping — только для online/stale -->
    <span
      v-if="state !== 'offline'"
      :class="['absolute inset-0 rounded-full animate-ping opacity-40', color]"
    />
    <!-- Static halo for lg (gives the dot its own backplate) -->
    <span
      v-if="state !== 'offline' && size === 'lg'"
      :class="['absolute -inset-1 rounded-full opacity-20', color]"
    />
    <!-- Solid core -->
    <span :class="['relative inline-block rounded-full', color, sz.core]" />
  </span>
</template>
