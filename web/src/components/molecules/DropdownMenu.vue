<script setup lang="ts">
/*
  DropdownMenu — anchored popover for short action lists.

  Usage:
    <DropdownMenu align="right" width="w-52">
      <template #trigger="{ open, toggle }">
        <IconButton :class="open ? 'bg-amber-400/15 text-amber-600' : ''" @click="toggle">
          <MoreHorizontal :size="16" />
        </IconButton>
      </template>
      <template #default="{ close }">
        <DropdownItem @click="copy(); close()">Скопировать</DropdownItem>
        <DropdownSeparator />
        <DropdownItem tone="danger" @click="del(); close()">Удалить</DropdownItem>
      </template>
    </DropdownMenu>

  Outside-click and Escape close it. The trigger slot receives `open` so
  the caller can style the active state; the default slot receives
  `close` for items to dismiss after acting. Positioning is via `absolute
  top-full mt-2`, anchored either right or left — the trigger's own
  position controls where the popover lands.
*/
import { ref, onMounted, onBeforeUnmount } from 'vue'

withDefaults(defineProps<{
  /** Which side of the trigger the menu opens from. */
  align?: 'left' | 'right'
  /** Tailwind width class for the menu panel. */
  width?: string
}>(), { align: 'right', width: 'w-52' })

const isOpen  = ref(false)
const rootRef = ref<HTMLElement | null>(null)

function toggle() { isOpen.value = !isOpen.value }
function close()  { isOpen.value = false }

function onDocPointer(e: PointerEvent) {
  if (!rootRef.value?.contains(e.target as Node)) close()
}
function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') close()
}

onMounted(() => {
  document.addEventListener('pointerdown', onDocPointer)
  document.addEventListener('keydown', onKey)
})
onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onDocPointer)
  document.removeEventListener('keydown', onKey)
})

defineExpose({ open: isOpen, close, toggle })
</script>

<template>
  <div ref="rootRef" class="relative inline-block">
    <slot name="trigger" :open="isOpen" :toggle="toggle" />

    <Transition
      enter-active-class="transition duration-120 ease-out"
      enter-from-class="opacity-0 scale-95 -translate-y-1"
      enter-to-class="opacity-100 scale-100 translate-y-0"
      leave-active-class="transition duration-100 ease-in"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0">
      <div
        v-if="isOpen"
        role="menu"
        :class="[
          // Same surface language as cards: bg-surface-raised + shadow-pop.
          // Adds a 1px hairline-equivalent ring so the panel has a defined
          // edge on bright surfaces where shadow alone reads as ambient.
          // py-1 for tight stack (less vertical waste than py-1.5).
          'absolute top-full mt-2 z-50 py-1',
          'rounded-2xl bg-surface-raised shadow-pop',
          'ring-1 ring-ink-900/[0.06] dark:ring-ink-900/20',
          align === 'right' ? 'right-0 origin-top-right' : 'left-0 origin-top-left',
          width,
        ]">
        <slot :close="close" />
      </div>
    </Transition>
  </div>
</template>
