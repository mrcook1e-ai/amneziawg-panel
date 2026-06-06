<script setup lang="ts">
import { computed, ref } from 'vue'
import type { SeriesPoint } from '@/types'
import { bytes } from '@/lib/format'

/*
  Двухпоточный sparkline (rx / tx) + интерактивный курсор.

  Контейнер абсолютно позиционируется так, чтобы курсорная линия и тултип
  могли уехать выше канвы, не растягивая родительский grid.

  Стрелка курсора: thin vertical hairline + точки на обеих линиях
  + tooltip pill с timestamp и значениями.
*/

const props = withDefaults(defineProps<{
  points: SeriesPoint[]
  height?: number
  showAxis?: boolean
}>(), { height: 120, showAxis: true })

const W = 1000
const H = 100

function pathFor(getY: (p: SeriesPoint) => number, scale: number) {
  if (!props.points.length) return ''
  const pts = props.points
  const stepX = W / Math.max(1, pts.length - 1)
  const ys = pts.map(p => H - (getY(p) / scale) * (H - 4))
  const xs = pts.map((_, i) => i * stepX)
  let d = `M ${xs[0].toFixed(2)} ${ys[0].toFixed(2)}`
  for (let i = 1; i < pts.length; i++) {
    const mx = (xs[i - 1] + xs[i]) / 2
    const my = (ys[i - 1] + ys[i]) / 2
    d += ` Q ${xs[i - 1].toFixed(2)} ${ys[i - 1].toFixed(2)} ${mx.toFixed(2)} ${my.toFixed(2)}`
  }
  d += ` T ${xs[xs.length - 1].toFixed(2)} ${ys[ys.length - 1].toFixed(2)}`
  return d
}

function areaFor(getY: (p: SeriesPoint) => number, scale: number) {
  const line = pathFor(getY, scale)
  if (!line) return ''
  return `${line} L ${W} ${H} L 0 ${H} Z`
}

const scale = computed(() => {
  let max = 0
  for (const p of props.points) {
    if (p.rx > max) max = p.rx
    if (p.tx > max) max = p.tx
  }
  return max || 1
})

const rxLine = computed(() => pathFor(p => p.rx, scale.value))
const txLine = computed(() => pathFor(p => p.tx, scale.value))
const rxArea = computed(() => areaFor(p => p.rx, scale.value))
const txArea = computed(() => areaFor(p => p.tx, scale.value))

const axis = computed(() => {
  const n = props.points.length
  if (n < 2) return [] as { x: number; label: string }[]
  const fmt = (ts: number) => {
    const d = new Date(ts * 1000)
    const h = d.getHours().toString().padStart(2, '0')
    const m = d.getMinutes().toString().padStart(2, '0')
    return `${h}:${m}`
  }
  return [
    { x: 0,         label: fmt(props.points[0].ts) },
    { x: W * 0.5,   label: fmt(props.points[Math.floor(n / 2)].ts) },
    { x: W,         label: fmt(props.points[n - 1].ts) },
  ]
})

// ─── Курсор / тултип ───
const wrap = ref<HTMLElement | null>(null)
const hoverIdx = ref<number | null>(null)
const mouseX = ref(0)   // px относительно wrap

function onMove(e: MouseEvent) {
  const el = wrap.value
  const pts = props.points
  if (!el || pts.length < 2) return
  const rect = el.getBoundingClientRect()
  const x = e.clientX - rect.left
  const px = Math.max(0, Math.min(rect.width, x))
  const ratio = px / rect.width
  const idx = Math.round(ratio * (pts.length - 1))
  hoverIdx.value = idx
  mouseX.value = px
}
function onLeave() {
  hoverIdx.value = null
}

const cursor = computed(() => {
  if (hoverIdx.value === null || !props.points.length) return null
  const idx = hoverIdx.value
  const pts = props.points
  const p = pts[idx]
  const stepX = 1 / Math.max(1, pts.length - 1)
  const xRel = idx * stepX           // 0..1
  const rxYRel = 1 - (p.rx / scale.value) * (1 - 4 / H)
  const txYRel = 1 - (p.tx / scale.value) * (1 - 4 / H)
  return {
    xPct: (xRel * 100).toFixed(2) + '%',
    rxYPct: (rxYRel * 100).toFixed(2) + '%',
    txYPct: (txYRel * 100).toFixed(2) + '%',
    p,
  }
})

function fmtTime(ts: number): string {
  const d = new Date(ts * 1000)
  const h = d.getHours().toString().padStart(2, '0')
  const m = d.getMinutes().toString().padStart(2, '0')
  return `${h}:${m}`
}

// Тултип хочет сесть около курсора, но не вылезти за края канвы.
const tipStyle = computed(() => {
  if (hoverIdx.value === null || !wrap.value) return { display: 'none' }
  const rect = wrap.value.getBoundingClientRect()
  const tipW = 152
  let left = mouseX.value + 10
  if (left + tipW > rect.width - 4) left = mouseX.value - tipW - 10
  if (left < 4) left = 4
  return {
    left: `${left}px`,
    top: '4px',
  } as Record<string, string>
})
</script>

<template>
  <div class="w-full">
    <div
      ref="wrap"
      class="relative w-full select-none"
      :style="{ height: height + 'px' }"
      @mousemove="onMove"
      @mouseleave="onLeave"
    >
      <svg
        :viewBox="`0 0 ${W} ${H}`"
        preserveAspectRatio="none"
        class="absolute inset-0 w-full h-full pointer-events-none"
      >
        <!-- базовая линия -->
        <line :x1="0" :x2="W" :y1="H - 0.5" :y2="H - 0.5"
              stroke="rgb(var(--ink-900) / 0.10)" stroke-width="0.5" vector-effect="non-scaling-stroke" />

        <!-- tx — тоньше + чуть приглушённее -->
        <path :d="txArea" fill="rgb(var(--ink-900) / 0.04)" />
        <path :d="txLine" fill="none" stroke="rgb(var(--ink-900) / 0.32)"
              stroke-width="1" vector-effect="non-scaling-stroke" />

        <!-- rx — главный -->
        <path :d="rxArea" fill="rgb(var(--ink-900) / 0.07)" />
        <path :d="rxLine" fill="none" stroke="rgb(var(--ink-900) / 0.85)"
              stroke-width="1.4" vector-effect="non-scaling-stroke" />
      </svg>

      <!-- Hairline-курсор + точки на обеих линиях -->
      <template v-if="cursor">
        <div
          class="absolute top-0 bottom-0 w-px bg-ink-900/30 pointer-events-none"
          :style="{ left: cursor.xPct, transform: 'translateX(-0.5px)' }"
        />
        <span
          class="absolute h-2 w-2 -ml-1 -mt-1 rounded-full bg-ink-900 ring-2 ring-surface pointer-events-none"
          :style="{ left: cursor.xPct, top: cursor.rxYPct }"
        />
        <span
          class="absolute h-1.5 w-1.5 -ml-[3px] -mt-[3px] rounded-full bg-ink-500 ring-2 ring-surface pointer-events-none"
          :style="{ left: cursor.xPct, top: cursor.txYPct }"
        />

        <!-- Тултип-пилюля -->
        <div
          class="absolute z-10 card px-3 py-2 pointer-events-none shadow-pop"
          :style="tipStyle"
          style="width: 152px;"
        >
          <div class="eyebrow tnum">{{ fmtTime(cursor.p.ts) }}</div>
          <div class="mt-1.5 flex items-center justify-between gap-2 text-[12px] tnum">
            <span class="text-ink-500">↓ вход.</span>
            <span class="text-ink-900 font-medium mono">{{ bytes(cursor.p.rx) }}</span>
          </div>
          <div class="flex items-center justify-between gap-2 text-[12px] tnum">
            <span class="text-ink-500">↑ исход.</span>
            <span class="text-ink-700 mono">{{ bytes(cursor.p.tx) }}</span>
          </div>
        </div>
      </template>
    </div>

    <div v-if="showAxis && axis.length" class="mt-2 flex justify-between text-[10.5px] text-ink-500 tracking-wider uppercase">
      <span v-for="a in axis" :key="a.x" class="mono">{{ a.label }}</span>
    </div>
  </div>
</template>
