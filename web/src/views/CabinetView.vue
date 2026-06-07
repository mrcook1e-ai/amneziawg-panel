<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '@/lib/api'
import type { CabinetView, CabinetDevice, AddDeviceResult } from '@/types'
import { genCfg } from '@/utils/generator'

const route = useRoute()
const token = computed(() => String(route.params.token || ''))

type Phase = 'loading' | 'invalid' | 'ready'
const phase   = ref<Phase>('loading')
const cabinet = ref<CabinetView | null>(null)

// ── Add-device wizard ─────────────────────────────────────────────────
type WizardStep = 'pick' | 'creating' | 'done'
const wizardOpen     = ref(false)
const wizardStep     = ref<WizardStep>('pick')
const wizardErr      = ref('')
const justAdded      = ref<AddDeviceResult | null>(null)

type DeviceTemplate = 'phone' | 'laptop' | 'desktop' | 'other'
const pickedTemplate = ref<DeviceTemplate>('phone')
const customName     = ref('')

const templates: Array<{ key: DeviceTemplate; icon: string; label: string }> = [
  { key: 'phone',   icon: '📱', label: 'Телефон'   },
  { key: 'laptop',  icon: '💻', label: 'Ноутбук'   },
  { key: 'desktop', icon: '🖥',  label: 'Компьютер' },
  { key: 'other',   icon: '🔑', label: 'Другое'    },
]

const defaultName: Record<DeviceTemplate, string> = {
  phone: 'Телефон', laptop: 'Ноутбук', desktop: 'Компьютер', other: 'Устройство',
}

// ── Delete state ───────────────────────────────────────────────────────
const deleteFor  = ref<CabinetDevice | null>(null)
const deleteBusy = ref(false)

// ── Load ────────────────────────────────────────────────────────────────
async function reload() {
  try {
    cabinet.value = await api.cabinetGet(token.value)
    phase.value   = 'ready'
  } catch {
    phase.value = 'invalid'
  }
}
onMounted(reload)

// ── Wizard ──────────────────────────────────────────────────────────────
function openWizard(tpl: DeviceTemplate = 'phone') {
  pickedTemplate.value = tpl
  customName.value     = ''
  wizardErr.value      = ''
  justAdded.value      = null
  wizardStep.value     = 'pick'
  wizardOpen.value     = true
}

function closeWizard() {
  wizardOpen.value = false
  justAdded.value  = null
}

async function createDevice() {
  wizardErr.value  = ''
  const name       = customName.value.trim() || defaultName[pickedTemplate.value]
  wizardStep.value = 'creating'

  const cfg    = genCfg({
    version: '2.0', intensity: 'medium', profile: 'quic_initial',
    customHost: '', mimicAll: false, useTagC: false,
    useTagT: true, useTagR: true, useTagRC: true, useTagRD: true,
    useBrowserFp: false, browserProfile: '', mtu: 1500,
    junkLevel: 5, iterCount: 0, routerMode: false, useExtremeMax: false,
  })
  const snippet = [
    '[Interface]',
    `H1 = ${cfg.h1}`, `H2 = ${cfg.h2}`, `H3 = ${cfg.h3}`, `H4 = ${cfg.h4}`,
    `S1 = ${cfg.s1}`, `S2 = ${cfg.s2}`, `S3 = ${cfg.s3}`, `S4 = ${cfg.s4}`,
    `Jc = ${cfg.jc}`, `Jmin = ${cfg.jmin}`, `Jmax = ${cfg.jmax}`,
    `I1 = ${cfg.i1}`, `I2 = ${cfg.i2}`, `I3 = ${cfg.i3}`, `I4 = ${cfg.i4}`, `I5 = ${cfg.i5}`,
  ].join('\n')

  try {
    justAdded.value  = await api.cabinetAddDevice(token.value, { snippet, deviceName: name })
    wizardStep.value = 'done'
    await reload()
  } catch (e: any) {
    wizardErr.value  = e?.message || 'Ошибка, попробуйте снова'
    wizardStep.value = 'pick'
  }
}

// ── URL helpers ──────────────────────────────────────────────────────────
const amneziaQr  = (id: string) => api.cabinetDeviceAmneziaQrUrl(token.value, id)
const amneziaVpn = (id: string) => api.cabinetDeviceAmneziaVpnUrl(token.value, id)
const confUrl    = (id: string) => api.cabinetDeviceConfUrl(token.value, id)

// ── Copy vpn:// to clipboard ─────────────────────────────────────────────
const copiedId = ref<string | null>(null)
async function copyVpn(devId: string) {
  try {
    const text = await fetch(amneziaVpn(devId)).then(r => r.text())
    await navigator.clipboard.writeText(text)
    copiedId.value = devId
    setTimeout(() => { if (copiedId.value === devId) copiedId.value = null }, 2200)
  } catch { /* ignore */ }
}

const justCopied = ref(false)
async function copyJustAddedVpn() {
  if (!justAdded.value) return
  try {
    const text = await fetch(amneziaVpn(justAdded.value.deviceId)).then(r => r.text())
    await navigator.clipboard.writeText(text)
    justCopied.value = true
    setTimeout(() => { justCopied.value = false }, 2500)
  } catch { /* ignore */ }
}

// ── Delete ────────────────────────────────────────────────────────────────
async function confirmDelete() {
  if (!deleteFor.value) return
  deleteBusy.value = true
  try {
    await api.cabinetDeleteDevice(token.value, deleteFor.value.id)
    deleteFor.value = null
    await reload()
  } finally { deleteBusy.value = false }
}

// ── Status helpers ────────────────────────────────────────────────────────
type DevStatus = 'online' | 'recent' | 'away' | 'never'
function devStatus(d: CabinetDevice): DevStatus {
  if (!d.latestHandshakeAt) return 'never'
  const ms = Date.now() - new Date(d.latestHandshakeAt).getTime()
  if (ms < 3 * 60_000)  return 'online'
  if (ms < 60 * 60_000) return 'recent'
  return 'away'
}

function relTime(s?: string | null) {
  if (!s) return 'никогда'
  try {
    const ms = Date.now() - new Date(s).getTime()
    if (ms < 60_000)    return 'только что'
    if (ms < 3_600_000) return `${Math.floor(ms / 60_000)} мин назад`
    if (ms < 86_400_000) return `${Math.floor(ms / 3_600_000)} ч назад`
    return `${Math.floor(ms / 86_400_000)} д назад`
  } catch { return s }
}

function devIcon(name: string): string {
  const n = name.toLowerCase()
  if (/phone|iphone|android|телефон|самсунг|samsung|pixel/.test(n)) return '📱'
  if (/laptop|ноутбук|macbook|ноут|notebook/.test(n))               return '💻'
  if (/desktop|компьютер|пк|\bpc\b|mac mini/.test(n))               return '🖥'
  if (/tablet|ipad|планшет/.test(n))                                 return '📱'
  return '🔑'
}

const onlineCount = computed(() =>
  (cabinet.value?.devices ?? []).filter(d => devStatus(d) === 'online').length
)
</script>

<template>
  <div class="min-h-screen antialiased" style="background: rgb(var(--surface)); color: rgb(var(--ink-900))">

    <!-- ─── Loading ───────────────────────────────────────────────────── -->
    <div v-if="phase === 'loading'" class="min-h-screen flex items-center justify-center">
      <div class="flex flex-col items-center gap-6">
        <div class="relative w-16 h-16">
          <span class="absolute inset-0 rounded-full border-2 border-ink-200 border-t-ink-600 animate-spin block" />
          <span class="absolute inset-[6px] rounded-full bg-ink-100 flex items-center justify-center text-[22px] block">🛡</span>
        </div>
        <p class="text-[13px] text-ink-400 tracking-wide">Загружаем кабинет…</p>
      </div>
    </div>

    <!-- ─── Invalid ──────────────────────────────────────────────────── -->
    <div v-else-if="phase === 'invalid'" class="min-h-screen flex items-center justify-center p-6">
      <div class="card w-full max-w-sm p-10 text-center space-y-5">
        <div class="w-16 h-16 rounded-full bg-danger/10 flex items-center justify-center mx-auto text-[28px]">🔒</div>
        <div class="space-y-2">
          <h1 class="text-[18px] font-semibold text-ink-900">Кабинет недоступен</h1>
          <p class="text-[13.5px] text-ink-500 leading-relaxed">
            Ссылка не существует или была отозвана.<br>Свяжитесь с администратором.
          </p>
        </div>
      </div>
    </div>

    <!-- ─── Ready ─────────────────────────────────────────────────────── -->
    <template v-else-if="cabinet">
      <div class="max-w-md mx-auto px-4 pt-14 pb-24">

        <!-- Header -->
        <header class="mb-12 animate-rise text-center">
          <div class="inline-flex items-center gap-1.5 text-[11px] uppercase tracking-[0.18em] text-ink-400 font-medium mb-5">
            <span>🛡</span>
            <span>Личный кабинет · AmneziaVPN</span>
          </div>
          <h1 class="stat-hero text-[48px] sm:text-[60px] text-ink-900 mb-4">
            {{ cabinet.name }}
          </h1>
          <div class="flex items-center justify-center gap-3 text-[13.5px] text-ink-400">
            <template v-if="cabinet.devices.length === 0">
              Нет подключённых устройств
            </template>
            <template v-else>
              <span>{{ cabinet.devices.length }} устр.</span>
              <template v-if="onlineCount > 0">
                <span class="text-ink-200">·</span>
                <span class="flex items-center gap-1.5 text-success font-medium">
                  <span class="live-dot" />
                  {{ onlineCount }} онлайн
                </span>
              </template>
            </template>
          </div>
        </header>

        <!-- ── Empty state ── -->
        <div v-if="!cabinet.devices.length"
             class="rounded-4xl border-2 border-dashed border-ink-200 p-10 text-center space-y-7 animate-rise">
          <div class="space-y-3">
            <p class="text-[36px]">🔑</p>
            <p class="text-[17px] font-semibold text-ink-900">Каждое устройство — свой ключ</p>
            <p class="text-[13.5px] text-ink-500 leading-relaxed max-w-[260px] mx-auto">
              Телефон, ноутбук, планшет — у каждого отдельный VPN-ключ. Потеряли устройство — отзываете только его ключ.
            </p>
          </div>

          <!-- Quickstart chips -->
          <div class="grid grid-cols-3 gap-2.5 max-w-[280px] mx-auto">
            <button
              v-for="t in templates.slice(0, 3)" :key="t.key"
              class="flex flex-col items-center gap-2 p-3 rounded-2xl border border-ink-200 hover:border-ink-400 hover:bg-ink-50 transition-all active:scale-[0.96]"
              @click="openWizard(t.key)">
              <span class="text-[24px]">{{ t.icon }}</span>
              <span class="text-[11px] font-semibold text-ink-600">{{ t.label }}</span>
            </button>
          </div>

          <button
            class="w-full max-w-[280px] mx-auto flex items-center justify-center gap-2 h-13 px-6 py-3.5 rounded-2xl bg-ink-900 text-white text-[14px] font-semibold hover:bg-ink-800 active:scale-[0.97] transition-all"
            @click="openWizard()">
            Добавить первое устройство
          </button>
        </div>

        <!-- ── Device list ── -->
        <section v-else class="space-y-3">

          <div
            v-for="(d, i) in cabinet.devices" :key="d.id"
            class="card device-card p-5 animate-rise"
            :class="`delay-${Math.min(i + 1, 6)}`">

            <!-- Device header -->
            <div class="flex items-start gap-4">
              <!-- Icon with status ring -->
              <div class="relative shrink-0">
                <div
                  class="w-12 h-12 rounded-[18px] flex items-center justify-center text-[24px]"
                  :class="{
                    'bg-success/10': devStatus(d) === 'online',
                    'bg-warning/10': devStatus(d) === 'recent',
                    'bg-ink-100':    devStatus(d) === 'away' || devStatus(d) === 'never',
                  }">
                  {{ devIcon(d.name) }}
                </div>
                <span
                  v-if="devStatus(d) === 'online'"
                  class="absolute -bottom-0.5 -right-0.5 w-3.5 h-3.5 rounded-full bg-success border-[2.5px] border-white"
                  style="box-shadow: 0 0 0 3px rgba(52,199,89,0.2)"
                />
                <span
                  v-else-if="devStatus(d) === 'recent'"
                  class="absolute -bottom-0.5 -right-0.5 w-3.5 h-3.5 rounded-full bg-warning border-[2.5px] border-white"
                />
              </div>

              <!-- Name + meta -->
              <div class="flex-1 min-w-0 pt-0.5">
                <div class="flex items-center gap-2 flex-wrap">
                  <span class="text-[15px] font-semibold text-ink-900 truncate">{{ d.name }}</span>
                  <span
                    v-if="!d.enabled"
                    class="text-[10px] uppercase tracking-wide font-semibold text-danger/80 bg-danger/8 px-2 py-0.5 rounded-full shrink-0">
                    выкл
                  </span>
                </div>
                <div class="flex items-center gap-1.5 mt-1.5">
                  <span class="mono text-[11.5px] text-ink-400">{{ d.address }}</span>
                  <span class="text-ink-200 text-[10px]">·</span>
                  <span class="text-[11.5px] text-ink-400">{{ relTime(d.latestHandshakeAt) }}</span>
                </div>
              </div>
            </div>

            <!-- Action row -->
            <div class="flex items-center gap-2 mt-4 pt-4 border-t border-ink-900/5">
              <!-- Amnezia QR — primary -->
              <a
                :href="amneziaQr(d.id)"
                target="_blank"
                rel="noopener"
                class="flex-1 h-10 flex items-center justify-center gap-1.5 rounded-xl bg-ink-900 text-white text-[12.5px] font-semibold hover:bg-ink-800 active:scale-[0.97] transition-all">
                <span class="text-[13px]">▣</span>
                QR-код
              </a>

              <!-- .vpn download -->
              <a
                :href="amneziaVpn(d.id)"
                :download="`${d.name}.vpn`"
                class="flex-1 h-10 flex items-center justify-center gap-1.5 rounded-xl bg-ink-100 text-ink-800 text-[12.5px] font-semibold hover:bg-ink-200 active:scale-[0.97] transition-all">
                <span class="text-[12px]">⬇</span>
                .vpn
              </a>

              <!-- Copy vpn:// -->
              <button
                class="h-10 w-10 flex items-center justify-center rounded-xl text-[15px] transition-all"
                :class="copiedId === d.id
                  ? 'bg-success/12 text-success'
                  : 'bg-ink-100 text-ink-500 hover:bg-ink-200'"
                :title="copiedId === d.id ? 'Скопировано' : 'Скопировать vpn://'"
                @click="copyVpn(d.id)">
                {{ copiedId === d.id ? '✓' : '📋' }}
              </button>

              <!-- Delete -->
              <button
                class="h-10 w-10 flex items-center justify-center rounded-xl text-ink-300 hover:bg-danger/8 hover:text-danger transition-all text-[14px]"
                title="Удалить устройство"
                @click="deleteFor = d">
                ✕
              </button>
            </div>

          </div>

          <!-- Add more device -->
          <button
            class="w-full h-14 flex items-center justify-center gap-2 rounded-3xl border-2 border-dashed border-ink-200 text-ink-400 text-[14px] font-medium hover:border-ink-400 hover:text-ink-700 active:scale-[0.99] transition-all mt-1"
            @click="openWizard()">
            <span class="text-[18px] leading-none">+</span>
            Добавить устройство
          </button>
        </section>

        <p class="text-center text-[11.5px] text-ink-400 mt-10 leading-relaxed">
          Потеряли ссылку на кабинет?<br>
          Попросите администратора выпустить новую.
        </p>

      </div>
    </template>

    <!-- ─── Add-device wizard ─────────────────────────────────────────── -->
    <Teleport to="body">
      <Transition name="sheet">
        <div
          v-if="wizardOpen"
          class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-6">

          <!-- Scrim -->
          <div
            class="absolute inset-0 scrim"
            @click="wizardStep !== 'creating' ? closeWizard() : undefined" />

          <!-- Sheet panel -->
          <div class="sheet-panel relative w-full sm:max-w-md bg-surface rounded-t-[32px] sm:rounded-[32px] shadow-pop overflow-hidden">

            <!-- ── Step: Pick ── -->
            <div v-if="wizardStep === 'pick'" class="p-6 space-y-6">
              <!-- Header -->
              <div class="flex items-start justify-between gap-3">
                <div>
                  <h3 class="text-[19px] font-semibold text-ink-900">Новый VPN-ключ</h3>
                  <p class="text-[12.5px] text-ink-400 mt-0.5">Каждый ключ — только для одного устройства</p>
                </div>
                <button
                  class="w-9 h-9 rounded-full flex items-center justify-center text-ink-400 hover:bg-ink-100 transition-colors text-[16px] shrink-0 mt-0.5"
                  @click="closeWizard">✕</button>
              </div>

              <!-- Device type picker -->
              <div>
                <p class="text-[12px] font-semibold text-ink-600 uppercase tracking-[0.1em] mb-3">Тип устройства</p>
                <div class="grid grid-cols-4 gap-2.5">
                  <button
                    v-for="t in templates" :key="t.key"
                    class="flex flex-col items-center gap-2 py-3 px-2 rounded-2xl border-2 transition-all active:scale-[0.95]"
                    :class="pickedTemplate === t.key
                      ? 'border-ink-900 bg-ink-900/5 shadow-sm'
                      : 'border-ink-100 hover:border-ink-300 bg-ink-50'"
                    @click="pickedTemplate = t.key">
                    <span class="text-[26px]">{{ t.icon }}</span>
                    <span class="text-[11px] font-semibold text-ink-700 leading-tight">{{ t.label }}</span>
                  </button>
                </div>
              </div>

              <!-- Custom name -->
              <div class="space-y-2">
                <label class="text-[12px] font-semibold text-ink-600 uppercase tracking-[0.1em] block">
                  Название <span class="normal-case tracking-normal font-normal text-ink-400">— необязательно</span>
                </label>
                <input
                  v-model="customName"
                  class="w-full h-12 px-4 rounded-2xl bg-ink-50 border border-ink-200 text-[14px] text-ink-900 placeholder:text-ink-400 focus:outline-none focus:border-ink-500 transition-colors"
                  :placeholder="defaultName[pickedTemplate]"
                  @keydown.enter="createDevice" />
              </div>

              <!-- Error -->
              <p v-if="wizardErr" class="text-[12.5px] text-danger bg-danger/8 rounded-xl px-4 py-3">
                {{ wizardErr }}
              </p>

              <!-- CTA -->
              <button
                class="w-full h-14 flex items-center justify-center gap-2 rounded-2xl bg-ink-900 text-white text-[15px] font-semibold hover:bg-ink-800 active:scale-[0.98] transition-all"
                @click="createDevice">
                Получить VPN-ключ →
              </button>

              <p class="text-[11.5px] text-ink-400 text-center leading-relaxed pb-1">
                Уникальная защита AmneziaWG 2.0 генерируется автоматически
              </p>
            </div>

            <!-- ── Step: Creating ── -->
            <div v-else-if="wizardStep === 'creating'"
                 class="p-10 flex flex-col items-center gap-7 min-h-[300px] justify-center">
              <div class="relative w-[72px] h-[72px]">
                <span class="absolute inset-0 rounded-full border-[3px] border-ink-100 border-t-ink-700 animate-spin block" />
                <span class="absolute inset-[10px] rounded-full bg-ink-100 flex items-center justify-center text-[26px] block">🔑</span>
              </div>
              <div class="text-center space-y-1.5">
                <p class="text-[16px] font-semibold text-ink-900">Создаём ключ…</p>
                <p class="text-[13px] text-ink-400">Генерируем уникальную защиту</p>
              </div>
            </div>

            <!-- ── Step: Done ── -->
            <div v-else-if="wizardStep === 'done' && justAdded" class="p-6 space-y-5 animate-fade-in">
              <!-- Header -->
              <div class="flex items-start justify-between gap-3">
                <div class="space-y-1">
                  <div class="flex items-center gap-2">
                    <div class="w-6 h-6 rounded-full bg-success/15 flex items-center justify-center text-[12px]">✓</div>
                    <h3 class="text-[18px] font-semibold text-ink-900">Ключ готов!</h3>
                  </div>
                  <p class="text-[12.5px] text-ink-400 pl-8">
                    {{ justAdded.name }}
                    <span class="mono">· {{ justAdded.address }}</span>
                  </p>
                </div>
                <button
                  class="w-9 h-9 rounded-full flex items-center justify-center text-ink-400 hover:bg-ink-100 transition-colors text-[15px] shrink-0 mt-0.5"
                  @click="closeWizard">✕</button>
              </div>

              <!-- Amnezia QR — HERO -->
              <div class="flex flex-col items-center gap-3 py-2">
                <div class="p-4 bg-white rounded-3xl border border-ink-100 shadow-card inline-block">
                  <img
                    :src="amneziaQr(justAdded.deviceId)"
                    alt="AmneziaVPN QR"
                    class="block w-[220px] h-[220px] sm:w-[248px] sm:h-[248px]" />
                </div>
                <div class="text-center space-y-0.5">
                  <p class="text-[12.5px] font-semibold text-ink-800">Отсканируйте в приложении AmneziaVPN</p>
                  <p class="text-[11px] text-ink-400">Android · iOS · Windows · macOS · Linux</p>
                </div>
              </div>

              <!-- Primary download -->
              <a
                :href="amneziaVpn(justAdded.deviceId)"
                :download="`${justAdded.name}.vpn`"
                class="flex w-full h-13 items-center justify-center gap-2 rounded-2xl bg-ink-900 text-white text-[14.5px] font-semibold hover:bg-ink-800 active:scale-[0.98] transition-all py-3.5">
                <span>⬇</span>
                Скачать .vpn файл
              </a>

              <!-- Secondary actions -->
              <div class="flex gap-2">
                <button
                  class="flex-1 h-11 flex items-center justify-center gap-1.5 rounded-xl text-[12.5px] font-semibold transition-all"
                  :class="justCopied
                    ? 'bg-success/12 text-success'
                    : 'bg-ink-100 text-ink-700 hover:bg-ink-200'"
                  @click="copyJustAddedVpn">
                  {{ justCopied ? '✓ Скопировано' : '📋 Скопировать vpn://' }}
                </button>
                <a
                  :href="confUrl(justAdded.deviceId)"
                  :download="`${justAdded.name}.conf`"
                  class="h-11 px-4 flex items-center justify-center rounded-xl bg-ink-100 text-ink-500 text-[12px] font-medium hover:bg-ink-200 transition-colors whitespace-nowrap">
                  .conf
                </a>
              </div>

              <p class="text-[11.5px] text-ink-400 text-center leading-relaxed">
                Этот ключ только для «{{ justAdded.name }}».<br>
                Каждое устройство получает отдельный ключ.
              </p>

              <button
                class="w-full h-11 flex items-center justify-center gap-1.5 rounded-xl border border-ink-200 text-ink-600 text-[13px] font-medium hover:bg-ink-50 transition-colors"
                @click="openWizard()">
                + Добавить ещё устройство
              </button>
            </div>

          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- ─── Delete confirm ────────────────────────────────────────────── -->
    <Teleport to="body">
      <div
        v-if="deleteFor"
        class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4">
        <div class="absolute inset-0 scrim" @click="deleteFor = null" />
        <div class="relative w-full sm:max-w-sm bg-surface rounded-t-[28px] sm:rounded-[28px] shadow-pop p-6 space-y-4">
          <h3 class="text-[17px] font-semibold text-ink-900">Удалить устройство?</h3>
          <p class="text-[13.5px] text-ink-500 leading-relaxed">
            <span class="font-semibold text-ink-800">{{ deleteFor.name }}</span> сразу потеряет подключение.
            Восстановить нельзя — нужно создать заново.
          </p>
          <div class="flex gap-2 pt-1">
            <button
              class="flex-1 h-12 rounded-2xl bg-ink-100 text-ink-700 text-[13px] font-semibold hover:bg-ink-200 transition-colors"
              @click="deleteFor = null">
              Отмена
            </button>
            <button
              class="flex-1 h-12 rounded-2xl bg-danger text-white text-[13px] font-semibold hover:opacity-90 disabled:opacity-50 active:scale-[0.98] transition-all"
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
