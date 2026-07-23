<script setup lang="ts">
import { useToastStore } from '@/stores/toasts'
import Icon from '@/components/atoms/Icon.vue'

/*
  Тосты живут наверху по центру в виде glass-пилюли. Иконка слева
  окрашена по типу: success → зелёный, warning → янтарный, danger → красный.
  Текст и фон остаются нейтральными, чтобы цвет работал как индикатор, а не
  кричал на весь экран.
*/

const toasts = useToastStore()

const iconFor = (kind: string): 'check' | 'x' | 'info' => ({
  info:    'info' as const,
  success: 'check' as const,
  warning: 'info' as const,
  danger:  'x' as const,
}[kind] || 'info')

const iconColor = (kind: string): string => ({
  info:    'text-ink-700',
  success: 'text-success',
  warning: 'text-warning',
  danger:  'text-danger',
}[kind] || 'text-ink-700')
</script>

<template>
  <div class="fixed top-20 inset-x-0 z-[70] flex flex-col items-center gap-2 px-4 pointer-events-none">
    <transition-group
      tag="div" class="flex flex-col items-center gap-2 w-full"
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="opacity-0 -translate-y-2 scale-95"
      enter-to-class="opacity-100 translate-y-0 scale-100"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="opacity-100 scale-100"
      leave-to-class="opacity-0 scale-95"
    >
      <button
        v-for="t in toasts.items" :key="t.id"
        type="button"
        @click="toasts.dismiss(t.id)"
        class="glass rounded-full pl-3 pr-4 h-10 flex items-center gap-2.5 pointer-events-auto max-w-md focus-ring"
      >
        <span
          :class="['inline-flex h-5 w-5 rounded-full items-center justify-center', iconColor(t.kind)]"
        >
          <Icon :name="iconFor(t.kind)" :size="14" />
        </span>
        <span class="text-[13px] font-medium text-ink-900 truncate">{{ t.message }}</span>
      </button>
    </transition-group>
  </div>
</template>
