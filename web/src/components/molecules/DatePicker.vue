<script setup lang="ts">
/*
  Свой date picker. Нативный input[type=date] показывает разный popup в каждом
  браузере и не стилизуется. Мы рендерим обычный текстовый input в формате
  ДД.ММ.ГГГГ и поповер с календарём — через <Teleport to="body"> поверх
  всего, поэтому overflow-hidden у родительских карточек не режет его.

  Позиция вычисляется по getBoundingClientRect() инпута: попап садится прямо
  под полем; если упирается в нижний край вьюпорта — переезжает наверх.

  v-model — ISO-строка YYYY-MM-DD.
*/

import { ref, computed, watch, onMounted, onBeforeUnmount, reactive, nextTick } from 'vue'

const props = withDefaults(defineProps<{
  modelValue: string             // ISO YYYY-MM-DD или ''
  placeholder?: string
  disabled?: boolean
  size?: 'sm' | 'md'
  min?: string                   // ISO нижняя граница
}>(), { placeholder: 'дд.мм.гггг', size: 'md' })

const emit = defineEmits<{ (e: 'update:modelValue', v: string): void }>()

const MONTHS = ['Январь','Февраль','Март','Апрель','Май','Июнь','Июль','Август','Сентябрь','Октябрь','Ноябрь','Декабрь']
const WEEKDAYS = ['Пн','Вт','Ср','Чт','Пт','Сб','Вс']

// ─── Текстовое представление ───
function isoToText(iso: string): string {
  if (!iso) return ''
  const [y, m, d] = iso.split('-')
  if (!y || !m || !d) return ''
  return `${d}.${m}.${y}`
}
function textToIso(txt: string): string {
  const m = txt.match(/^(\d{2})\.(\d{2})\.(\d{4})$/)
  if (!m) return ''
  const d = +m[1], mo = +m[2], y = +m[3]
  if (mo < 1 || mo > 12 || d < 1 || d > 31 || y < 1900) return ''
  return `${y.toString().padStart(4, '0')}-${mo.toString().padStart(2, '0')}-${d.toString().padStart(2, '0')}`
}

const text = ref(isoToText(props.modelValue))
watch(() => props.modelValue, v => { text.value = isoToText(v) })

function commitText() {
  if (!text.value) { emit('update:modelValue', ''); return }
  const iso = textToIso(text.value)
  if (iso) emit('update:modelValue', iso)
  else text.value = isoToText(props.modelValue)
}

// ─── Состояние поповера ───
const open = ref(false)
const anchor = ref<HTMLElement | null>(null)   // обёртка-relative с инпутом
const popover = ref<HTMLElement | null>(null)  // сам попап
const pos = reactive({ left: 0, top: 0, width: 300 })

function isoNow(): string {
  const d = new Date()
  return `${d.getFullYear()}-${(d.getMonth() + 1).toString().padStart(2, '0')}-${d.getDate().toString().padStart(2, '0')}`
}

const cursor = ref({ y: new Date().getFullYear(), m: new Date().getMonth() })
function syncCursorFromValue() {
  const v = props.modelValue || isoNow()
  const [y, m] = v.split('-').map(n => parseInt(n, 10))
  if (Number.isFinite(y) && Number.isFinite(m)) cursor.value = { y, m: m - 1 }
}

const POP_W = 304
const POP_H = 360   // приблизительная высота попапа (заголовок + сетка + действия)

async function reposition() {
  const a = anchor.value
  if (!a) return
  const r = a.getBoundingClientRect()
  const viewW = window.innerWidth
  const viewH = window.innerHeight

  // Базово — под инпутом, влево от него.
  let left = r.left
  // Не вылезать вправо.
  if (left + POP_W + 8 > viewW) left = Math.max(8, viewW - POP_W - 8)
  if (left < 8) left = 8

  // Снизу если умещается, иначе сверху.
  let top = r.bottom + 8
  if (top + POP_H > viewH - 8 && r.top - POP_H - 8 > 8) {
    top = r.top - POP_H - 8
  }
  pos.left = left
  pos.top = top
}

async function show() {
  if (props.disabled) return
  syncCursorFromValue()
  open.value = true
  await nextTick()
  reposition()
}
function close() { open.value = false }
function toggle() { open.value ? close() : show() }

function onDocPointer(e: MouseEvent) {
  if (!open.value) return
  const t = e.target as Node
  if (anchor.value?.contains(t)) return
  if (popover.value?.contains(t)) return
  close()
}
function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape' && open.value) close()
}
function onScrollResize() {
  if (open.value) reposition()
}

onMounted(() => {
  document.addEventListener('mousedown', onDocPointer)
  document.addEventListener('keydown', onKey)
  window.addEventListener('resize', onScrollResize)
  window.addEventListener('scroll', onScrollResize, true)
})
onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDocPointer)
  document.removeEventListener('keydown', onKey)
  window.removeEventListener('resize', onScrollResize)
  window.removeEventListener('scroll', onScrollResize, true)
})

// ─── Сетка месяца ───
type Cell = { d: number; iso: string; outside: boolean; today: boolean; selected: boolean; disabled: boolean }

const grid = computed<Cell[]>(() => {
  const { y, m } = cursor.value
  const first = new Date(y, m, 1)
  const lead = (first.getDay() + 6) % 7
  const daysInMonth = new Date(y, m + 1, 0).getDate()
  const todayIso = isoNow()

  const out: Cell[] = []
  const prevDays = new Date(y, m, 0).getDate()
  for (let i = lead - 1; i >= 0; i--) {
    const d = prevDays - i
    out.push(cellFor(new Date(y, m - 1, d), true, todayIso))
  }
  for (let d = 1; d <= daysInMonth; d++) {
    out.push(cellFor(new Date(y, m, d), false, todayIso))
  }
  while (out.length < 42) {
    const idx = out.length - (lead + daysInMonth) + 1
    out.push(cellFor(new Date(y, m + 1, idx), true, todayIso))
  }
  return out
})

function cellFor(dt: Date, outside: boolean, todayIso: string): Cell {
  const iso = `${dt.getFullYear()}-${(dt.getMonth() + 1).toString().padStart(2, '0')}-${dt.getDate().toString().padStart(2, '0')}`
  const disabled = !!props.min && iso < props.min
  return {
    d: dt.getDate(),
    iso,
    outside,
    today: iso === todayIso,
    selected: !!props.modelValue && iso === props.modelValue,
    disabled,
  }
}

function pick(c: Cell) {
  if (c.disabled) return
  emit('update:modelValue', c.iso)
  text.value = isoToText(c.iso)
  close()
}
function shiftMonth(delta: number) {
  let { y, m } = cursor.value
  m += delta
  if (m < 0)   { m = 11; y-- }
  if (m > 11)  { m = 0;  y++ }
  cursor.value = { y, m }
}
function pickToday() {
  const iso = isoNow()
  emit('update:modelValue', iso)
  text.value = isoToText(iso)
  close()
}
function clear() {
  emit('update:modelValue', '')
  text.value = ''
  close()
}

const inputCls = computed(() => [
  'block w-full bg-ink-100 text-ink-900 placeholder-ink-500 rounded-xl border-0 outline-none transition pr-10',
  'focus:bg-ink-200 focus-visible:outline-none',
  props.size === 'sm' ? 'h-9 px-3.5 text-[13.5px]' : 'h-11 px-4 text-[15px]',
  props.disabled && 'opacity-50 cursor-not-allowed',
])
</script>

<template>
  <div ref="anchor" class="relative">
    <input
      :value="text"
      @input="(e) => text = (e.target as HTMLInputElement).value"
      @blur="commitText"
      @focus="show"
      @keydown.enter.prevent="commitText"
      :placeholder="placeholder"
      :disabled="disabled"
      :class="inputCls"
      type="text"
      inputmode="numeric"
      autocomplete="off"
    />
    <button
      type="button"
      class="absolute inset-y-0 right-0 px-3 grid place-items-center text-ink-500 hover:text-ink-900 transition-colors"
      :aria-label="open ? 'Закрыть календарь' : 'Открыть календарь'"
      :disabled="disabled"
      @mousedown.prevent="toggle"
      tabindex="-1"
    >
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <rect x="3" y="5" width="18" height="16" rx="2" />
        <path d="M3 10h18M8 3v4M16 3v4" />
      </svg>
    </button>

    <!--
      Teleport в body чтобы выйти из overflow-hidden родителей (Section/card).
      Позиция выставляется через :style по getBoundingClientRect инпута.
      z-50 поверх скримов модалок (40), но под глобальными toast'ами (70).
    -->
    <Teleport to="body">
      <transition
        enter-active-class="transition duration-150 ease-out"
        enter-from-class="opacity-0 -translate-y-1 scale-[0.98]"
        enter-to-class="opacity-100 translate-y-0 scale-100"
        leave-active-class="transition duration-100 ease-in"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
      >
        <div
          v-if="open"
          ref="popover"
          class="fixed z-50 card p-3 shadow-pop origin-top-left"
          :style="{ left: pos.left + 'px', top: pos.top + 'px', width: POP_W + 'px' }"
        >
          <!-- Заголовок месяца -->
          <div class="flex items-center justify-between px-1 mb-2">
            <div class="text-[13px] text-ink-900 font-semibold tabular-nums tracking-tight">
              {{ MONTHS[cursor.m] }} {{ cursor.y }}
            </div>
            <div class="flex items-center gap-0.5">
              <button
                type="button"
                class="h-7 w-7 rounded-md grid place-items-center text-ink-700 hover:bg-ink-100 transition-colors"
                @click="shiftMonth(-1)"
                aria-label="Предыдущий месяц"
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 6l-6 6 6 6"/></svg>
              </button>
              <button
                type="button"
                class="h-7 w-7 rounded-md grid place-items-center text-ink-700 hover:bg-ink-100 transition-colors"
                @click="shiftMonth(1)"
                aria-label="Следующий месяц"
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 6l6 6-6 6"/></svg>
              </button>
            </div>
          </div>

          <!-- Дни недели -->
          <div class="grid grid-cols-7 mb-1">
            <div
              v-for="w in WEEKDAYS"
              :key="w"
              class="h-7 grid place-items-center text-[10px] uppercase tracking-[0.1em] text-ink-500 font-medium"
            >{{ w }}</div>
          </div>

          <!-- Сетка -->
          <div class="grid grid-cols-7 gap-y-0.5">
            <button
              v-for="(c, i) in grid"
              :key="i"
              type="button"
              :disabled="c.disabled"
              @click="pick(c)"
              class="h-9 grid place-items-center text-[13px] tabular-nums rounded-md transition-colors"
              :class="[
                c.selected   && 'bg-ink-900 text-ink-50 font-semibold',
                !c.selected  && c.today    && 'ring-1 ring-ink-900/20 text-ink-900',
                !c.selected  && !c.outside && !c.disabled && 'text-ink-800 hover:bg-ink-100',
                !c.selected  &&  c.outside && 'text-ink-400 hover:bg-ink-100',
                c.disabled   && 'opacity-30 cursor-not-allowed',
              ]"
            >{{ c.d }}</button>
          </div>

          <!-- Действия -->
          <div class="mt-2 pt-2 flex items-center justify-between border-t border-ink-900/10">
            <button
              type="button"
              class="h-7 px-2 rounded-md text-[12px] text-ink-500 hover:bg-ink-100 hover:text-ink-900 transition-colors"
              @click="clear"
            >Очистить</button>
            <button
              type="button"
              class="h-7 px-2 rounded-md text-[12px] text-ink-900 hover:bg-ink-100 transition-colors font-medium"
              @click="pickToday"
            >Сегодня</button>
          </div>
        </div>
      </transition>
    </Teleport>
  </div>
</template>
