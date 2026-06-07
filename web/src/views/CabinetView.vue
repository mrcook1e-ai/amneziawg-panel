<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '@/lib/api'
import type { CabinetView, CabinetDevice, AddDeviceResult } from '@/types'
import { genCfg, type Intensity } from '@/utils/generator'

const route = useRoute()
const token = computed(() => String(route.params.token || ''))

type Phase = 'loading' | 'invalid' | 'ready'
const phase   = ref<Phase>('loading')
const cabinet = ref<CabinetView | null>(null)

const addOpen   = ref(false)
const addBusy   = ref(false)
const deviceName = ref('')
const snippet    = ref('')
const addErr     = ref('')
const justAdded  = ref<AddDeviceResult | null>(null)

const deleteFor  = ref<CabinetDevice | null>(null)
const deleteBusy = ref(false)

const genIntensity = ref<Intensity>('medium')

async function reload() {
  try {
    cabinet.value = await api.cabinetGet(token.value)
    phase.value = 'ready'
  } catch {
    phase.value = 'invalid'
  }
}

onMounted(reload)

function openAdd() {
  deviceName.value = ''
  snippet.value    = ''
  addErr.value     = ''
  justAdded.value  = null
  addOpen.value    = true
}

function closeAdd() {
  addOpen.value   = false
  justAdded.value = null
}

async function submitAdd() {
  addErr.value = ''
  if (!snippet.value.trim()) { addErr.value = 'Сначала сгенерируйте или вставьте параметры обфускации'; return }
  addBusy.value = true
  try {
    justAdded.value = await api.cabinetAddDevice(token.value, {
      snippet:    snippet.value.trim(),
      deviceName: deviceName.value.trim() || 'устройство',
    })
    await reload()
  } catch (e: any) {
    addErr.value = e?.message || 'Что-то пошло не так, попробуйте ещё раз'
  } finally {
    addBusy.value = false
  }
}

function downloadJustAdded() {
  if (!justAdded.value) return
  const blob = new Blob([justAdded.value.conf], { type: 'text/plain' })
  const url  = URL.createObjectURL(blob)
  const a    = document.createElement('a')
  a.href     = url
  a.download = `${justAdded.value.name.replace(/[^a-zA-Z0-9-_]+/g, '-')}.conf`
  document.body.appendChild(a); a.click(); a.remove()
  URL.revokeObjectURL(url)
}

const copied = ref(false)
async function copyJustAdded() {
  if (!justAdded.value) return
  try {
    await navigator.clipboard.writeText(justAdded.value.conf)
    copied.value = true
    setTimeout(() => { copied.value = false }, 2000)
  } catch { /* ignore */ }
}

function generateSnippet() {
  const cfg = genCfg({
    version:        '2.0',
    intensity:      genIntensity.value,
    profile:        'quic_initial',
    customHost:     '',
    mimicAll:       false,
    useTagC:        false,
    useTagT:        true,
    useTagR:        true,
    useTagRC:       true,
    useTagRD:       true,
    useBrowserFp:   false,
    browserProfile: '',
    mtu:            1500,
    junkLevel:      5,
    iterCount:      0,
    routerMode:     false,
    useExtremeMax:  false,
  })
  snippet.value = [
    '[Interface]',
    `H1 = ${cfg.h1}`, `H2 = ${cfg.h2}`, `H3 = ${cfg.h3}`, `H4 = ${cfg.h4}`,
    `S1 = ${cfg.s1}`, `S2 = ${cfg.s2}`, `S3 = ${cfg.s3}`, `S4 = ${cfg.s4}`,
    `Jc = ${cfg.jc}`, `Jmin = ${cfg.jmin}`, `Jmax = ${cfg.jmax}`,
    `I1 = ${cfg.i1}`, `I2 = ${cfg.i2}`, `I3 = ${cfg.i3}`, `I4 = ${cfg.i4}`, `I5 = ${cfg.i5}`,
  ].join('\n')
}

function deviceConfUrl(devId: string) { return api.cabinetDeviceConfUrl(token.value, devId) }
function deviceQrUrl(devId: string)   { return api.cabinetDeviceQrUrl(token.value, devId) }

async function confirmDelete() {
  if (!deleteFor.value) return
  deleteBusy.value = true
  try {
    await api.cabinetDeleteDevice(token.value, deleteFor.value.id)
    deleteFor.value = null
    await reload()
  } finally { deleteBusy.value = false }
}

type DeviceStatus = 'online' | 'recent' | 'away' | 'never'
function deviceStatus(d: CabinetDevice): DeviceStatus {
  if (!d.latestHandshakeAt) return 'never'
  const diff = Date.now() - new Date(d.latestHandshakeAt).getTime()
  if (diff < 3 * 60_000)  return 'online'
  if (diff < 60 * 60_000) return 'recent'
  return 'away'
}

function relTime(s?: string | null): string {
  if (!s) return 'никогда'
  try {
    const diff = Date.now() - new Date(s).getTime()
    if (diff < 60_000)    return 'только что'
    if (diff < 3600_000)  return `${Math.floor(diff / 60_000)} мин назад`
    if (diff < 86400_000) return `${Math.floor(diff / 3600_000)} ч назад`
    return `${Math.floor(diff / 86400_000)} д назад`
  } catch { return s }
}

const intensityLabels: Record<Intensity, string> = {
  low:    'Низкая',
  medium: 'Средняя',
  high:   'Высокая',
}
</script>

<template>
  <div class="min-h-screen bg-ink-50 text-ink-900 antialiased">

    <!-- ─── Loading ─────────────────────────────────────────────────── -->
    <div v-if="phase === 'loading'" class="min-h-screen flex items-center justify-center">
      <div class="flex flex-col items-center gap-4">
        <div class="w-10 h-10 rounded-full border-2 border-ink-200 border-t-ink-500 animate-spin" />
        <p class="text-[13px] text-ink-400">Загружаем кабинет…</p>
      </div>
    </div>

    <!-- ─── Invalid ──────────────────────────────────────────────────── -->
    <div v-else-if="phase === 'invalid'" class="min-h-screen flex items-center justify-center p-6">
      <div class="card w-full max-w-sm p-8 text-center space-y-3">
        <div class="w-14 h-14 rounded-full bg-danger/10 flex items-center justify-center mx-auto text-[24px]">🔒</div>
        <h1 class="text-[17px] font-semibold text-ink-900">Кабинет недоступен</h1>
        <p class="text-[13px] text-ink-500 leading-relaxed">
          Ссылка не существует или была отозвана. Свяжитесь с администратором — он выпустит новую.
        </p>
      </div>
    </div>

    <!-- ─── Ready ────────────────────────────────────────────────────── -->
    <template v-else-if="cabinet">
      <div class="max-w-lg mx-auto px-4 pt-12 pb-20">

        <!-- Header -->
        <header class="mb-10 animate-rise">
          <p class="eyebrow text-center mb-2">Личный кабинет · AmneziaWG</p>
          <h1 class="num-display text-[38px] sm:text-[46px] text-center leading-none mb-1">
            {{ cabinet.name }}
          </h1>
          <p class="text-center text-[13px] text-ink-400 mt-3">
            {{ cabinet.devices.length === 0
               ? 'Устройств пока нет'
               : `${cabinet.devices.length} устр${cabinet.devices.length === 1 ? 'ойство' : cabinet.devices.length < 5 ? 'ойства' : 'ойств'}` }}
          </p>
        </header>

        <!-- ── Empty state ── -->
        <div v-if="!cabinet.devices.length"
             class="card p-10 text-center space-y-5 animate-rise">
          <div class="w-16 h-16 rounded-full bg-ink-100 flex items-center justify-center mx-auto text-[28px]">
            📱
          </div>
          <div class="space-y-1.5">
            <p class="text-[15px] font-semibold text-ink-800">Добавьте первое устройство</p>
            <p class="text-[13px] text-ink-500 leading-relaxed max-w-xs mx-auto">
              Подключите телефон, ноутбук или любой другой гаджет — каждое получит свой конфиг.
            </p>
          </div>
          <button
            class="inline-flex items-center gap-2 px-6 py-3 rounded-2xl bg-ink-900 text-white text-[14px] font-semibold hover:bg-ink-800 active:scale-[0.97] transition-all"
            @click="openAdd">
            <span class="text-[16px]">+</span>
            Добавить устройство
          </button>
        </div>

        <!-- ── Devices list ── -->
        <section v-else class="space-y-3">

          <div v-for="(d, i) in cabinet.devices" :key="d.id"
               class="card p-5 animate-rise"
               :class="`delay-${Math.min(i + 1, 6)}`">
            <div class="flex items-start gap-4">

              <!-- Status dot -->
              <div class="pt-0.5 shrink-0">
                <span v-if="deviceStatus(d) === 'online'"
                      class="block w-2.5 h-2.5 rounded-full bg-success animate-pulse mt-1" />
                <span v-else-if="deviceStatus(d) === 'recent'"
                      class="block w-2.5 h-2.5 rounded-full bg-warning mt-1" />
                <span v-else
                      class="block w-2.5 h-2.5 rounded-full bg-ink-300 mt-1" />
              </div>

              <!-- Info -->
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-2 flex-wrap">
                  <span class="text-[15px] font-semibold text-ink-900 truncate">{{ d.name }}</span>
                  <span v-if="!d.enabled"
                        class="text-[10px] uppercase tracking-[0.1em] font-semibold text-danger bg-danger/10 px-1.5 py-0.5 rounded-md">
                    отключено
                  </span>
                </div>
                <div class="flex items-center gap-2 mt-1 flex-wrap">
                  <span class="mono text-[11.5px] text-ink-500">{{ d.address }}</span>
                  <span class="text-ink-300 text-[10px]">·</span>
                  <span class="text-[11.5px] text-ink-400">{{ relTime(d.latestHandshakeAt) }}</span>
                </div>
              </div>

              <!-- Actions -->
              <div class="flex items-center gap-1 shrink-0">
                <a :href="deviceConfUrl(d.id)"
                   :download="`${d.name.replace(/[^a-zA-Z0-9-_]+/g, '-')}.conf`"
                   class="h-8 px-3 flex items-center rounded-xl bg-ink-100 text-ink-700 text-[12px] font-medium hover:bg-ink-200 transition-colors">
                  .conf
                </a>
                <a :href="deviceQrUrl(d.id)" target="_blank" rel="noopener"
                   class="h-8 w-8 flex items-center justify-center rounded-xl bg-ink-100 text-ink-700 hover:bg-ink-200 transition-colors text-[14px]">
                  ▣
                </a>
                <button
                  class="h-8 w-8 flex items-center justify-center rounded-xl text-ink-400 hover:bg-danger/10 hover:text-danger transition-colors text-[14px]"
                  @click="deleteFor = d">
                  ✕
                </button>
              </div>

            </div>
          </div>

          <!-- Add more button -->
          <button
            class="w-full py-4 rounded-2xl border-2 border-dashed border-ink-200 text-ink-400 text-[13px] font-medium hover:border-ink-400 hover:text-ink-600 transition-colors mt-1"
            @click="openAdd">
            + Добавить устройство
          </button>

        </section>

        <!-- Footer note -->
        <p class="text-center text-[11px] text-ink-400 mt-10">
          Потеряли ссылку? Попросите администратора выпустить новую.
        </p>

      </div>
    </template>

    <!-- ─── Add-device modal ──────────────────────────────────────────── -->
    <Teleport to="body">
      <div v-if="addOpen"
           class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4"
           @click.self="closeAdd">

        <!-- Scrim -->
        <div class="absolute inset-0 scrim" @click="closeAdd" />

        <!-- Sheet -->
        <div class="relative w-full sm:max-w-lg bg-surface rounded-t-[28px] sm:rounded-[28px] shadow-pop overflow-hidden">

          <!-- ── Success state ── -->
          <div v-if="justAdded" class="p-6 space-y-5">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-3">
                <div class="w-9 h-9 rounded-full bg-success/15 flex items-center justify-center text-[18px]">✓</div>
                <div>
                  <p class="text-[15px] font-semibold text-ink-900">Готово!</p>
                  <p class="text-[12px] text-ink-500">{{ justAdded.name }}</p>
                </div>
              </div>
              <button class="w-8 h-8 rounded-full hover:bg-ink-100 flex items-center justify-center text-ink-500 transition-colors" @click="closeAdd">✕</button>
            </div>

            <p class="text-[12.5px] text-ink-500 leading-relaxed">
              Сохраните конфиг прямо сейчас — QR-код и файл всегда доступны в списке устройств через кнопку <code class="mono text-[11px]">.conf</code>.
            </p>

            <!-- QR -->
            <div class="flex flex-col items-center gap-3 py-2">
              <div class="p-3 bg-white rounded-3xl border border-ink-900/6 shadow-card inline-block">
                <img :src="`data:image/png;base64,${justAdded.qrPng64}`" alt="QR-код" class="block w-[180px] h-[180px]" />
              </div>
              <p class="text-[11px] text-ink-400">Отсканируйте в приложении AmneziaVPN</p>
            </div>

            <!-- Download / Copy -->
            <div class="flex gap-2">
              <button
                class="flex-1 py-3 rounded-2xl bg-ink-900 text-white text-[13px] font-semibold hover:bg-ink-800 active:scale-[0.98] transition-all"
                @click="downloadJustAdded">
                ⬇ Скачать .conf
              </button>
              <button
                class="flex-1 py-3 rounded-2xl bg-ink-100 text-ink-800 text-[13px] font-semibold hover:bg-ink-200 active:scale-[0.98] transition-all"
                :class="{ 'bg-success/15 text-success': copied }"
                @click="copyJustAdded">
                {{ copied ? '✓ Скопировано' : '📋 Скопировать' }}
              </button>
            </div>
          </div>

          <!-- ── Form state ── -->
          <div v-else class="p-6 space-y-5">
            <div class="flex items-center justify-between">
              <h3 class="text-[17px] font-semibold">Новое устройство</h3>
              <button class="w-8 h-8 rounded-full hover:bg-ink-100 flex items-center justify-center text-ink-500 transition-colors" @click="closeAdd">✕</button>
            </div>

            <!-- Device name -->
            <div class="space-y-2">
              <label class="text-[12px] font-medium text-ink-600">Название устройства</label>
              <input
                v-model="deviceName"
                class="w-full h-12 px-4 rounded-2xl bg-ink-100/70 border border-ink-900/8 text-[14px] text-ink-900 placeholder:text-ink-400 focus-ring transition-colors"
                placeholder="iPhone / MacBook / ноут жены"
                autofocus />
            </div>

            <!-- Obfuscation generator -->
            <div class="rounded-2xl bg-ink-50 border border-ink-900/6 p-4 space-y-3">
              <div class="flex items-center justify-between">
                <p class="text-[12px] font-semibold text-ink-700">Параметры обфускации</p>
                <span class="text-[10px] text-ink-400 uppercase tracking-[0.1em] font-medium">AWG 2.0</span>
              </div>

              <p class="text-[11.5px] text-ink-500 leading-relaxed">
                Уникальный набор для каждого устройства — не используйте один сниппет на нескольких.
              </p>

              <!-- Intensity + Generate -->
              <div class="flex items-center gap-2">
                <div class="flex rounded-xl border border-ink-200 overflow-hidden text-[11.5px] font-semibold shrink-0">
                  <button
                    v-for="lvl in (['low', 'medium', 'high'] as Intensity[])"
                    :key="lvl"
                    class="px-3 py-2 transition-colors"
                    :class="genIntensity === lvl
                      ? 'bg-ink-900 text-white'
                      : 'bg-white text-ink-600 hover:bg-ink-50'"
                    @click="genIntensity = lvl">
                    {{ intensityLabels[lvl] }}
                  </button>
                </div>
                <button
                  class="flex-1 py-2 rounded-xl bg-ink-900 text-white text-[12px] font-semibold hover:bg-ink-800 active:scale-[0.97] transition-all"
                  @click="generateSnippet">
                  {{ snippet ? '↺ Ещё раз' : '✦ Сгенерировать' }}
                </button>
              </div>
            </div>

            <!-- Snippet textarea -->
            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <label class="text-[12px] font-medium text-ink-600">Блок [Interface]</label>
                <a class="text-[11px] text-ink-400 hover:text-ink-700 underline transition-colors"
                   href="https://vadim-khristenko.github.io/AmneziaWG-Architect/"
                   target="_blank" rel="noopener">
                  Вставить из Architect ↗
                </a>
              </div>
              <textarea
                v-model="snippet"
                rows="8"
                class="w-full px-4 py-3 rounded-2xl bg-ink-100/70 border border-ink-900/8 text-[11px] mono leading-relaxed text-ink-800 placeholder:text-ink-400 focus-ring resize-none transition-colors"
                placeholder="Нажмите «Сгенерировать» или вставьте блок [Interface] из AmneziaWG-Architect" />
            </div>

            <!-- Error -->
            <p v-if="addErr" class="text-[12px] text-danger bg-danger/8 rounded-xl px-3.5 py-2.5">
              {{ addErr }}
            </p>

            <!-- Actions -->
            <div class="flex gap-2 pt-1">
              <button
                class="px-5 py-3 rounded-2xl text-ink-600 text-[13px] font-semibold hover:bg-ink-100 transition-colors"
                @click="closeAdd">
                Отмена
              </button>
              <button
                class="flex-1 py-3 rounded-2xl bg-ink-900 text-white text-[13px] font-semibold hover:bg-ink-800 disabled:opacity-40 active:scale-[0.98] transition-all"
                :disabled="addBusy"
                @click="submitAdd">
                <span v-if="addBusy" class="flex items-center justify-center gap-2">
                  <span class="w-4 h-4 rounded-full border-2 border-white/30 border-t-white animate-spin inline-block" />
                  Создаём…
                </span>
                <span v-else>Получить конфиг →</span>
              </button>
            </div>

          </div>
        </div>
      </div>
    </Teleport>

    <!-- ─── Delete confirm ────────────────────────────────────────────── -->
    <Teleport to="body">
      <div v-if="deleteFor"
           class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4"
           @click.self="deleteFor = null">
        <div class="absolute inset-0 scrim" @click="() => deleteFor = null" />
        <div class="relative w-full sm:max-w-sm bg-surface rounded-t-[28px] sm:rounded-[28px] shadow-pop p-6 space-y-4">
          <h3 class="text-[16px] font-semibold">Удалить устройство?</h3>
          <p class="text-[13px] text-ink-500 leading-relaxed">
            <span class="font-semibold text-ink-800">«{{ deleteFor.name }}»</span> сразу потеряет подключение.
            Восстановить нельзя — нужно будет создать заново.
          </p>
          <div class="flex gap-2 pt-1">
            <button
              class="flex-1 py-3 rounded-2xl bg-ink-100 text-ink-700 text-[13px] font-semibold hover:bg-ink-200 transition-colors"
              @click="() => deleteFor = null">
              Отмена
            </button>
            <button
              class="flex-1 py-3 rounded-2xl bg-danger text-white text-[13px] font-semibold hover:opacity-90 disabled:opacity-50 active:scale-[0.98] transition-all"
              :disabled="deleteBusy"
              @click="confirmDelete">
              <span v-if="deleteBusy" class="flex items-center justify-center gap-2">
                <span class="w-4 h-4 rounded-full border-2 border-white/30 border-t-white animate-spin inline-block" />
                Удаляем…
              </span>
              <span v-else>Удалить</span>
            </button>
          </div>
        </div>
      </div>
    </Teleport>

  </div>
</template>
