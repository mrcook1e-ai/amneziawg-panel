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
  <button
    type="button"
    role="menuitem"
    :disabled="disabled"
    class="w-full flex items-center gap-2.5 px-3.5 py-2.5 text-[13px] text-left transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
    :class="tone === 'danger'
      ? 'text-danger hover:bg-danger/10'
      : 'text-ink-700 hover:bg-ink-100 dark:hover:bg-ink-200/50'">
    <slot />
  </button>
</template>
