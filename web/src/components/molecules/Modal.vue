<script setup lang="ts">
import { onMounted, onBeforeUnmount } from 'vue'
import Icon from '@/components/atoms/Icon.vue'
import IconButton from '@/components/atoms/IconButton.vue'

const props = withDefaults(defineProps<{
  open: boolean
  title?: string
  size?: 'sm' | 'md' | 'lg'
}>(), { size: 'md' })

const emit = defineEmits<{ (e: 'close'): void }>()

const widths = { sm: 'max-w-sm', md: 'max-w-md', lg: 'max-w-2xl' }
const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape' && props.open) emit('close') }

onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <transition
    enter-active-class="transition duration-150"
    enter-from-class="opacity-0"
    enter-to-class="opacity-100"
    leave-active-class="transition duration-100"
    leave-from-class="opacity-100"
    leave-to-class="opacity-0"
  >
    <div v-if="open" class="fixed inset-0 z-40 bg-ink-950/50 backdrop-blur-[2px]" @click="$emit('close')" />
  </transition>

  <transition
    enter-active-class="transition duration-150 ease-out"
    enter-from-class="opacity-0 translate-y-2 scale-[0.98]"
    enter-to-class="opacity-100 translate-y-0 scale-100"
    leave-active-class="transition duration-100 ease-in"
    leave-from-class="opacity-100"
    leave-to-class="opacity-0"
  >
    <div
      v-if="open"
      class="fixed inset-0 z-50 flex items-start sm:items-center justify-center p-4 pointer-events-none"
    >
      <div :class="['w-full bg-white rounded-xl shadow-pop border border-ink-200 pointer-events-auto', widths[size]]" role="dialog" aria-modal="true">
        <header v-if="title || $slots.title" class="flex items-center justify-between gap-4 px-5 py-3.5 border-b border-ink-100">
          <h2 class="text-[15px] font-semibold text-ink-900">
            <slot name="title">{{ title }}</slot>
          </h2>
          <IconButton size="sm" title="Close" @click="$emit('close')"><Icon name="x" :size="16" /></IconButton>
        </header>
        <div class="px-5 py-4"><slot /></div>
        <footer v-if="$slots.footer" class="px-5 py-3.5 border-t border-ink-100 bg-ink-50/60 rounded-b-xl flex items-center justify-end gap-2">
          <slot name="footer" />
        </footer>
      </div>
    </div>
  </transition>
</template>
