<script setup lang="ts">
import { bytesParts } from '@/lib/format'
import { computed } from 'vue'

/*
  Большая метрика: eyebrow + крупная цифра + ед. + опц. подпись.
  Подпись (sub) умеет окрашиваться через sub-tone — единственное место,
  где живой статус (online → зелёный) прорывается на главный экран.
*/

type Tone = 'muted' | 'success' | 'warning' | 'danger'

const props = withDefaults(defineProps<{
  eyebrow: string
  value?: number
  numerator?: number
  denominator?: number
  valueStr?: string
  unit?: string
  sub?: string
  subTone?: Tone
  kind?: 'bytes' | 'bytes-per-sec' | 'ratio' | 'raw'
  size?: 'hero' | 'normal' | 'compact'
}>(), { kind: 'bytes', size: 'normal', subTone: 'muted' })

const parts = computed(() => {
  if (props.kind === 'raw') return { v: props.valueStr || '—', u: props.unit || '' }
  if (props.kind === 'ratio') return { v: String(props.numerator ?? 0), u: `из ${props.denominator ?? 0}` }
  const p = bytesParts(props.value || 0)
  return { v: p.value, u: props.kind === 'bytes-per-sec' ? `${p.unit}/с` : p.unit }
})

const sizes = {
  hero:    'text-[88px] sm:text-[110px]',
  normal:  'text-[56px] sm:text-[68px]',
  compact: 'text-[36px]',
}

const subCls = computed(() => ({
  muted:   'text-ink-500',
  success: 'text-success',
  warning: 'text-warning',
  danger:  'text-danger',
}[props.subTone || 'muted']))

const dotCls = computed(() => ({
  muted:   '',
  success: 'bg-success',
  warning: 'bg-warning',
  danger:  'bg-danger',
}[props.subTone || 'muted']))
</script>

<template>
  <div class="flex flex-col gap-2 min-w-0">
    <div class="eyebrow">{{ eyebrow }}</div>
    <div class="flex items-end gap-2 flex-wrap min-w-0">
      <span :class="['num-display tnum text-ink-900', sizes[size]]">{{ parts.v }}</span>
      <span class="mono text-[12px] text-ink-500 pb-2 sm:pb-3 uppercase tracking-wider">{{ parts.u }}</span>
    </div>
    <div v-if="sub" class="flex items-center gap-1.5 -mt-0.5 text-[12.5px]" :class="subCls">
      <span v-if="subTone !== 'muted'" :class="['inline-block h-1.5 w-1.5 rounded-full', dotCls]" />
      <span>{{ sub }}</span>
    </div>
  </div>
</template>
