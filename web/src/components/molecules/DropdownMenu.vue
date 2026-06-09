<script setup lang="ts">
/*
  DropdownMenu — anchored popover for short action lists.

  Panel is rendered via Teleport to <body> so it escapes any parent
  stacking context (e.g. transform on .device-card:hover). Position is
  computed from the trigger's getBoundingClientRect on each open.

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
*/
import { ref, onMounted, onBeforeUnmount } from 'vue'

const props = withDefaults(defineProps<{
  /** Which side of the trigger the menu opens from. */
  align?: 'left' | 'right'
  /** Tailwind width class for the menu panel. */
  width?: string
}>(), { align: 'right', width: 'w-52' })

const isOpen     = ref(false)
const triggerRef = ref<HTMLElement | null>(null)
const panelRef   = ref<HTMLElement | null>(null)
const panelStyle = ref<Record<string, string>>({})

function computePos() {
  const el = triggerRef.value
  if (!el) return
  const r = el.getBoundingClientRect()
  const top = r.bottom + window.scrollY + 8
  if (props.align === 'right') {
    panelStyle.value = {
      position: 'fixed',
      top:   `${r.bottom + 8}px`,
      right: `${window.innerWidth - r.right}px`,
    }
  } else {
    panelStyle.value = {
      position: 'fixed',
      top:  `${r.bottom + 8}px`,
      left: `${r.left}px`,
    }
  }
  void top // suppress unused-var lint
}

function toggle() {
  if (!isOpen.value) computePos()
  isOpen.value = !isOpen.value
}
function close() { isOpen.value = false }

function onDocPointer(e: PointerEvent) {
  const t = e.target as Node
  if (!triggerRef.value?.contains(t) && !panelRef.value?.contains(t)) close()
}
function onKey(e: KeyboardEvent) { if (e.key === 'Escape') close() }
function onScroll() { if (isOpen.value) computePos() }

onMounted(() => {
  document.addEventListener('pointerdown', onDocPointer)
  document.addEventListener('keydown', onKey)
  window.addEventListener('scroll', onScroll, { passive: true })
  window.addEventListener('resize', computePos)
})
onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onDocPointer)
  document.removeEventListener('keydown', onKey)
  window.removeEventListener('scroll', onScroll)
  window.removeEventListener('resize', computePos)
})

defineExpose({ open: isOpen, close, toggle })
</script>

<template>
  <div ref="triggerRef" class="relative inline-block">
    <slot name="trigger" :open="isOpen" :toggle="toggle" />

    <Teleport to="body">
      <Transition
        enter-active-class="transition duration-120 ease-out"
        enter-from-class="opacity-0 scale-95 -translate-y-1"
        enter-to-class="opacity-100 scale-100 translate-y-0"
        leave-active-class="transition duration-100 ease-in"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0">
        <div
          v-if="isOpen"
          ref="panelRef"
          role="menu"
          :style="panelStyle"
          :class="[
            'z-[200] py-1',
            'rounded-2xl shadow-pop',
            'ring-1 ring-ink-900/[0.06] dark:ring-ink-900/20',
            align === 'right' ? 'origin-top-right' : 'origin-top-left',
            width,
          ]"
          style="background-color: rgb(var(--surface-raised));">
          <slot :close="close" />
        </div>
      </Transition>
    </Teleport>
  </div>
</template>
