<script setup lang="ts">
/*
  iOS-style switch. Полностью завязан на CSS-переменные палитры, поэтому
  одинаково корректно выглядит в светлой и тёмной темах:
    – трек off: ink-300 (приглушённая бумага в light, графит в dark)
    – трек on : ink-900 (ink в light, молоко в dark)
    – бегунок : surface (молочно-белый в light, графит в dark) — всегда
                максимально контрастен против трека.
*/
const props = defineProps<{ modelValue: boolean; disabled?: boolean; label?: string }>()
const emit = defineEmits<{ (e: 'update:modelValue', v: boolean): void }>()
const toggle = () => { if (!props.disabled) emit('update:modelValue', !props.modelValue) }
</script>

<template>
  <button
    type="button"
    role="switch"
    :aria-checked="modelValue"
    :aria-label="label"
    :disabled="disabled"
    @click="toggle"
    :class="[
      'relative inline-flex shrink-0 h-7 w-[46px] rounded-full transition-colors duration-200 focus-ring align-middle',
      modelValue ? 'bg-ink-900' : 'bg-ink-300',
      disabled && 'opacity-50 cursor-not-allowed',
    ]"
  >
    <span
      :class="[
        'absolute top-0.5 left-0.5 h-6 w-6 rounded-full bg-surface transition-transform duration-200',
        modelValue && 'translate-x-[18px]',
      ]"
      style="box-shadow: 0 1px 2px rgba(0,0,0,0.18), 0 2px 6px -2px rgba(0,0,0,0.12);"
    />
  </button>
</template>
