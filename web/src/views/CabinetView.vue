<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useTitle } from '@/composables/useTitle'
import { useRoute } from 'vue-router'
import {
  Shield, Lock, Key, Smartphone, Laptop, Monitor,
  QrCode, Download, Copy, Check, Trash2, X,
  Sun, Moon, Plus, RefreshCw, ChevronLeft, ChevronRight, Loader2,
  Compass,
} from 'lucide-vue-next'

import { api } from '@/lib/api'
import { useThemeStore } from '@/stores/theme'
import { useToastStore } from '@/stores/toasts'
import type { CabinetView, CabinetDevice, AddDeviceResult } from '@/types'
import { genCfg } from '@/utils/generator'
import Button from '@/components/atoms/Button.vue'
import Badge from '@/components/atoms/Badge.vue'
import QrCarousel from '@/components/molecules/QrCarousel.vue'
import SplitTunnelPicker from '@/components/molecules/SplitTunnelPicker.vue'

const route = useRoute()
const token  = computed(() => String(route.params.token || ''))
const theme  = useThemeStore()
const toasts = useToastStore()

type Phase = 'loading' | 'invalid' | 'ready'
const phase   = ref<Phase>('loading')
const cabinet = ref<CabinetView | null>(null)

// ── Browser title ──────────────────────────────────────────────────────
useTitle(() => cabinet.value
  ? `${cabinet.value.name} · Личный кабинет`
  : 'Личный кабинет · AmneziaVPN')

// ── Theme toggle — only light / dark, auto is the system default ───────
const isDark = computed(() => {
  if (theme.mode === 'auto') return theme.resolved === 'dark'
  return theme.mode === 'dark'
})
function toggleTheme() {
  theme.set(isDark.value ? 'light' : 'dark')
}

// ── Add-device wizard ──────────────────────────────────────────────────
type WizardStep = 'pick' | 'creating' | 'done'
const wizardOpen     = ref(false)
const wizardStep     = ref<WizardStep>('pick')
const wizardErr      = ref('')
const justAdded      = ref<AddDeviceResult | null>(null)

type DeviceTemplate = 'phone' | 'laptop' | 'desktop' | 'other'
const pickedTemplate = ref<DeviceTemplate>('phone')
const customName     = ref('')

// ── Advanced obfuscation params (collapsed by default) ──────────────────
// genCfg() takes ~15 toggles; exposing them all to end-users is overwhelming
// and most are admin-territory (browser fingerprint, junk iteration count).
// We surface the four that meaningfully change behavior in restrictive
// networks: obfuscation intensity, mimicry profile, MTU, and an "extreme"
// fallback for networks that block "medium".
import type { Intensity, MimicProfile } from '@/utils/generator'

const advIntensity   = ref<Intensity>('medium')
const advProfile     = ref<MimicProfile>('quic_initial')
const advMtu         = ref<number>(1500)
const advExtreme     = ref<boolean>(false)

const INTENSITIES: { v: Intensity; label: string; hint: string }[] = [
  { v: 'low',    label: 'Лёгкая',  hint: 'быстрее, меньше нагрузки' },
  { v: 'medium', label: 'Средняя', hint: 'баланс — рекомендуется'   },
  { v: 'high',   label: 'Сильная', hint: 'устойчивее к блокировкам' },
]
// Curated subset of generator MimicProfile values that the user can
// reasonably reason about. Esoteric ones (dtls, sip, dns_query) stay
// admin-only — exposing them adds confusion without changing outcomes
// for the common "is anything blocked" use case.
const PROFILES: { v: MimicProfile; label: string }[] = [
  { v: 'quic_initial',     label: 'QUIC' },
  { v: 'tls_client_hello', label: 'TLS · HTTPS' },
  { v: 'http3',            label: 'HTTP/3' },
  { v: 'random',           label: 'Случайно' },
]

interface Template { key: DeviceTemplate; icon: any; label: string }
const templates: Template[] = [
  { key: 'phone',   icon: Smartphone, label: 'Телефон'   },
  { key: 'laptop',  icon: Laptop,     label: 'Ноутбук'   },
  { key: 'desktop', icon: Monitor,    label: 'Компьютер' },
  { key: 'other',   icon: Key,        label: 'Другое'    },
]

const defaultName: Record<DeviceTemplate, string> = {
  phone: 'Телефон', laptop: 'Ноутбук', desktop: 'Компьютер', other: 'Устройство',
}

// ── QR fullscreen carousel ────────────────────────────────────────────
const qrOpenFor   = ref<string | null>(null)
const qrChunks    = ref<string[]>([])          // base64-encoded PNG per chunk
const qrIdx       = ref(0)
const qrLoading   = ref(false)
const qrError     = ref(false)
let   qrTimer: ReturnType<typeof setInterval> | null = null

function stopQrTimer() {
  if (qrTimer) { clearInterval(qrTimer); qrTimer = null }
}

function startQrTimer() {
  stopQrTimer()
  if (qrChunks.value.length <= 1) return
  // Each chunk shown for 2.5 s so the Amnezia scanner has time to capture it
  qrTimer = setInterval(() => {
    qrIdx.value = (qrIdx.value + 1) % qrChunks.value.length
  }, 2500)
}

async function openQr(id: string) {
  qrOpenFor.value = id
  qrChunks.value  = []
  qrIdx.value     = 0
  qrLoading.value = true
  qrError.value   = false
  try {
    const res = await api.cabinetDeviceAmneziaQrChunks(token.value, id)
    qrChunks.value = res.chunks
    startQrTimer()
  } catch {
    qrError.value = true
  } finally {
    qrLoading.value = false
  }
}

function closeQr() {
  stopQrTimer()
  qrOpenFor.value = null
  qrChunks.value  = []
}

function qrPrev() {
  stopQrTimer()
  qrIdx.value = (qrIdx.value - 1 + qrChunks.value.length) % qrChunks.value.length
  startQrTimer()
}
function qrNext() {
  stopQrTimer()
  qrIdx.value = (qrIdx.value + 1) % qrChunks.value.length
  startQrTimer()
}

// ── Delete state ───────────────────────────────────────────────────────
const deleteFor  = ref<CabinetDevice | null>(null)
const deleteBusy = ref(false)

// ── Routes (split tunnel) sheet ───────────────────────────────────────
// Per-device, ephemeral. Picked services are not persisted server-side —
// the user re-imports the resulting .vpn / QR; server only learns the
// override at request time. See AmneziaVPNURLWith on the backend.
const routesFor       = ref<CabinetDevice | null>(null)
const routesMode      = ref<'all' | 'selected'>('all')
const routesAllowedIPs = ref('')

function openRoutes(d: CabinetDevice) {
  routesFor.value = d
  routesMode.value = 'all'
  routesAllowedIPs.value = ''
}
function closeRoutes() {
  routesFor.value = null
}
// The override actually sent to the server. "all" mode → empty string,
// which means "use server default" (0.0.0.0/0, ::/0 in most configs).
const effectiveAllowedIPs = computed(() =>
  routesMode.value === 'selected' ? routesAllowedIPs.value : ''
)

// ── Load ───────────────────────────────────────────────────────────────
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
function closeWizard() { wizardOpen.value = false; justAdded.value = null }

async function createDevice() {
  if (wizardStep.value === 'creating') return
  wizardErr.value  = ''
  const name       = customName.value.trim() || defaultName[pickedTemplate.value]
  wizardStep.value = 'creating'

  // Map the user-facing knobs onto genCfg's full parameter surface. "extreme"
  // mode pins intensity to 'high' and flips useExtremeMax — the underlying
  // junk parameters scale up automatically inside the generator.
  const intensity = advExtreme.value ? 'high' : advIntensity.value
  const cfg = genCfg({
    version: '2.0',
    intensity,
    profile: advProfile.value,
    customHost: '', mimicAll: false, useTagC: false,
    useTagT: true, useTagR: true, useTagRC: true, useTagRD: true,
    useBrowserFp: false, browserProfile: '',
    mtu: Math.max(576, Math.min(1500, Number(advMtu.value) || 1500)),
    junkLevel: 5, iterCount: 0, routerMode: false,
    useExtremeMax: advExtreme.value,
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
const amneziaVpnWith = (id: string, allowedIPs: string) =>
  api.cabinetDeviceAmneziaVpnUrl(token.value, id, allowedIPs)

// ── Copy vpn:// ───────────────────────────────────────────────────────────
const copiedId   = ref<string | null>(null)
const justCopied = ref(false)

async function copyVpn(devId: string) {
  try {
    const res = await fetch(amneziaVpn(devId))
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const text = await res.text()
    await navigator.clipboard.writeText(text)
    copiedId.value = devId
    setTimeout(() => { if (copiedId.value === devId) copiedId.value = null }, 2200)
  } catch (e: any) {
    toasts.error('Не удалось скопировать. Попробуйте «Скачать .vpn»')
  }
}

async function copyJustAddedVpn() {
  if (!justAdded.value) return
  try {
    const res = await fetch(amneziaVpn(justAdded.value.deviceId))
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const text = await res.text()
    await navigator.clipboard.writeText(text)
    justCopied.value = true
    setTimeout(() => { justCopied.value = false }, 2500)
  } catch (e: any) {
    toasts.error('Не удалось скопировать. Используйте кнопку скачивания')
  }
}

// ── Delete ────────────────────────────────────────────────────────────────
async function confirmDelete() {
  if (!deleteFor.value || deleteBusy.value) return
  deleteBusy.value = true
  try {
    await api.cabinetDeleteDevice(token.value, deleteFor.value.id)
    deleteFor.value = null
    await reload()
  } finally { deleteBusy.value = false }
}

// ── Status helpers ─────────────────────────────────────────────────────────
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
  try { return new Date(s).toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' }) }
  catch { return '' }
}

function devIcon(name: string) {
  const n = name.toLowerCase()
  if (/phone|iphone|android|телефон|самсунг|samsung|pixel/.test(n)) return Smartphone
  if (/laptop|ноутбук|macbook|ноут|notebook/.test(n))               return Laptop
  if (/desktop|компьютер|пк|\bpc\b|mac mini/.test(n))               return Monitor
  if (/tablet|ipad|планшет/.test(n))                                 return Smartphone
  return Key
}

const onlineCount = computed(() =>
  (cabinet.value?.devices ?? []).filter(d => devStatus(d) === 'online').length
)

// QR name for the open device
const qrDeviceName = computed(() =>
  cabinet.value?.devices.find(d => d.id === qrOpenFor.value)?.name ?? ''
)
</script>

<template>
  <div class="min-h-screen antialiased bg-ink-50 text-ink-900">

    <!-- Ambient glow — dark only -->
    <div
      class="pointer-events-none fixed inset-0 opacity-0 dark:opacity-100 transition-opacity duration-700"
      style="background: radial-gradient(ellipse 90% 45% at 50% -5%, rgba(232,160,65,0.08) 0%, transparent 65%)"
      aria-hidden="true"
    />

    <!-- ─── Loading ──────────────────────────────────────────────────── -->
    <div v-if="phase === 'loading'" class="min-h-screen flex items-center justify-center">
      <div class="flex flex-col items-center gap-5">
        <div class="relative w-14 h-14">
          <span class="absolute inset-0 rounded-full border-2 border-ink-200 border-t-amber-400 animate-spin block" />
          <span class="absolute inset-[7px] rounded-full bg-ink-100 flex items-center justify-center">
            <Shield :size="18" class="text-ink-500" />
          </span>
        </div>
        <p class="text-[13px] text-ink-500 tracking-wide">Загружаем кабинет…</p>
      </div>
    </div>

    <!-- ─── Invalid ─────────────────────────────────────────────────── -->
    <div v-else-if="phase === 'invalid'" class="min-h-screen flex items-center justify-center p-6">
      <div class="card w-full max-w-sm p-10 text-center space-y-5">
        <div class="w-16 h-16 rounded-full bg-danger/10 flex items-center justify-center mx-auto">
          <Lock :size="28" class="text-danger/70" />
        </div>
        <div class="space-y-2">
          <h1 class="text-[18px] font-semibold">Кабинет недоступен</h1>
          <p class="text-[13.5px] text-ink-500 leading-relaxed">
            Ссылка не существует или была отозвана.<br>Свяжитесь с администратором.
          </p>
        </div>
      </div>
    </div>

    <!-- ─── Ready ───────────────────────────────────────────────────── -->
    <template v-else-if="cabinet">
      <div class="relative max-w-md mx-auto px-5 pt-8 pb-24">

        <!-- ── Header ── -->
        <header class="mb-10 animate-rise">
          <!-- Top row: brand on left, theme toggle on right. Replaces the
               floating absolute toggle that previously had no anchor. -->
          <div class="flex items-center justify-between mb-8">
            <div class="inline-flex items-center gap-1.5 text-[10px] uppercase tracking-[0.20em] text-ink-400 font-semibold">
              <Shield :size="11" class="shrink-0" />
              <span>AmneziaVPN · Личный кабинет</span>
            </div>
            <button
              class="h-9 w-9 flex items-center justify-center rounded-xl text-ink-500 hover:text-ink-900 hover:bg-ink-100 dark:hover:bg-ink-100/60 dark:hover:text-ink-900 transition-colors"
              :title="isDark ? 'Светлая тема' : 'Тёмная тема'"
              :aria-label="isDark ? 'Включить светлую тему' : 'Включить тёмную тему'"
              @click="toggleTheme">
              <Sun v-if="isDark" :size="18" />
              <Moon v-else :size="18" />
            </button>
          </div>

          <div class="text-center">

          <h1 class="text-[56px] sm:text-[64px] font-bold tracking-tight leading-none text-ink-900 mb-5">
            {{ cabinet.name }}
          </h1>

          <!-- Status chips -->
          <div class="flex items-center justify-center gap-2 flex-wrap">
            <span class="inline-flex items-center gap-1.5 px-3.5 py-1.5 rounded-full bg-ink-100 dark:bg-ink-200/60 text-[12px] text-ink-600 dark:text-ink-500 font-medium">
              {{ cabinet.devices.length }}
              {{ cabinet.devices.length === 1 ? 'устройство' : cabinet.devices.length < 5 ? 'устройства' : 'устройств' }}
            </span>
            <span
              v-if="onlineCount > 0"
              class="inline-flex items-center gap-1.5 px-3.5 py-1.5 rounded-full bg-success/10 text-[12px] text-success font-semibold">
              <span class="live-dot scale-75 shrink-0" />
              {{ onlineCount }} онлайн
            </span>
            <span
              v-else-if="cabinet.devices.length > 0"
              class="inline-flex items-center gap-1.5 px-3.5 py-1.5 rounded-full bg-ink-100 dark:bg-ink-200/60 text-[12px] text-ink-500 dark:text-ink-600">
              нет подключений
            </span>
          </div>
          </div>
        </header>

        <!-- ── Empty state ── -->
        <div
          v-if="!cabinet.devices.length"
          class="rounded-3xl border-2 border-dashed border-ink-200 dark:border-ink-300/40 p-10 text-center space-y-7 animate-rise">
          <div class="space-y-3">
            <div class="w-16 h-16 rounded-full bg-ink-100 dark:bg-ink-200/50 flex items-center justify-center mx-auto">
              <Key :size="28" class="text-ink-500" />
            </div>
            <p class="text-[17px] font-semibold">Каждое устройство — свой ключ</p>
            <p class="text-[13px] text-ink-500 leading-relaxed max-w-[260px] mx-auto">
              Телефон, ноутбук, планшет — у каждого отдельный VPN-ключ.
              Потеряли устройство — отзываете только его ключ.
            </p>
          </div>

          <div class="grid grid-cols-3 gap-2.5 max-w-[280px] mx-auto">
            <button
              v-for="t in templates.slice(0, 3)" :key="t.key"
              class="flex flex-col items-center gap-2.5 p-3.5 rounded-2xl border border-ink-200 dark:border-ink-300/40 hover:border-ink-400 hover:bg-ink-100/60 dark:hover:bg-ink-200/30 transition-all active:scale-[0.96]"
              @click="openWizard(t.key)">
              <component :is="t.icon" :size="24" class="text-ink-600 dark:text-ink-500" />
              <span class="text-[11px] font-semibold text-ink-600 dark:text-ink-500">{{ t.label }}</span>
            </button>
          </div>

          <Button variant="accent" size="lg" block class="!max-w-[280px] mx-auto" @click="openWizard()">
            Добавить первое устройство
          </Button>
        </div>

        <!-- ── Device list ── -->
        <section v-else class="space-y-3">

          <div
            v-for="(d, i) in cabinet.devices"
            :key="d.id"
            class="card device-card overflow-hidden animate-rise"
            :class="`delay-${Math.min(i + 1, 6)}`">

            <!-- Card body -->
            <div class="p-4">
              <!-- Top row: icon + name + status -->
              <div class="flex items-start gap-3.5 mb-3.5">
                <!-- Device icon with status colour -->
                <div
                  class="w-11 h-11 rounded-2xl flex items-center justify-center shrink-0"
                  :class="{
                    'bg-success/12': devStatus(d) === 'online',
                    'bg-warning/10': devStatus(d) === 'recent',
                    'bg-ink-100 dark:bg-ink-200/50': devStatus(d) === 'away' || devStatus(d) === 'never',
                  }">
                  <component
                    :is="devIcon(d.name)"
                    :size="20"
                    :class="{
                      'text-success':  devStatus(d) === 'online',
                      'text-warning':  devStatus(d) === 'recent',
                      'text-ink-500':  devStatus(d) === 'away' || devStatus(d) === 'never',
                    }" />
                </div>

                <!-- Name + address + last seen -->
                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-2 mb-1">
                    <span class="text-[15px] font-semibold leading-tight truncate text-ink-900">{{ d.name }}</span>
                    <Badge v-if="!d.enabled" tone="danger" size="xs" class="shrink-0">выкл</Badge>
                  </div>
                  <div class="flex items-center gap-1.5 flex-wrap">
                    <span class="mono text-[11px] text-ink-500 bg-ink-100 dark:bg-ink-200/50 px-1.5 py-0.5 rounded-md leading-tight">{{ d.address }}</span>
                    <span class="text-ink-300 text-[9px] select-none">·</span>
                    <span
                      class="text-[11.5px]"
                      :class="{
                        'text-success font-medium': devStatus(d) === 'online',
                        'text-warning font-medium': devStatus(d) === 'recent',
                        'text-ink-500':             devStatus(d) === 'away' || devStatus(d) === 'never',
                      }">
                      <template v-if="devStatus(d) === 'online'">онлайн</template>
                      <template v-else>{{ relTime(d.latestHandshakeAt) }}</template>
                    </span>
                  </div>
                </div>
              </div>

              <!-- Info strip -->
              <div class="flex items-center gap-2.5 text-[11px] text-ink-400 dark:text-ink-500 pb-3.5 border-b border-ink-900/6 dark:border-ink-900/10">
                <span>Добавлено {{ fmtDate(d.createdAt) }}</span>
                <span class="text-ink-200 dark:text-ink-300 select-none">·</span>
                <span class="mono">AmneziaWG 2.0</span>
                <span class="ml-auto inline-flex items-center gap-1">
                  <Lock :size="10" />
                  зашифровано
                </span>
              </div>

              <!-- Actions -->
              <div class="flex items-center gap-2 pt-3.5">
                <!-- QR — primary, opens inline fullscreen -->
                <Button
                  variant="accent"
                  class="flex-1 !text-[12.5px]"
                  :title="`QR-код для ${d.name}`"
                  :aria-label="`Открыть QR-код для ${d.name}`"
                  @click="openQr(d.id)">
                  <QrCode :size="14" />
                  QR-код
                </Button>

                <!-- .vpn download -->
                <a
                  :href="amneziaVpn(d.id)"
                  :download="`${d.name}.vpn`"
                  class="btn-secondary flex-1 flex items-center justify-center gap-1.5 h-10 text-[12.5px] font-semibold"
                  title="Скачать .vpn">
                  <Download :size="14" />
                  .vpn
                </a>

                <!-- Routes — split tunnel sheet -->
                <button
                  class="h-10 w-10 flex items-center justify-center rounded-xl text-ink-400 hover:text-ink-700 hover:bg-ink-100 dark:hover:bg-ink-200/50 transition-all shrink-0"
                  title="Маршруты — выбрать сервисы для VPN"
                  :aria-label="`Настроить маршруты для ${d.name}`"
                  @click="openRoutes(d)">
                  <Compass :size="15" />
                </button>

                <!-- Copy vpn:// -->
                <button
                  class="h-10 w-10 flex items-center justify-center rounded-xl transition-all shrink-0"
                  :class="copiedId === d.id
                    ? 'bg-success/12 text-success'
                    : 'text-ink-400 hover:text-ink-700 hover:bg-ink-100 dark:hover:bg-ink-200/50'"
                  :title="copiedId === d.id ? 'Скопировано' : 'Скопировать vpn://'"
                  @click="copyVpn(d.id)">
                  <Check v-if="copiedId === d.id" :size="15" />
                  <Copy v-else :size="15" />
                </button>

                <!-- Delete -->
                <button
                  class="h-10 w-10 flex items-center justify-center rounded-xl text-ink-400 hover:bg-danger/10 hover:text-danger transition-all shrink-0"
                  title="Удалить устройство"
                  @click="deleteFor = d">
                  <Trash2 :size="15" />
                </button>
              </div>
            </div>

          </div>

          <!-- Add more -->
          <button
            class="w-full h-14 flex items-center justify-center gap-2 rounded-3xl border-2 border-dashed border-ink-200 dark:border-ink-300/40 text-ink-500 dark:text-ink-600 text-[14px] font-medium hover:border-ink-400 dark:hover:border-ink-400/60 hover:text-ink-700 dark:hover:text-ink-500 active:scale-[0.99] transition-all mt-1"
            @click="openWizard()">
            <Plus :size="18" />
            Добавить устройство
          </button>
        </section>

        <p class="text-center text-[11.5px] text-ink-400 dark:text-ink-500 mt-12 leading-relaxed">
          Потеряли ссылку на кабинет?<br>
          Попросите администратора выпустить новую.
        </p>

      </div>
    </template>

    <!-- ─── QR Fullscreen carousel ───────────────────────────────────── -->
    <!--
      Uses the .scrim class (with a heavier local override of --scrim-alpha)
      so the backdrop-filter is composited from the first frame, matching
      the lag-free behavior of the regular modal scrim. Bare black at 0.96
      is dark enough that the QR (white card) reads cleanly.
    -->
    <Teleport to="body">
      <div
        v-if="qrOpenFor"
        class="fixed inset-0 z-50 scrim flex flex-col items-center justify-center gap-4"
        style="--scrim-alpha: 0.96"
        @click.self="closeQr">

          <!-- Close -->
          <button
            class="absolute top-5 right-5 w-10 h-10 flex items-center justify-center rounded-full bg-white/10 hover:bg-white/20 text-white transition-colors z-10"
            @click="closeQr">
            <X :size="18" />
          </button>

          <!-- Header: chunk counter -->
          <div class="flex items-center gap-3 h-7">
            <template v-if="qrChunks.length > 1">
              <span class="text-white/40 text-[12px] font-mono">
                {{ qrIdx + 1 }} / {{ qrChunks.length }}
              </span>
              <!-- Progress dots -->
              <div class="flex items-center gap-1.5">
                <span
                  v-for="(_, i) in qrChunks"
                  :key="`${qrOpenFor}-${i}`"
                  class="rounded-full transition-all duration-300"
                  :class="i === qrIdx
                    ? 'w-4 h-2 bg-amber-400'
                    : 'w-2 h-2 bg-white/25'"
                />
              </div>
            </template>
          </div>

          <!-- QR image area -->
          <div class="relative flex items-center justify-center"
               style="width: min(88vw, 76vh); height: min(88vw, 76vh)">

            <!-- Loading -->
            <div v-if="qrLoading"
                 class="absolute inset-0 flex items-center justify-center bg-white rounded-2xl">
              <Loader2 :size="36" class="text-ink-300 animate-spin" />
            </div>

            <!-- Error -->
            <div v-else-if="qrError"
                 class="absolute inset-0 flex flex-col items-center justify-center gap-3 bg-white rounded-2xl p-8 text-center">
              <p class="text-[14px] font-semibold text-ink-700">Не удалось загрузить QR</p>
              <p class="text-[12px] text-ink-500">Используйте файл .vpn для импорта</p>
            </div>

            <!-- QR chunks -->
            <template v-else-if="qrChunks.length">
              <Transition
                enter-active-class="transition-opacity duration-200"
                leave-active-class="transition-opacity duration-150"
                enter-from-class="opacity-0"
                leave-to-class="opacity-0"
                mode="out-in">
                <div
                  :key="qrIdx"
                  class="w-full h-full bg-white rounded-2xl p-3 shadow-2xl">
                  <img
                    :src="`data:image/png;base64,${qrChunks[qrIdx]}`"
                    :alt="`QR часть ${qrIdx + 1} из ${qrChunks.length}`"
                    class="w-full h-full block"
                    style="image-rendering: pixelated"
                  />
                </div>
              </Transition>

              <!-- Prev / Next (only when multiple chunks) -->
              <template v-if="qrChunks.length > 1">
                <button
                  class="absolute left-0 -translate-x-12 w-9 h-9 flex items-center justify-center rounded-full bg-white/10 hover:bg-white/20 text-white transition-colors"
                  @click="qrPrev">
                  <ChevronLeft :size="18" />
                </button>
                <button
                  class="absolute right-0 translate-x-12 w-9 h-9 flex items-center justify-center rounded-full bg-white/10 hover:bg-white/20 text-white transition-colors"
                  @click="qrNext">
                  <ChevronRight :size="18" />
                </button>
              </template>
            </template>
          </div>

          <!-- Instructions -->
          <div class="text-center space-y-1 px-4">
            <p class="text-white font-semibold text-[15px]">
              <template v-if="qrChunks.length > 1">
                Наведите камеру · сканирует по очереди
              </template>
              <template v-else>
                Отсканируйте в AmneziaVPN
              </template>
            </p>
            <p class="text-white/40 text-[12px]">Android · iOS · Windows · macOS · Linux</p>
          </div>

          <!-- Download fallback -->
          <a
            v-if="qrOpenFor"
            :href="amneziaVpn(qrOpenFor)"
            :download="`${qrDeviceName}.vpn`"
            class="flex items-center gap-1.5 px-5 py-2.5 rounded-xl bg-white/10 hover:bg-white/20 text-white text-[13px] font-medium transition-colors"
            @click.stop>
            <Download :size="14" />
            Скачать .vpn
          </a>
        </div>
    </Teleport>

    <!-- ─── Add-device wizard ────────────────────────────────────────── -->
    <Teleport to="body">
      <!-- Scrim mounts directly so backdrop-filter composites on frame 1 — no blur lag. -->
      <div
        v-if="wizardOpen"
        class="fixed inset-0 z-50 scrim"
        @click="wizardStep !== 'creating' ? closeWizard() : undefined"
      />
      <Transition name="sheet">
        <div
          v-if="wizardOpen"
          class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-6 pointer-events-none">
          <div class="sheet-panel relative w-full sm:max-w-md bg-surface-raised rounded-t-5xl sm:rounded-5xl shadow-pop overflow-hidden pointer-events-auto">

            <!-- ── Step: Pick ── -->
            <div v-if="wizardStep === 'pick'" class="p-6 space-y-6">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <h3 class="text-[19px] font-semibold">Новый VPN-ключ</h3>
                  <p class="text-[12.5px] text-ink-500 mt-0.5">Каждый ключ — только для одного устройства</p>
                </div>
                <button
                  class="w-9 h-9 rounded-full flex items-center justify-center text-ink-400 hover:bg-ink-100 dark:hover:bg-ink-200/50 transition-colors shrink-0"
                  @click="closeWizard">
                  <X :size="16" />
                </button>
              </div>

              <div>
                <p class="text-[11px] font-semibold text-ink-500 uppercase tracking-[0.12em] mb-3">Тип устройства</p>
                <div class="grid grid-cols-4 gap-2">
                  <button
                    v-for="t in templates" :key="t.key"
                    class="flex flex-col items-center gap-2.5 py-3.5 px-1 rounded-2xl transition-colors duration-150 active:translate-y-px"
                    :class="pickedTemplate === t.key
                      ? 'bg-amber-400/15 dark:bg-amber-400/15 shadow-[inset_0_0_0_2px_theme(colors.amber.400)]'
                      : 'bg-ink-100 hover:bg-ink-200'"
                    @click="pickedTemplate = t.key">
                    <component
                      :is="t.icon"
                      :size="22"
                      :class="pickedTemplate === t.key ? 'text-amber-500' : 'text-ink-500'" />
                    <span class="text-[10.5px] font-semibold text-ink-600 dark:text-ink-500 leading-tight">{{ t.label }}</span>
                  </button>
                </div>
              </div>

              <div class="space-y-2">
                <label class="text-[11px] font-semibold text-ink-500 uppercase tracking-[0.12em] block">
                  Название <span class="normal-case tracking-normal font-normal text-ink-400">— необязательно</span>
                </label>
                <input
                  v-model="customName"
                  class="w-full h-12 px-4 rounded-2xl bg-ink-100 text-[14px] text-ink-900 placeholder:text-ink-400 outline-none transition-colors duration-150 focus:bg-amber-50 dark:focus:bg-amber-400/10"
                  :placeholder="defaultName[pickedTemplate]"
                  @keydown.enter="createDevice" />
              </div>

              <!-- ── Advanced (collapsed) ── -->
              <details class="rounded-2xl bg-ink-100/60 dark:bg-ink-200/30 group">
                <summary class="cursor-pointer list-none flex items-center justify-between px-4 py-3 select-none">
                  <span class="text-[12px] font-semibold text-ink-600 dark:text-ink-500">
                    Дополнительно
                  </span>
                  <ChevronRight :size="14" class="text-ink-400 transition-transform group-open:rotate-90" />
                </summary>
                <div class="px-4 pb-4 pt-1 space-y-4">

                  <!-- Intensity -->
                  <div>
                    <label class="text-[10.5px] font-semibold text-ink-500 uppercase tracking-[0.12em] block mb-2">
                      Уровень обфускации
                    </label>
                    <div class="grid grid-cols-3 gap-1.5">
                      <button
                        v-for="i in INTENSITIES" :key="i.v"
                        type="button"
                        class="flex flex-col items-start py-2 px-2.5 rounded-xl text-left transition-colors duration-150 active:translate-y-px"
                        :class="advIntensity === i.v
                          ? 'bg-amber-400/15 shadow-[inset_0_0_0_2px_theme(colors.amber.400)]'
                          : 'bg-ink-100 hover:bg-ink-200'"
                        @click="advIntensity = i.v">
                        <span class="text-[12px] font-semibold leading-tight">{{ i.label }}</span>
                        <span class="text-[10px] text-ink-500 leading-tight mt-0.5">{{ i.hint }}</span>
                      </button>
                    </div>
                  </div>

                  <!-- Mimic profile -->
                  <div>
                    <label class="text-[10.5px] font-semibold text-ink-500 uppercase tracking-[0.12em] block mb-2">
                      Маскировка под
                    </label>
                    <div class="grid grid-cols-2 gap-1.5">
                      <button
                        v-for="p in PROFILES" :key="p.v"
                        type="button"
                        class="py-2 px-3 rounded-xl text-[12px] font-semibold transition-colors duration-150 active:translate-y-px"
                        :class="advProfile === p.v
                          ? 'bg-amber-400/15 shadow-[inset_0_0_0_2px_theme(colors.amber.400)]'
                          : 'bg-ink-100 hover:bg-ink-200'"
                        @click="advProfile = p.v">{{ p.label }}</button>
                    </div>
                  </div>

                  <!-- MTU -->
                  <div>
                    <label class="text-[10.5px] font-semibold text-ink-500 uppercase tracking-[0.12em] block mb-2">
                      MTU
                    </label>
                    <input
                      v-model.number="advMtu"
                      type="number" min="576" max="1500" step="1"
                      class="w-full h-10 px-3.5 rounded-xl bg-ink-100 text-[13px] text-ink-900 outline-none focus:bg-amber-50 dark:focus:bg-amber-400/10 transition-colors"
                    />
                    <p class="mt-1 text-[10.5px] text-ink-500">По умолчанию 1500. Уменьшите до 1280, если есть проблемы со связью.</p>
                  </div>

                  <!-- Extreme toggle -->
                  <label class="flex items-start gap-3 cursor-pointer p-3 rounded-xl bg-ink-100 hover:bg-ink-200 transition-colors">
                    <input
                      v-model="advExtreme"
                      type="checkbox"
                      class="mt-0.5 h-4 w-4 rounded accent-amber-500"
                    />
                    <div class="min-w-0 flex-1">
                      <p class="text-[12.5px] font-semibold leading-tight">Усиленный режим</p>
                      <p class="text-[11px] text-ink-500 mt-0.5 leading-snug">Для самых строгих сетей (Иран, Туркменистан, школьный WiFi). Включает максимальное junk-наполнение.</p>
                    </div>
                  </label>
                </div>
              </details>

              <p v-if="wizardErr" class="text-[12.5px] text-danger bg-danger/10 rounded-xl px-4 py-3">{{ wizardErr }}</p>

              <Button variant="accent" size="xl" block @click="createDevice">
                Получить VPN-ключ
              </Button>

              <p class="text-[11.5px] text-ink-500 text-center pb-1">
                Уникальная защита AmneziaWG 2.0 создаётся автоматически
              </p>
            </div>

            <!-- ── Step: Creating ── -->
            <div v-else-if="wizardStep === 'creating'"
                 class="p-10 flex flex-col items-center gap-7 min-h-[300px] justify-center">
              <div class="relative w-[72px] h-[72px]">
                <span class="absolute inset-0 rounded-full border-[3px] border-ink-200 border-t-amber-400 animate-spin block" />
                <span class="absolute inset-[10px] rounded-full bg-ink-100 dark:bg-ink-200/50 flex items-center justify-center">
                  <Key :size="22" class="text-ink-600 dark:text-ink-500" />
                </span>
              </div>
              <div class="text-center space-y-1.5">
                <p class="text-[16px] font-semibold">Создаём ключ…</p>
                <p class="text-[13px] text-ink-500">Генерируем уникальную защиту</p>
              </div>
            </div>

            <!-- ── Step: Done ── -->
            <div v-else-if="wizardStep === 'done' && justAdded" class="p-6 space-y-5 animate-fade-in">
              <div class="flex items-start justify-between gap-3">
                <div class="space-y-1">
                  <div class="flex items-center gap-2">
                    <div class="w-6 h-6 rounded-full bg-success/15 flex items-center justify-center">
                      <Check :size="13" class="text-success" />
                    </div>
                    <h3 class="text-[18px] font-semibold">Ключ готов!</h3>
                  </div>
                  <p class="text-[12.5px] text-ink-500 pl-8">
                    {{ justAdded.name }} <span class="mono">· {{ justAdded.address }}</span>
                  </p>
                </div>
                <button
                  class="w-9 h-9 rounded-full flex items-center justify-center text-ink-400 hover:bg-ink-100 dark:hover:bg-ink-200/50 transition-colors shrink-0"
                  @click="closeWizard">
                  <X :size="16" />
                </button>
              </div>

              <!--
                Amnezia QR hero — chunked carousel.
                The legacy single-PNG endpoint produced un-scannable QRs once
                the obfuscated payload exceeded ~3KB; we reuse the same
                chunked carousel that the device-card fullscreen viewer uses.
              -->
              <div class="flex flex-col items-center gap-3 py-1">
                <QrCarousel
                  :token="token"
                  :device-id="justAdded.deviceId"
                  :device-name="justAdded.name"
                  :size="240"
                />
                <div class="text-center">
                  <p class="text-[12.5px] font-semibold text-ink-700 dark:text-ink-600">Отсканируйте в приложении AmneziaVPN</p>
                  <p class="text-[11px] text-ink-500 mt-0.5">Android · iOS · Windows · macOS · Linux</p>
                </div>
              </div>

              <a
                :href="amneziaVpn(justAdded.deviceId)"
                :download="`${justAdded.name}.vpn`"
                class="btn-primary flex w-full h-13 items-center justify-center gap-2 text-[14.5px] py-3.5">
                <Download :size="16" />
                Скачать .vpn файл
              </a>

              <div class="flex gap-2">
                <button
                  class="flex-1 h-11 flex items-center justify-center gap-1.5 rounded-xl text-[12.5px] font-semibold transition-all"
                  :class="justCopied ? 'bg-success/12 text-success' : 'btn-secondary'"
                  @click="copyJustAddedVpn">
                  <Check v-if="justCopied" :size="14" />
                  <Copy v-else :size="14" />
                  {{ justCopied ? 'Скопировано' : 'Скопировать vpn://' }}
                </button>
              </div>

              <p class="text-[11.5px] text-ink-500 text-center leading-relaxed">
                Этот ключ только для «{{ justAdded.name }}».<br>
                Каждое устройство — свой ключ.
              </p>

              <button
                class="w-full h-11 flex items-center justify-center gap-1.5 rounded-xl bg-ink-100 hover:bg-ink-200 text-ink-700 text-[13px] font-medium transition-colors duration-150 active:translate-y-px"
                @click="openWizard()">
                <Plus :size="15" />
                Добавить ещё устройство
              </button>
            </div>

          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- ─── Delete confirm ───────────────────────────────────────────── -->
    <Teleport to="body">
      <!-- Scrim mounted directly — no opacity transition wrapping it. -->
      <div v-if="deleteFor" class="fixed inset-0 z-50 scrim" @click="deleteFor = null" />
      <Transition
        enter-active-class="transition-[opacity,transform] duration-150"
        leave-active-class="transition-[opacity,transform] duration-120"
        enter-from-class="opacity-0 translate-y-2"
        leave-to-class="opacity-0 translate-y-2">
        <div v-if="deleteFor" class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4 pointer-events-none">
          <div class="relative w-full sm:max-w-sm bg-surface-raised rounded-t-4xl sm:rounded-4xl shadow-pop p-6 space-y-4 pointer-events-auto">
            <h3 class="text-[17px] font-semibold">Удалить устройство?</h3>
            <p class="text-[13.5px] text-ink-500 leading-relaxed">
              <span class="font-semibold text-ink-800 dark:text-ink-700">{{ deleteFor.name }}</span> сразу потеряет подключение.
              Восстановить нельзя — нужно создать заново.
            </p>
            <div class="flex gap-2 pt-1">
              <Button
                variant="secondary"
                block
                class="flex-1 !h-12 !rounded-2xl !text-[13px] !font-semibold"
                :disabled="deleteBusy"
                @click="deleteFor = null">
                Отмена
              </Button>
              <Button
                variant="danger"
                block
                class="flex-1 !h-12 !rounded-2xl !text-[13px] !font-semibold !bg-danger !text-white hover:!bg-danger/90"
                :loading="deleteBusy"
                @click="confirmDelete">
                {{ deleteBusy ? 'Удаляем…' : 'Удалить' }}
              </Button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- ─── Routes (split tunnel) sheet ──────────────────────────────── -->
    <Teleport to="body">
      <div v-if="routesFor" class="fixed inset-0 z-50 scrim" @click="closeRoutes" />
      <Transition name="sheet">
        <div
          v-if="routesFor"
          class="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-6 pointer-events-none">
          <div class="sheet-panel relative w-full sm:max-w-lg bg-surface-raised rounded-t-5xl sm:rounded-5xl shadow-pop overflow-hidden pointer-events-auto max-h-[92vh] flex flex-col">

            <!-- Header -->
            <div class="flex items-start justify-between gap-3 px-6 pt-5 pb-3 shrink-0">
              <div class="space-y-0.5">
                <h3 class="text-[18px] font-semibold leading-tight">Маршруты</h3>
                <p class="text-[12px] text-ink-500">
                  {{ routesFor.name }} · <span class="mono">{{ routesFor.address }}</span>
                </p>
              </div>
              <button
                class="w-9 h-9 rounded-full flex items-center justify-center text-ink-400 hover:bg-ink-100 dark:hover:bg-ink-200/50 transition-colors shrink-0"
                aria-label="Закрыть"
                @click="closeRoutes">
                <X :size="16" />
              </button>
            </div>

            <!-- Mode picker + body — scroll if tall -->
            <div class="px-6 pb-5 pt-1 space-y-5 overflow-y-auto">

              <!-- Mode radios -->
              <div class="grid grid-cols-2 gap-2">
                <button
                  type="button"
                  class="text-left p-3.5 rounded-2xl transition-colors duration-150 active:translate-y-px"
                  :class="routesMode === 'all'
                    ? 'bg-amber-400/15 shadow-[inset_0_0_0_2px_theme(colors.amber.400)]'
                    : 'bg-ink-100 hover:bg-ink-200'"
                  @click="routesMode = 'all'">
                  <div class="flex items-center gap-2 mb-1">
                    <Shield :size="14" :class="routesMode === 'all' ? 'text-amber-500' : 'text-ink-500'" />
                    <span class="text-[13px] font-semibold">Весь трафик</span>
                  </div>
                  <p class="text-[11.5px] text-ink-500 leading-snug">Полный VPN — всё уходит в туннель.</p>
                </button>
                <button
                  type="button"
                  class="text-left p-3.5 rounded-2xl transition-colors duration-150 active:translate-y-px"
                  :class="routesMode === 'selected'
                    ? 'bg-amber-400/15 shadow-[inset_0_0_0_2px_theme(colors.amber.400)]'
                    : 'bg-ink-100 hover:bg-ink-200'"
                  @click="routesMode = 'selected'">
                  <div class="flex items-center gap-2 mb-1">
                    <Compass :size="14" :class="routesMode === 'selected' ? 'text-amber-500' : 'text-ink-500'" />
                    <span class="text-[13px] font-semibold">Выбранные сервисы</span>
                  </div>
                  <p class="text-[11.5px] text-ink-500 leading-snug">Только нужные сайты пойдут через VPN.</p>
                </button>
              </div>

              <!-- Service picker — only in selected mode -->
              <SplitTunnelPicker
                v-if="routesMode === 'selected'"
                :value="routesAllowedIPs"
                @update:value="v => routesAllowedIPs = v"
              />

              <!-- Preview: QR + .vpn — updates live when allowedIPs change -->
              <div class="flex flex-col items-center gap-4 pt-2">
                <QrCarousel
                  :token="token"
                  :device-id="routesFor.id"
                  :device-name="routesFor.name"
                  :allowed-ips="effectiveAllowedIPs"
                  :size="220"
                />
                <a
                  :href="amneziaVpnWith(routesFor.id, effectiveAllowedIPs)"
                  :download="`${routesFor.name}.vpn`"
                  class="btn-primary inline-flex items-center gap-2 px-5 h-11 text-[13.5px]">
                  <Download :size="15" />
                  Скачать .vpn с этими маршрутами
                </a>
                <p class="text-[11.5px] text-ink-500 text-center max-w-xs leading-relaxed">
                  После импорта новый профиль заменит старый в приложении AmneziaVPN.
                  Серверные ключи не меняются — это просто другой пресет маршрутов.
                </p>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

  </div>
</template>
