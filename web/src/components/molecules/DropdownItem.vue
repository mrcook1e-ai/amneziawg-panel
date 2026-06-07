<script setup lang="ts">
/*
  Single item inside <DropdownMenu>. Renders as a full-width button row.
  tone="danger" tints the row red on hover (used for destructive actions
  like "Удалить устройство"); default tone is ink-700 with ink-100 hover.
*/
withDefaults(defineProps<{
  tone?: 'neutral' | 'danger'
  disabled?: boolean
}>(), { tone: 'neutral' })
</script>

<template>
  <!--
    Tight rows (h-9 ≈ 36px) — matches the iOS/macOS context-menu cadence.
    Items get an inner 4px gutter so the hover-fill doesn't touch the
    panel edges (matches the elevated card aesthetic — content sits ON
    the surface, not flush to its border).
  -->
  <div class="px-1">
    <button
      type="button"
      role="menuitem"
      :disabled="disabled"
      class="w-full flex items-center gap-2.5 px-3 h-9 rounded-xl text-[13px] text-left transition-colors duration-100 disabled:opacity-50 disabled:cursor-not-allowed"
      :class="tone === 'danger'
        ? 'text-danger hover:bg-danger/10 active:bg-danger/15'
        : 'text-ink-800 hover:bg-ink-100 dark:hover:bg-ink-200/50 active:bg-ink-200'">
      <slot />
    </button>
  </div>
</template>
