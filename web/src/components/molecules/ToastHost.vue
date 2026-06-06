<script setup lang="ts">
import { useToastStore } from '@/stores/toasts'
import IconButton from '@/components/atoms/IconButton.vue'
import Icon from '@/components/atoms/Icon.vue'

const toasts = useToastStore()

const cls = (kind: string) => ({
  info:    'border-ink-200 bg-white',
  success: 'border-success/30 bg-white',
  warning: 'border-warning/30 bg-white',
  danger:  'border-danger/30 bg-white',
}[kind] || 'border-ink-200 bg-white')

const dot = (kind: string) => ({
  info:    'bg-ink-400',
  success: 'bg-success',
  warning: 'bg-warning',
  danger:  'bg-danger',
}[kind] || 'bg-ink-400')
</script>

<template>
  <div class="fixed bottom-4 right-4 z-[60] flex flex-col gap-2 w-[320px] pointer-events-none">
    <transition-group
      tag="div" class="flex flex-col gap-2"
      enter-active-class="transition duration-150 ease-out"
      enter-from-class="opacity-0 translate-y-2"
      enter-to-class="opacity-100 translate-y-0"
      leave-active-class="transition duration-100 ease-in"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div
        v-for="t in toasts.items" :key="t.id"
        :class="['pointer-events-auto flex items-start gap-3 rounded-xl border shadow-card px-3.5 py-3', cls(t.kind)]"
      >
        <span :class="['mt-1.5 h-2 w-2 shrink-0 rounded-full', dot(t.kind)]" />
        <p class="flex-1 text-[13px] text-ink-800 leading-snug">{{ t.message }}</p>
        <IconButton size="sm" title="Dismiss" @click="toasts.dismiss(t.id)">
          <Icon name="x" :size="14" />
        </IconButton>
      </div>
    </transition-group>
  </div>
</template>
