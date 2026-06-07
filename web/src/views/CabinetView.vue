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

function relTime(s?: string | null): string {
  if (!s) return 'никогда'
  try {
    const ms = Date.now() - new Date(s).getTime()
    if (ms < 60_000)     return 'только что'
    if (ms < 3_600_000)  return `${Math.floor(ms / 60_000)} мин назад`
    if (ms < 86_400_000) return `${Math.floor(ms / 3_600_000)} ч назад`
    return `${Math.floor(ms / 86_400_000)} д назад`
  } catch { return s }
}

function fmtDate(s: string): string {
  try {
    return new Date(s).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' })
  } catch { return '' }
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
  <div class="min-h-screen antialiased bg-ink-50 text-ink-900">

    <!-- Ambient background glow — dark only -->
    <div
      class="pointer-events-none fixed inset-0 opacity-0 dark:opacity-100"
      style="background: radial-gradient(ellipse 80% 50% at 50% -10%, rgba(232,160,65,0.07) 0%, transparent 70%)"
      aria-hidden="true"
    />

    <!-- ─── Loading ───────────────────────────────────────────────────── -->
    <div v-if="phase === 'loading'" class="min-h-screen flex items-center justify-center">
      <div class="flex flex-col items-center gap-6">
        <div class="relative w-14 h-14">
          <span class="absolute inset-0 rounded-full border-2 border-ink-200 border-t-ink-600 animate-spin block" />
          <span class="absolute inset-[6px] rounded-full bg-ink-100 flex items-center justify-center text-[20px]">🛡</span>
        </div>
        <p class="text-[13px] text-ink-500 tracking-wide">Загружаем кабинет…</p>
      </div>
    </div>

    <!-- ─── Invalid ──────────────────────────────────────────────────── -->
    <div v-else-if="phase === 'invalid'" class="min-h-screen flex items-center justify-center p-6">
      <div class="card w-full max-w-sm p-10 text-center space-y-5">
        <div class="w-16 h-16 rounded-full bg-danger/10 flex items-center justify-center mx-auto text-[28px]">🔒</div>
        <div class="space-y-2">
          <h1 class="text-[18px] font-semibold">Кабинет недоступен</h1>
          <p class="text-[13.5px] text-ink-500 leading-relaxed">
            Ссылка не существует или была отозвана.<br>Свяжитесь с администратором.
          </p>
        </div>
      </div>
    </div>

    <!-- ─── Ready ─────────────────────────────────────────────────────── -->
    <template v-else-if="cabinet">
      <div class="relative max-w-md mx-auto px-4 pt-14 pb-24">

        <!-- Header -->
        <header class="mb-10 animate-rise text-center">
          <div class="inline-flex items-center gap-1.5 text-[10.5px] uppercase tracking-[0.18em] text-ink-500 font-medium mb-5">
            <span>🛡</span>
            <span>Личный кабинет · AmneziaVPN</span>
          </div>
          <h1 class="stat-hero text-[52px] sm:text-[64px] text-ink-900 mb-4 leading-none">
            {{ cabinet.name }}
          </h1>

          <!-- Status summary chips -->
          <div class="flex items-center justify-center gap-2 flex-wrap">
            <span class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-ink-100 text-[12px] text-ink-600 font-medium">
              {{ cabinet.devices.length }}
              {{ cabinet.devices.length === 1 ? 'устройство' : cabinet.devices.length < 5 ? 'устройства' : 'устройств' }}
            </span>
            <span
              v-if="onlineCount > 0"
              class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-success/10 text-[12px] text-success font-semibold">
              <span class="live-dot scale-75" />
              {{ onlineCount }} онлайн
            </span>
            <span
              v-else-if="cabinet.devices.length > 0"
              class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-ink-100 text-[12px] text-ink-500">
              нет подключений
            </span>
          </div>
        </header>

        <!-- ── Empty state ── -->
        <div
          v-if="!cabinet.devices.length"
          class="rounded-3xl border-2 border-dashed border-ink-200 p-10 text-center space-y-7 animate-rise">
          <div class="space-y-3">
            <p class="text-[40px]">🔑</p>
            <p class="text-[17px] font-semibold">Каждое устройство — свой ключ</p>
            <p class="text-[13px] text-ink-500 leading-relaxed max-w-[260px] mx-auto">
              Телефон, ноутбук, планшет — у каждого отдельный VPN-ключ.
              Потеряли устройство — отзываете только его ключ.
            </p>
          </div>
          <div class="grid grid-cols-3 gap-2.5 max-w-[280px] mx-auto">
            <button
              v-for="t in templates.slice(0, 3)" :key="t.key"
              class="flex flex-col items-center gap-2 p-3 rounded-2xl border border-ink-200 hover:border-ink-400 hover:bg-ink-100/50 transition-all active:scale-[0.96]"
              @click="openWizard(t.key)">
              <span class="text-[26px]">{{ t.icon }}</span>
              <span class="text-[11px] font-semibold text-ink-600">{{ t.label }}</span>
            </button>
          </div>
          <button
            class="btn-primary w-full max-w-[280px] mx-auto flex items-center justify-center gap-2"
            @click="openWizard()">
            Добавить первое устройство
          </button>
        </div>

        <!-- ── Device list ── -->
        <section v-else class="space-y-3">

          <div
            v-for="(d, i) in cabinet.devices"
            :key="d.id"
            class="card device-card p-5 animate-rise"
            :class="`delay-${Math.min(i + 1, 6)}`">

            <!-- Top row: icon + name + status -->
            <div class="flex items-start gap-3.5">
              <!-- Device icon with status ring -->
              <div class="relative shrink-0">
                <div
                  class="w-11 h-11 rounded-[16px] flex items-center justify-center text-[22px]"
                  :class="{
                    'bg-success/12 dark:bg-success/15': devStatus(d) === 'online',
                    'bg-warning/10':                    devStatus(d) === 'recent',
                    'bg-ink-100':                       devStatus(d) === 'away' || devStatus(d) === 'never',
                  }">
                  {{ devIcon(d.name) }}
                </div>
                <!-- Status dot badge -->
                <span
                  v-if="devStatus(d) === 'online'"
                  class="absolute -bottom-0.5 -right-0.5 w-3.5 h-3.5 rounded-full bg-success border-2 border-surface"
                  style="box-shadow: 0 0 0 3px rgba(52,199,89,0.2)"
                />
                <span
                  v-else-if="devStatus(d) === 'recent'"
                  class="absolute -bottom-0.5 -right-0.5 w-3.5 h-3.5 rounded-full bg-warning border-2 border-surface"
                />
              </div>

              <!-- Name + meta -->
              <div class="flex-1 min-w-0 pt-0.5">
                <div class="flex items-center gap-2 flex-wrap mb-1">
                  <span class="text-[15px] font-semibold truncate">{{ d.name }}</span>
                  <span
                    v-if="!d.enabled"
                    class="text-[10px] uppercase tracking-wide font-semibold text-danger/80 bg-danger/10 px-2 py-0.5 rounded-full shrink-0">
                    выкл
                  </span>
                </div>
                <!-- IP + last seen -->
                <div class="flex items-center gap-1.5 flex-wrap">
                  <span class="mono text-[11.5px] text-ink-500 bg-ink-100 px-2 py-0.5 rounded-lg">{{ d.address }}</span>
                  <span class="text-ink-300 text-[10px]">·</span>
                  <span
                    class="text-[11.5px]"
                    :class="{
                      'text-success font-medium':  devStatus(d) === 'online',
                      'text-warning font-medium':  devStatus(d) === 'recent',
                      'text-ink-500':              devStatus(d) === 'away' || devStatus(d) === 'never',
                    }">
                    <template v-if="devStatus(d) === 'online'">● онлайн</template>
                    <template v-else>{{ relTime(d.latestHandshakeAt) }}</template>
                  </span>
                </div>
              </div>
            </div>

            <!-- Info strip -->
            <div class="mt-3.5 flex items-center gap-3 text-[11px] text-ink-500 border-t border-ink-900/6 pt-3">
              <span>Добавлено {{ fmtDate(d.createdAt) }}</span>
              <span class="text-ink-200">·</span>
              <span class="mono">AmneziaWG 2.0</span>
              <span class="ml-auto inline-flex items-center gap-1 text-ink-400">
                <svg class="w-3 h-3" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/>
                </svg>
                зашифровано
              </span>
            </div>

            <!-- Action row -->
            <div class="flex items-center gap-2 mt-3">
              <!-- QR — primary, explicit amber accent -->
              <a
                :href="amneziaQr(d.id)"
                target="_blank"
                rel="noopener"
                class="btn-primary flex-1 flex items-center justify-center gap-1.5 h-10 text-[12.5px]">
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                  <rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/>
                  <rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="3" height="3"/>
                  <rect x="18" y="14" width="3" height="3"/><rect x="14" y="18" width="3" height="3"/>
                  <rect x="18" y="18" width="3" height="3"/>
                </svg>
                QR-код
              </a>

              <!-- .vpn download -->
              <a
                :href="amneziaVpn(d.id)"
                :download="`${d.name}.vpn`"
                class="btn-secondary flex-1 flex items-center justify-center gap-1.5 h-10 text-[12.5px]">
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"/>
                </svg>
                .vpn
              </a>

              <!-- Copy vpn:// -->
              <button
                class="h-10 w-10 flex items-center justify-center rounded-xl transition-all text-[13px]"
                :class="copiedId === d.id
                  ? 'bg-success/15 text-success'
                  : 'btn-ghost'"
                :title="copiedId === d.id ? 'Скопировано' : 'Скопировать vpn://'"
                @click="copyVpn(d.id)">
                <svg v-if="copiedId === d.id" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                </svg>
                <svg v-else class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
                  <rect x="9" y="9" width="13" height="13" rx="2"/><path stroke-linecap="round" d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>
                </svg>
              </button>

              <!-- Delete -->
              <button
                class="h-10 w-10 flex items-center justify-center rounded-xl text-ink-400 hover:bg-danger/10 hover:text-danger transition-all"
                title="Удалить устройство"
                @click="deleteFor = d">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
                </svg>
              </button>
            </div>

          </div>

          <!-- Add more -->
          <button
            class="w-full h-14 flex items-center justify-center gap-2 rounded-3xl border-2 border-dashed border-ink-200 text-ink-500 text-[14px] font-medium hover:border-ink-400 hover:text-ink-700 dark:hover:text-ink-300 active:scale-[0.99] transition-all mt-1"
            @click="openWizard()">
            <span class="text-[18px] leading-none">+</span>
            Добавить устройство
          </button>
        </section>

        <p class="text-center text-[11.5px] text-ink-500 mt-10 leading-relaxed">
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
          <div class="absolute inset-0 scrim" @click="wizardStep !== 'creating' ? closeWizard() : undefined" />

          <div class="sheet-panel relative w-full sm:max-w-md bg-surface-raised rounded-t-[32px] sm:rounded-[32px] shadow-pop overflow-hidden">

            <!-- Step: Pick -->
            <div v-if="wizardStep === 'pick'" class="p-6 space-y-6">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <h3 class="text-[19px] font-semibold">Новый VPN-ключ</h3>
                  <p class="text-[12.5px] text-ink-500 mt-0.5">Каждый ключ — только для одного устройства</p>
                </div>
                <button class="w-9 h-9 rounded-full flex items-center justify-center text-ink-400 hover:bg-ink-100 transition-colors shrink-0 mt-0.5" @click="closeWizard">
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/></svg>
                </button>
              </div>

              <!-- Type chips -->
              <div>
                <p class="text-[11px] font-semibold text-ink-500 uppercase tracking-[0.12em] mb-3">Тип устройства</p>
                <div class="grid grid-cols-4 gap-2">
                  <button
                    v-for="t in templates" :key="t.key"
                    class="flex flex-col items-center gap-2 py-3 px-1 rounded-2xl border-2 transition-all active:scale-[0.95]"
                    :class="pickedTemplate === t.key
                      ? 'border-amber-400 bg-amber-50 dark:bg-amber-400/10'
                      : 'border-ink-200 hover:border-ink-300 bg-ink-50'"
                    @click="pickedTemplate = t.key">
                    <span class="text-[26px]">{{ t.icon }}</span>
                    <span class="text-[10.5px] font-semibold text-ink-700 dark:text-ink-600 leading-tight">{{ t.label }}</span>
                  </button>
                </div>
              </div>

              <!-- Custom name -->
              <div class="space-y-2">
                <label class="text-[11px] font-semibold text-ink-500 uppercase tracking-[0.12em] block">
                  Название <span class="normal-case tracking-normal font-normal text-ink-400">— необязательно</span>
                </label>
                <input
                  v-model="customName"
                  class="w-full h-12 px-4 rounded-2xl bg-ink-100 border border-ink-200 text-[14px] placeholder:text-ink-400 focus:outline-none focus:border-amber-400 transition-colors"
                  :placeholder="defaultName[pickedTemplate]"
                  @keydown.enter="createDevice" />
              </div>

              <p v-if="wizardErr" class="text-[12.5px] text-danger bg-danger/10 rounded-xl px-4 py-3">{{ wizardErr }}</p>

              <button class="btn-primary w-full h-14 flex items-center justify-center gap-2 text-[15px]" @click="createDevice">
                Получить VPN-ключ →
              </button>

              <p class="text-[11.5px] text-ink-500 text-center pb-1">
                Уникальная защита AmneziaWG 2.0 создаётся автоматически
              </p>
            </div>

            <!-- Step: Creating -->
            <div v-else-if="wizardStep === 'creating'" class="p-10 flex flex-col items-center gap-7 min-h-[300px] justify-center">
              <div class="relative w-[72px] h-[72px]">
                <span class="absolute inset-0 rounded-full border-[3px] border-ink-200 border-t-amber-400 animate-spin block" />
                <span class="absolute inset-[10px] rounded-full bg-ink-100 flex items-center justify-center text-[26px]">🔑</span>
              </div>
              <div class="text-center space-y-1.5">
                <p class="text-[16px] font-semibold">Создаём ключ…</p>
                <p class="text-[13px] text-ink-500">Генерируем уникальную защиту</p>
              </div>
            </div>

            <!-- Step: Done -->
            <div v-else-if="wizardStep === 'done' && justAdded" class="p-6 space-y-5 animate-fade-in">
              <div class="flex items-start justify-between gap-3">
                <div class="space-y-1">
                  <div class="flex items-center gap-2">
                    <div class="w-6 h-6 rounded-full bg-success/15 flex items-center justify-center text-success">
                      <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/></svg>
                    </div>
                    <h3 class="text-[18px] font-semibold">Ключ готов!</h3>
                  </div>
                  <p class="text-[12.5px] text-ink-500 pl-8">
                    {{ justAdded.name }} <span class="mono">· {{ justAdded.address }}</span>
                  </p>
                </div>
                <button class="w-9 h-9 rounded-full flex items-center justify-center text-ink-400 hover:bg-ink-100 transition-colors shrink-0" @click="closeWizard">
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/></svg>
                </button>
              </div>

              <!-- Amnezia QR hero -->
              <div class="flex flex-col items-center gap-3 py-1">
                <div class="p-4 bg-white rounded-3xl border border-ink-100 shadow-sm inline-block">
                  <img :src="amneziaQr(justAdded.deviceId)" alt="AmneziaVPN QR" class="block w-[220px] h-[220px] sm:w-[252px] sm:h-[252px]" />
                </div>
                <div class="text-center">
                  <p class="text-[12.5px] font-semibold text-ink-800 dark:text-ink-700">Отсканируйте в приложении AmneziaVPN</p>
                  <p class="text-[11px] text-ink-500 mt-0.5">Android · iOS · Windows · macOS · Linux</p>
                </div>
              </div>

              <a
                :href="amneziaVpn(justAdded.deviceId)"
                :download="`${justAdded.name}.vpn`"
                class="btn-primary flex w-full h-13 items-center justify-center gap-2 text-[14.5px] py-3.5">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"/></svg>
                Скачать .vpn файл
              </a>

              <div class="flex gap-2">
                <button
                  class="flex-1 h-11 flex items-center justify-center gap-1.5 rounded-xl text-[12.5px] font-semibold transition-all"
                  :class="justCopied ? 'bg-success/15 text-success' : 'btn-secondary'"
                  @click="copyJustAddedVpn">
                  <svg v-if="justCopied" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2.5" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/></svg>
                  <svg v-else class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24"><rect x="9" y="9" width="13" height="13" rx="2"/><path stroke-linecap="round" d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/></svg>
                  {{ justCopied ? 'Скопировано' : 'Скопировать vpn://' }}
                </button>
                <a
                  :href="confUrl(justAdded.deviceId)"
                  :download="`${justAdded.name}.conf`"
                  class="h-11 px-4 flex items-center justify-center rounded-xl btn-ghost text-[12px] font-medium whitespace-nowrap">
                  .conf
                </a>
              </div>

              <p class="text-[11.5px] text-ink-500 text-center leading-relaxed">
                Этот ключ только для «{{ justAdded.name }}».<br>
                Каждое устройство — свой ключ.
              </p>

              <button class="w-full h-11 flex items-center justify-center gap-1.5 rounded-xl border border-ink-200 text-ink-600 dark:text-ink-500 text-[13px] font-medium hover:bg-ink-100/50 transition-colors" @click="openWizard()">
                + Добавить ещё устройство
              </button>
            </div>

          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- ─── Delete confirm ────────────────────────────────────────────── -->
    <Teleport to="body">
      <div v-if="deleteFor" class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4">
        <div class="absolute inset-0 scrim" @click="deleteFor = null" />
        <div class="relative w-full sm:max-w-sm bg-surface-raised rounded-t-[28px] sm:rounded-[28px] shadow-pop p-6 space-y-4">
          <h3 class="text-[17px] font-semibold">Удалить устройство?</h3>
          <p class="text-[13.5px] text-ink-500 leading-relaxed">
            <span class="font-semibold text-ink-800 dark:text-ink-700">{{ deleteFor.name }}</span> сразу потеряет подключение.
            Восстановить нельзя — нужно создать заново.
          </p>
          <div class="flex gap-2 pt-1">
            <button class="btn-secondary flex-1 h-12 rounded-2xl text-[13px] font-semibold" @click="deleteFor = null">
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
