<script setup lang="ts">
/*
  Личный кабинет клиента. Публичный — auth по токену в URL.
  Клиент видит свои устройства, может добавить новое (вставив snippet из
  Architect), скачать .conf/QR любого устройства, удалить устройство.
*/

import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '@/lib/api'
import type { CabinetView, CabinetDevice, AddDeviceResult } from '@/types'

const route = useRoute()
const token = computed(() => String(route.params.token || ''))

type Phase = 'loading' | 'invalid' | 'ready'
const phase = ref<Phase>('loading')
const cabinet = ref<CabinetView | null>(null)
const err = ref('')

const addOpen = ref(false)
const addBusy = ref(false)
const deviceName = ref('')
const snippet = ref('')
const addErr = ref('')
const justAdded = ref<AddDeviceResult | null>(null)

const deleteFor = ref<CabinetDevice | null>(null)
const deleteBusy = ref(false)

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
  snippet.value = ''
  addErr.value = ''
  justAdded.value = null
  addOpen.value = true
}

async function submitAdd() {
  addErr.value = ''
  if (!snippet.value.trim()) { addErr.value = 'Вставьте блок [Interface] из генератора'; return }
  addBusy.value = true
  try {
    justAdded.value = await api.cabinetAddDevice(token.value, {
      snippet: snippet.value.trim(),
      deviceName: deviceName.value.trim() || 'устройство',
    })
    await reload()
  } catch (e: any) {
    addErr.value = e?.message || 'Ошибка'
  } finally {
    addBusy.value = false
  }
}

function downloadJustAdded() {
  if (!justAdded.value) return
  const blob = new Blob([justAdded.value.conf], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${justAdded.value.name.replace(/[^a-zA-Z0-9-_]+/g, '-')}.conf`
  document.body.appendChild(a); a.click(); a.remove()
  URL.revokeObjectURL(url)
}

async function copyJustAdded() {
  if (!justAdded.value) return
  try { await navigator.clipboard.writeText(justAdded.value.conf) } catch { /* ignore */ }
}

function closeAdd() {
  addOpen.value = false
  // justAdded остаётся в локальном состоянии — пользователь видит запись в списке.
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

function fmtDate(s?: string | null) {
  if (!s) return '—'
  try { return new Date(s).toLocaleString() } catch { return s }
}
function relTime(s?: string | null) {
  if (!s) return 'никогда'
  try {
    const diff = Date.now() - new Date(s).getTime()
    if (diff < 60_000) return 'только что'
    if (diff < 3600_000) return Math.floor(diff / 60_000) + ' мин назад'
    if (diff < 86400_000) return Math.floor(diff / 3600_000) + ' ч назад'
    return Math.floor(diff / 86400_000) + ' д назад'
  } catch { return s }
}
</script>

<template>
  <div class="min-h-screen bg-ink-50 text-ink-900 flex items-start justify-center py-8 px-4">
    <div class="w-full max-w-2xl">

      <!-- Loading -->
      <div v-if="phase === 'loading'" class="rounded-2xl bg-white shadow-card border border-ink-900/5 p-8 text-center text-ink-500">
        Загружаем кабинет…
      </div>

      <!-- Invalid -->
      <div v-else-if="phase === 'invalid'" class="rounded-2xl bg-white shadow-card border border-ink-900/5 p-8 space-y-3">
        <div class="text-[15px] font-semibold text-danger">Кабинет недоступен</div>
        <p class="text-[13px] text-ink-500 leading-relaxed">
          Ссылка не существует или была отозвана. Свяжитесь с админом — он выпустит новую.
        </p>
      </div>

      <!-- Ready -->
      <template v-else-if="cabinet">
        <header class="text-center mb-6">
          <div class="eyebrow">AmneziaWG · Личный кабинет</div>
          <h1 class="num-display text-[32px] sm:text-[40px] mt-1">{{ cabinet.name }}</h1>
        </header>

        <!-- Just-added device (one-shot view of conf/QR for the newest device) -->
        <div v-if="justAdded" class="rounded-2xl bg-white shadow-card border border-success/30 p-5 sm:p-6 mb-6 space-y-4">
          <div class="text-success text-[13px] font-medium">
            ✓ Устройство «{{ justAdded.name }}» создано
          </div>
          <p class="text-[12px] text-ink-500 leading-relaxed">
            Сохраните конфиг — этот блок исчезнет после перезагрузки страницы.
            Позже сможете перескачать через кнопку «.conf» в списке устройств.
          </p>
          <div class="grid sm:grid-cols-[auto_1fr] gap-5 items-start">
            <div class="bg-white border border-ink-900/10 rounded-xl p-3 mx-auto sm:mx-0">
              <img :src="`data:image/png;base64,${justAdded.qrPng64}`" alt="QR" class="block w-[200px] h-[200px]" />
            </div>
            <div class="space-y-2 min-w-0">
              <button class="w-full py-2.5 rounded-lg bg-ink-900 text-white text-[13px] font-medium hover:bg-ink-800" @click="downloadJustAdded">⬇ Скачать .conf</button>
              <button class="w-full py-2.5 rounded-lg bg-ink-100 text-ink-900 text-[13px] font-medium hover:bg-ink-200" @click="copyJustAdded">📋 Скопировать конфиг</button>
            </div>
          </div>
        </div>

        <!-- Devices list -->
        <section class="space-y-3">
          <div class="flex items-center justify-between gap-3">
            <h2 class="eyebrow">Ваши устройства · {{ cabinet.devices.length }}</h2>
            <button class="text-[12px] px-3 py-1.5 rounded-lg bg-ink-900 text-white hover:bg-ink-800 font-medium" @click="openAdd">
              + Добавить устройство
            </button>
          </div>

          <div v-if="!cabinet.devices.length" class="rounded-2xl bg-white shadow-card border border-ink-900/5 p-6 text-center text-[13px] text-ink-500">
            Устройств пока нет. Нажмите «Добавить устройство», чтобы получить первый .conf.
          </div>

          <div v-else class="rounded-2xl bg-white shadow-card border border-ink-900/5 divide-y divide-ink-900/5">
            <div v-for="d in cabinet.devices" :key="d.id" class="p-4 flex items-center gap-3 flex-wrap">
              <div class="flex-1 min-w-0">
                <div class="flex items-baseline gap-2 flex-wrap">
                  <span class="text-[14px] font-medium text-ink-900">{{ d.name }}</span>
                  <span v-if="!d.enabled" class="text-[10px] uppercase tracking-[0.12em] text-danger px-1.5 py-0.5 rounded bg-danger/10">отключено</span>
                </div>
                <div class="text-[11px] text-ink-500 mt-0.5">
                  <span class="mono">{{ d.address }}</span> · последнее подключение: {{ relTime(d.latestHandshakeAt) }}
                </div>
              </div>
              <div class="flex items-center gap-1.5">
                <a :href="deviceConfUrl(d.id)" :download="`${d.name.replace(/[^a-zA-Z0-9-_]+/g, '-')}.conf`"
                   class="text-[12px] px-2.5 py-1.5 rounded-lg bg-ink-100 text-ink-900 hover:bg-ink-200 font-medium">.conf</a>
                <a :href="deviceQrUrl(d.id)" target="_blank" rel="noopener"
                   class="text-[12px] px-2.5 py-1.5 rounded-lg bg-ink-100 text-ink-900 hover:bg-ink-200 font-medium">QR</a>
                <button class="text-[12px] px-2.5 py-1.5 rounded-lg text-danger hover:bg-danger/10 font-medium" @click="deleteFor = d">
                  Удалить
                </button>
              </div>
            </div>
          </div>
        </section>

        <p class="text-[11px] text-ink-400 text-center mt-8">
          Если потеряете ссылку — попросите админа выпустить новую.
        </p>
      </template>

      <!-- Add-device modal (lightweight inline, no shared Modal dep to keep public bundle small) -->
      <div v-if="addOpen" class="fixed inset-0 bg-black/40 z-50 flex items-start sm:items-center justify-center p-4 overflow-y-auto" @click.self="closeAdd">
        <div class="bg-white rounded-2xl shadow-2xl border border-ink-900/10 w-full max-w-xl my-auto">
          <div class="p-5 sm:p-6 space-y-4">
            <div class="flex items-center justify-between">
              <h3 class="text-[16px] font-semibold">Добавить устройство</h3>
              <button class="text-ink-500 hover:text-ink-900 text-[18px]" @click="closeAdd">×</button>
            </div>
            <p class="text-[12px] text-ink-500 leading-relaxed">
              Сгенерируйте параметры обфускации в
              <a class="underline" target="_blank" rel="noopener" href="https://vadim-khristenko.github.io/AmneziaWG-Architect/">AmneziaWG-Architect</a>
              (AWG 2.0), скопируйте блок <code class="mono">[Interface]</code> и вставьте сюда.
              На каждом устройстве свой snippet — обфускация не должна совпадать между ними.
            </p>

            <div class="space-y-1.5">
              <label class="text-[12px] text-ink-700 font-medium">Имя устройства</label>
              <input v-model="deviceName" class="w-full p-2.5 rounded-lg bg-ink-100/60 border border-ink-900/10 text-[13px] focus-ring"
                     placeholder="iPhone / MacBook / ноут жены" />
            </div>

            <div class="space-y-1.5">
              <label class="text-[12px] text-ink-700 font-medium">Блок [Interface]</label>
              <textarea v-model="snippet" rows="12"
                        class="w-full p-3 rounded-lg bg-ink-100/60 border border-ink-900/10 text-[11.5px] mono leading-snug focus-ring"
                        :placeholder="`[Interface]\nJc = 4\nJmin = 362\nJmax = 943\nS1 = 43\nS2 = 65\nS3 = 35\nS4 = 28\nH1 = 320858491-320865164\nH2 = 1445464973-1445512660\nH3 = 3235131120-3235164350\nH4 = 3875042355-3875063814\nI1 = <b 0x...>`" />
            </div>

            <p v-if="addErr" class="text-[12px] text-danger">{{ addErr }}</p>

            <div class="flex justify-end gap-2 pt-2">
              <button class="text-[13px] px-4 py-2 rounded-lg text-ink-700 hover:bg-ink-100" @click="closeAdd">Отмена</button>
              <button class="text-[13px] px-4 py-2 rounded-lg bg-ink-900 text-white hover:bg-ink-800 disabled:opacity-50 font-medium"
                      :disabled="addBusy" @click="submitAdd">
                {{ addBusy ? 'Создаём…' : 'Получить конфиг' }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Delete confirm -->
      <div v-if="deleteFor" class="fixed inset-0 bg-black/40 z-50 flex items-center justify-center p-4" @click.self="deleteFor = null">
        <div class="bg-white rounded-2xl shadow-2xl border border-ink-900/10 w-full max-w-sm p-5 space-y-4">
          <h3 class="text-[15px] font-semibold">Удалить устройство?</h3>
          <p class="text-[12.5px] text-ink-500 leading-relaxed">
            «{{ deleteFor.name }}» сразу потеряет подключение. Восстановить нельзя — нужно будет создать заново.
          </p>
          <div class="flex justify-end gap-2">
            <button class="text-[13px] px-4 py-2 rounded-lg text-ink-700 hover:bg-ink-100" @click="deleteFor = null">Отмена</button>
            <button class="text-[13px] px-4 py-2 rounded-lg bg-danger text-white hover:opacity-90 disabled:opacity-50 font-medium"
                    :disabled="deleteBusy" @click="confirmDelete">
              {{ deleteBusy ? 'Удаляем…' : 'Удалить' }}
            </button>
          </div>
        </div>
      </div>

    </div>
  </div>
</template>
