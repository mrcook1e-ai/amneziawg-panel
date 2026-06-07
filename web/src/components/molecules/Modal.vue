<script setup lang="ts">
import { onBeforeUnmount, ref, watch, nextTick } from 'vue'
import Icon from '@/components/atoms/Icon.vue'
import IconButton from '@/components/atoms/IconButton.vue'

const props = withDefaults(defineProps<{
  open: boolean
  title?: string
  size?: 'sm' | 'md' | 'lg'
}>(), { size: 'md' })

const emit = defineEmits<{ (e: 'close'): void }>()

const widths = { sm: 'max-w-sm', md: 'max-w-md', lg: 'max-w-2xl' }
const panelRef = ref<HTMLElement | null>(null)
let lastFocused: HTMLElement | null = null

// ── Modal stack — only the topmost modal reacts to ESC ─────────────────
const modalStack: symbol[] = []
const id = Symbol('modal')

const onKey = (e: KeyboardEvent) => {
  if (!props.open) return
  if (modalStack[modalStack.length - 1] !== id) return
  if (e.key === 'Escape') {
    e.stopPropagation()
    emit('close')
    return
  }
  if (e.key === 'Tab') trapFocus(e)
}

function focusables(): HTMLElement[] {
  const root = panelRef.value
  if (!root) return []
  return Array.from(root.querySelectorAll<HTMLElement>(
    'a[href], button:not([disabled]), input:not([disabled]):not([type="hidden"]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
  )).filter(el => el.offsetParent !== null || el === document.activeElement)
}

function trapFocus(e: KeyboardEvent) {
  const els = focusables()
  if (!els.length) return
  const first = els[0]
  const last  = els[els.length - 1]
  const active = document.activeElement as HTMLElement | null
  if (e.shiftKey && active === first) { e.preventDefault(); last.focus() }
  else if (!e.shiftKey && active === last) { e.preventDefault(); first.focus() }
}

watch(() => props.open, async (open) => {
  if (open) {
    lastFocused = document.activeElement as HTMLElement | null
    modalStack.push(id)
    window.addEventListener('keydown', onKey, true)
    await nextTick()
    // Honour an explicit autofocus inside the modal; otherwise focus first focusable.
    const explicit = panelRef.value?.querySelector<HTMLElement>('[autofocus]')
    const target = explicit ?? focusables()[0] ?? panelRef.value
    target?.focus()
  } else {
    teardown()
  }
})

function teardown() {
  const i = modalStack.indexOf(id)
  if (i >= 0) modalStack.splice(i, 1)
  window.removeEventListener('keydown', onKey, true)
  lastFocused?.focus?.()
  lastFocused = null
}

onBeforeUnmount(teardown)
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
    <div v-if="open" class="fixed inset-0 z-40 scrim" @click="$emit('close')" />
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
      <div
        ref="panelRef"
        :class="['w-full bg-surface rounded-3xl shadow-pop pointer-events-auto overflow-hidden outline-none', widths[size]]"
        role="dialog"
        aria-modal="true"
        :aria-label="title"
        tabindex="-1"
      >
        <header v-if="title || $slots.title" class="flex items-center justify-between gap-4 px-6 pt-5 pb-3">
          <h2 class="text-[19px] font-semibold text-ink-900 tracking-tight">
            <slot name="title">{{ title }}</slot>
          </h2>
          <IconButton size="sm" title="Закрыть" aria-label="Закрыть" @click="$emit('close')"><Icon name="x" :size="16" /></IconButton>
        </header>
        <div class="px-6 pb-5 pt-1"><slot /></div>
        <footer v-if="$slots.footer" class="px-6 py-4 border-t border-ink-200/40 bg-ink-100/40 flex items-center justify-end gap-2">
          <slot name="footer" />
        </footer>
      </div>
    </div>
  </transition>
</template>
