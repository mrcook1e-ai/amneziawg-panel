<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed, watch } from 'vue'
import { useTitle } from '@/composables/useTitle'
import { useRoute } from 'vue-router'
import {
  Shield, Lock, Key, Smartphone, Laptop, Monitor,
  QrCode, Download, Copy, Check, Trash2, X,
  Sun, Moon, Plus, ChevronLeft, ChevronRight, Loader2,
  MoreHorizontal, Globe, ExternalLink, Zap, EyeOff, Gauge,
} from 'lucide-vue-next'

import { api } from '@/lib/api'
import { useThemeStore } from '@/stores/theme'
import { useToastStore } from '@/stores/toasts'
import type { CabinetView, CabinetDevice, AddDeviceResult, CabinetBillingSummary } from '@/types'
  import { genCfg, snippetFromCfg } from '@/utils/generator'
import Button from '@/components/atoms/Button.vue'
import Badge from '@/components/atoms/Badge.vue'
import IconButton from '@/components/atoms/IconButton.vue'
import Input from '@/components/atoms/Input.vue'
import StatusDot from '@/components/atoms/StatusDot.vue'
import Modal from '@/components/molecules/Modal.vue'
import ConfirmDialog from '@/components/molecules/ConfirmDialog.vue'
import QrCarousel from '@/components/molecules/QrCarousel.vue'
import DropdownMenu from '@/components/molecules/DropdownMenu.vue'
import DropdownItem from '@/components/molecules/DropdownItem.vue'
import DropdownSeparator from '@/components/molecules/DropdownSeparator.vue'
import Field from '@/components/molecules/Field.vue'
import CabinetBillingCard from '@/components/organisms/CabinetBillingCard.vue'

const route = useRoute()
const token  = computed(() => String(route.params.token || ''))
const theme  = useThemeStore()
const toasts = useToastStore()

type Phase = 'loading' | 'invalid' | 'ready'
const phase   = ref<Phase>('loading')
const cabinet = ref<CabinetView | null>(null)
const billing = ref<CabinetBillingSummary | null>(null)
const billingBlocked = computed(() => billing.value?.derivedStatus === 'overdue')
const checkoutOpen = ref(false)
const checkoutEmail = ref('')
const checkoutBusy = ref(false)
const checkoutError = ref('')

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
// Two-stage flow: pick (type + name) → config (protection profile) →
// creating → done. The config step is opt-out-friendly: 'Авто' is
// pre-selected, the user can just tap "Создать ключ" without thinking.
type WizardStep = 'pick' | 'config' | 'split' | 'creating' | 'done'
const wizardOpen     = ref(false)
const wizardStep     = ref<WizardStep>('pick')
const wizardErr      = ref('')
const justAdded      = ref<AddDeviceResult | null>(null)

type DeviceTemplate = 'phone' | 'laptop' | 'desktop' | 'other'
const pickedTemplate = ref<DeviceTemplate>('phone')
const customName     = ref('')

import type { Intensity, MimicProfile } from '@/utils/generator'

/*
  Protection profile — 3 named presets bake the right combinations of
  intensity/profile/mtu/extreme. Avg user picks "Авто" and forgets;
  power users can flip to "Тихий" for strict networks (Iran, Turkmenistan,
  school WiFi) or "Быстрый" for low-latency / low-overhead.
*/
type PresetKey = 'auto' | 'stealth' | 'fast'

interface Params {
  intensity: Intensity
  profile:   MimicProfile
  mtu:       number
  extreme:   boolean
}

interface PresetDef {
  v: PresetKey
  label: string
  hint: string
  icon: any
  params: Params
}

const PRESETS: PresetDef[] = [
  {
    v: 'auto', label: 'Авто', icon: Shield,
    hint: 'Баланс скорости и обхода — рекомендуется',
    // mtu 1280 matches typical WG_MTU and keeps junk under path MTU
    params: { intensity: 'medium', profile: 'quic_initial', mtu: 1280, extreme: false },
  },
  {
    v: 'stealth', label: 'Тихий', icon: EyeOff,
    hint: 'Сильнее H/S/Jc (без I1–I5 — стабильнее на WAN)',
    params: { intensity: 'high', profile: 'tls_client_hello', mtu: 1280, extreme: true },
  },
  {
    v: 'fast', label: 'Быстрый', icon: Gauge,
    hint: 'Минимум обфускации, ниже задержка',
    params: { intensity: 'low', profile: 'random', mtu: 1280, extreme: false },
  },
]

const pickedPreset = ref<PresetKey>('auto')

function presetParams(): Params {
  return (PRESETS.find(p => p.v === pickedPreset.value) || PRESETS[0]).params
}

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

// Stop timer whenever QR modal closes (covers ESC, backdrop, X button)
watch(qrOpenFor, (val) => { if (!val) stopQrTimer() })

function onKeyDown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    if (qrOpenFor.value) { closeQr(); return }
    if (wizardOpen.value && wizardStep.value !== 'creating') closeWizard()
  }
}
onMounted(() => document.addEventListener('keydown', onKeyDown))
onBeforeUnmount(() => document.removeEventListener('keydown', onKeyDown))

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

// ── Blocked-resources list URL (account-wide, not per-device) ─────────
// AmneziaVPN imports split-tunnel rules inside its own app — we don't
// inject them into the .vpn config. So we just hand the user a download
// link to the curated iplist.opencck.org "amnezia" format and show the
// in-app import steps. One link covers all devices on the account.
const IPLIST_URL = 'https://iplist.opencck.org/?format=amnezia&data=cidr4&filesave=1'

// Sites modal — the full info+download lives in a popup, triggered by
// a small button below the device list. The card-on-page version was
// visually heavy and competed with the actual devices for attention.
const sitesOpen = ref(false)

// ── Load ───────────────────────────────────────────────────────────────
async function reload() {
  try {
    cabinet.value = await api.cabinetGet(token.value)
		 try { billing.value = await api.cabinetBilling(token.value) } catch { billing.value = null }
    phase.value   = 'ready'
  } catch {
    phase.value = 'invalid'
  }
}
onMounted(async () => {
	await reload()
	if (route.query.payment === 'success') toasts.success('Оплата подтверждена. Спасибо!')
	else if (route.query.payment === 'pending') toasts.info('Платёж обрабатывается. Статус обновится автоматически.')
})

function openCheckout() {
	checkoutError.value = ''
	checkoutOpen.value = true
}

async function startCheckout() {
	if (!billing.value?.latestInvoice || checkoutBusy.value) return
	checkoutError.value = ''
	checkoutBusy.value = true
	try {
		const result = await api.cabinetCheckout(token.value, billing.value.latestInvoice.id, checkoutEmail.value.trim())
		window.location.assign(result.confirmationUrl)
	} catch (e: any) {
		checkoutError.value = e?.message || 'Не удалось открыть оплату'
	} finally { checkoutBusy.value = false }
}

// ── Wizard ──────────────────────────────────────────────────────────────
function openWizard(tpl: DeviceTemplate = 'phone') {
	if (billingBlocked.value) {
		toasts.error('Сначала оплатите просроченный счёт')
		return
	}
  pickedTemplate.value = tpl
  customName.value     = ''
  pickedPreset.value   = 'auto'
  wizardErr.value      = ''
  justAdded.value      = null
  wizardStep.value     = 'pick'
  wizardOpen.value     = true
}
function closeWizard() { wizardOpen.value = false; justAdded.value = null }

function goToConfig() {
  // Name is optional; defaults to the template name. No validation gate here.
  wizardStep.value = 'config'
}

function goToSplit() {
  wizardStep.value = 'split'
}

async function createDevice() {
  if (wizardStep.value === 'creating') return
  wizardErr.value  = ''
  const name       = customName.value.trim() || defaultName[pickedTemplate.value]
  wizardStep.value = 'creating'

  const p = presetParams()
  // emitCPS: false — I1–I5 are initiator-only junk; large/mimic chains have
  // broken WAN handshakes on CGNAT paths while LAN still worked. H/S/Jc is
  // the stable default against pinned amneziawg-go v0.2.18.
  const cfg = genCfg({
    version: '2.0',
    intensity: p.intensity,
    profile:   p.profile,
    customHost: '', mimicAll: false, useTagC: false,
    useTagT: true, useTagR: true, useTagRC: true, useTagRD: true,
    useBrowserFp: false, browserProfile: '',
    mtu: p.mtu,
    junkLevel: 5, iterCount: 0, routerMode: false,
    useExtremeMax: p.extreme,
    emitCPS: false,
  })
  const snippet = snippetFromCfg(cfg, { includeI: false })

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
      style="background: radial-gradient(ellipse 90% 45% at 50% -5%, rgba(232,160,65,0.05) 0%, transparent 65%)"
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
            <IconButton
              :title="isDark ? 'Светлая тема' : 'Тёмная тема'"
              @click="toggleTheme">
              <Sun v-if="isDark" :size="18" />
              <Moon v-else :size="18" />
            </IconButton>
          </div>

          <div class="text-center">

          <h1 class="text-[56px] sm:text-[64px] font-bold tracking-tight leading-none text-ink-900 mb-5">
            {{ cabinet.name }}
          </h1>

          <!-- Status chips — shared Badge atom for consistent tone/size -->
          <div class="flex items-center justify-center gap-2 flex-wrap">
            <Badge tone="neutral">
              {{ cabinet.devices.length }}
              {{ cabinet.devices.length === 1 ? 'устройство' : cabinet.devices.length < 5 ? 'устройства' : 'устройств' }}
            </Badge>
            <Badge v-if="onlineCount > 0" tone="success">
              <StatusDot state="online" size="sm" />
              {{ onlineCount }} онлайн
            </Badge>
            <Badge v-else-if="cabinet.devices.length > 0" tone="neutral">
              нет подключений
            </Badge>
          </div>
          </div>
        </header>

				<CabinetBillingCard v-if="billing" :billing="billing" @pay="openCheckout" />

        <!-- ── Empty state ── -->
        <div
          v-if="!cabinet.devices.length"
          class="rounded-3xl bg-ink-100/60 dark:bg-ink-200/30 p-10 text-center space-y-7 animate-rise">
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
            class="card device-card animate-rise"
            :class="`delay-${Math.min(i + 1, 6)}`">

            <!-- Card body -->
            <div class="p-4">
              <!-- Top row: icon + name + status -->
              <div class="flex items-start gap-3.5 mb-3.5">
                <!-- Device icon — background matches status: green / amber / gray -->
                <div
                  class="w-11 h-11 rounded-2xl flex items-center justify-center shrink-0"
                  :class="{
                    'bg-success/20':                devStatus(d) === 'online',
                    'bg-warning/15':                devStatus(d) === 'recent',
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
                    <!--
                      Online state gets a chip-shaped backplate (Badge atom) so the
                      live indicator visually anchors itself instead of floating as
                      bare text. Other states stay as plain captions — they don't
                      need to be highlighted.
                    -->
                    <Badge
                      v-if="devStatus(d) === 'online'"
                      tone="success"
                      size="xs"
                      class="shrink-0">
                      <StatusDot state="online" size="sm" />
                      онлайн
                    </Badge>
                    <template v-else>
                      <span class="text-ink-300 text-[9px] select-none">·</span>
                      <span class="text-[11.5px] text-ink-500">
                        {{ relTime(d.latestHandshakeAt) }}
                      </span>
                    </template>
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
                <!-- QR — primary, opens fullscreen carousel -->
                <Button
                  variant="accent"
                  class="flex-1 !text-[12.5px]"
                  :title="`QR-код для ${d.name}`"
                  :aria-label="`Открыть QR-код для ${d.name}`"
                  @click="openQr(d.id)">
                  <QrCode :size="14" />
                  QR-код
                </Button>

                <!-- Copy vpn:// — replaces the old .vpn download button -->
                <button
                  class="flex-1 flex items-center justify-center gap-1.5 h-10 rounded-xl text-[12.5px] font-semibold transition-colors"
                  :class="copiedId === d.id
                    ? 'bg-success/12 text-success'
                    : 'bg-ink-100 dark:bg-ink-200/50 text-ink-700 hover:bg-ink-200 dark:hover:bg-ink-300/50'"
                  :title="copiedId === d.id ? 'Скопировано!' : 'Скопировать ключ'"
                  :aria-label="`Скопировать ключ для ${d.name}`"
                  @click="copyVpn(d.id)">
                  <Check v-if="copiedId === d.id" :size="14" />
                  <Copy  v-else :size="14" />
                  {{ copiedId === d.id ? 'Скопировано' : 'Скопировать' }}
                </button>

                <!-- Overflow menu — .vpn download + delete -->
                <DropdownMenu align="right" width="w-52">
                  <template #trigger="{ open, toggle }">
                    <button
                      class="h-10 w-10 flex items-center justify-center rounded-xl transition-all shrink-0"
                      :class="open
                        ? 'bg-amber-400/15 text-amber-600'
                        : 'text-ink-400 hover:text-ink-700 hover:bg-ink-100 dark:hover:bg-ink-200/50'"
                      :aria-haspopup="true"
                      :aria-expanded="open"
                      :aria-label="`Действия с ${d.name}`"
                      title="Ещё"
                      @click="toggle">
                      <MoreHorizontal :size="16" />
                    </button>
                  </template>
                  <template #default="{ close }">
                    <DropdownItem @click="close()">
                      <a
                        :href="amneziaVpn(d.id)"
                        :download="`${d.name}.vpn`"
                        class="flex items-center gap-2 w-full"
                        @click.stop="close()">
                        <Download :size="15" class="text-ink-500 shrink-0" />
                        Скачать .vpn
                      </a>
                    </DropdownItem>
                    <DropdownSeparator />
                    <DropdownItem tone="danger" @click="deleteFor = d; close()">
                      <Trash2 :size="15" />
                      Удалить устройство
                    </DropdownItem>
                  </template>
                </DropdownMenu>
              </div>
            </div>

          </div>

          <!-- Add more — shared Button atom, secondary tone. -->
          <Button variant="secondary" size="lg" block class="mt-1" @click="openWizard()">
            <Plus :size="18" />
            Добавить устройство
          </Button>
        </section>

        <!--
          Split tunneling — collapsed to a single ghost button that opens
          the full info+download in a modal. The on-page card we used
          before competed with the device list for vertical space and
          read as "another thing to figure out" on first load. Most
          users don't need split-tunnel — those who do tap the link.
        -->
        <Button
          v-if="cabinet.devices.length"
          variant="ghost"
          size="md"
          block
          class="mt-6 animate-rise"
          @click="sitesOpen = true">
          <Globe :size="14" class="text-ink-400" />
          Список заблокированных сайтов
        </Button>

        <p class="text-center text-[11.5px] text-ink-400 dark:text-ink-500 mt-12 leading-relaxed">
          Потеряли ссылку на кабинет?<br>
          Попросите администратора выпустить новую.
        </p>

      </div>
    </template>

		<Modal :open="checkoutOpen" size="sm" title="Оплата хостинга" @close="checkoutOpen = false">
			<div class="space-y-4">
				<div v-if="billing?.latestInvoice" class="rounded-2xl bg-ink-100 p-4 flex items-end justify-between gap-3">
					<div><div class="eyebrow text-ink-500">Ваша доля</div><div class="text-[12px] text-ink-500 mt-1">{{ billing.latestCycle?.title }}</div></div>
					<div class="num-display text-[28px]">{{ new Intl.NumberFormat('ru-RU', { style: 'currency', currency: 'RUB' }).format(billing.latestInvoice.amount / 100) }}</div>
				</div>
				<Field label="Email для чека" hint="ЮKassa отправит электронный чек на этот адрес." :error="checkoutError">
					<Input v-model="checkoutEmail" type="email" autocomplete="email" placeholder="you@example.com" @keydown.enter="startCheckout" />
				</Field>
				<p class="text-[11px] text-ink-500 leading-relaxed">Оплачивая счёт, вы подтверждаете согласие с условиями предоставления доступа к VPN.</p>
			</div>
			<template #footer>
				<Button variant="ghost" size="sm" @click="checkoutOpen = false">Отмена</Button>
				<Button variant="accent" size="sm" :loading="checkoutBusy" @click="startCheckout">Перейти в ЮKassa</Button>
			</template>
		</Modal>

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
                  class="absolute left-2 w-9 h-9 flex items-center justify-center rounded-full bg-white/10 hover:bg-white/20 text-white transition-colors"
                  @click="qrPrev">
                  <ChevronLeft :size="18" />
                </button>
                <button
                  class="absolute right-2 w-9 h-9 flex items-center justify-center rounded-full bg-white/10 hover:bg-white/20 text-white transition-colors"
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
                <IconButton size="sm" title="Закрыть" @click="closeWizard">
                  <X :size="16" />
                </IconButton>
              </div>

              <div>
                <p class="eyebrow mb-3">Тип устройства</p>
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
                <label class="eyebrow block">
                  Название <span class="normal-case tracking-normal font-normal text-ink-400">— необязательно</span>
                </label>
                <Input
                  v-model="customName"
                  :placeholder="defaultName[pickedTemplate]"
                  @keydown.enter="goToConfig"
                />
              </div>

              <p v-if="wizardErr" class="text-[12.5px] text-danger bg-danger/10 rounded-xl px-4 py-3">{{ wizardErr }}</p>

              <Button variant="accent" size="xl" block @click="goToConfig">
                Далее
                <ChevronRight :size="16" />
              </Button>

              <!-- Step indicator — 3 dots -->
              <div class="flex items-center justify-center gap-1.5 pt-1">
                <span class="w-5 h-1 rounded-full bg-amber-400" />
                <span class="w-1 h-1 rounded-full bg-ink-300" />
                <span class="w-1 h-1 rounded-full bg-ink-300" />
              </div>
            </div>

            <!-- ── Step: Config — protection profile ── -->
            <div v-else-if="wizardStep === 'config'" class="p-6 space-y-5">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <h3 class="text-[19px] font-semibold">Профиль защиты</h3>
                  <p class="text-[12.5px] text-ink-500 mt-0.5">Подбираем под вашу сеть</p>
                </div>
                <IconButton size="sm" title="Закрыть" @click="closeWizard">
                  <X :size="16" />
                </IconButton>
              </div>

              <!--
                3 preset cards — full-width rows. Each card is a real
                situation phrased in user-language, not a generator-config
                combo. Avg user keeps "Авто"; power users flip to "Тихий"
                for strict networks or "Быстрый" for low-latency.
              -->
              <div class="space-y-2">
                <button
                  v-for="p in PRESETS" :key="p.v"
                  type="button"
                  role="radio"
                  :aria-checked="pickedPreset === p.v"
                  class="w-full text-left p-3.5 rounded-2xl transition-all duration-150 active:translate-y-px focus-ring relative flex items-start gap-3"
                  :class="pickedPreset === p.v
                    ? 'bg-amber-400/15 shadow-[inset_0_0_0_2px_theme(colors.amber.400)]'
                    : 'bg-ink-100 hover:bg-ink-200 dark:bg-ink-200/40 dark:hover:bg-ink-200/60'"
                  @click="pickedPreset = p.v">
                  <span
                    class="w-9 h-9 rounded-xl flex items-center justify-center shrink-0"
                    :class="pickedPreset === p.v ? 'bg-amber-400/25' : 'bg-ink-50/60 dark:bg-ink-100/40'">
                    <component
                      :is="p.icon"
                      :size="16"
                      :class="pickedPreset === p.v ? 'text-amber-600' : 'text-ink-500'" />
                  </span>
                  <span class="min-w-0 flex-1">
                    <span
                      class="block text-[13.5px] font-semibold leading-tight"
                      :class="pickedPreset === p.v ? 'text-amber-700 dark:text-amber-400' : 'text-ink-900'">
                      {{ p.label }}
                    </span>
                    <span class="block text-[11.5px] text-ink-500 leading-snug mt-0.5">{{ p.hint }}</span>
                  </span>
                  <span
                    class="absolute top-3.5 right-3.5 w-3 h-3 rounded-full transition-all"
                    :class="pickedPreset === p.v
                      ? 'bg-amber-400 ring-[3px] ring-amber-400/25'
                      : 'border border-ink-300 dark:border-ink-400/60'"
                  />
                </button>
              </div>

              <p v-if="wizardErr" class="text-[12.5px] text-danger bg-danger/10 rounded-xl px-4 py-3">{{ wizardErr }}</p>

              <div class="flex items-center gap-2">
                <Button variant="secondary" size="lg" @click="wizardStep = 'pick'">
                  <ChevronLeft :size="16" />
                  Назад
                </Button>
                <Button variant="accent" size="lg" block @click="goToSplit">
                  Далее
                  <ChevronRight :size="16" />
                </Button>
              </div>

              <!-- Step indicator — 3 dots -->
              <div class="flex items-center justify-center gap-1.5 pt-1">
                <span class="w-1 h-1 rounded-full bg-ink-300" />
                <span class="w-5 h-1 rounded-full bg-amber-400" />
                <span class="w-1 h-1 rounded-full bg-ink-300" />
              </div>
            </div>

            <!-- ── Step: Split tunneling ── -->
            <!--
              Step 3 of 3 — shown before key creation so the user downloads
              the IP list while the app is fresh in mind. Most users skipped
              this when it lived as a ghost button at the bottom of the page.
              "Пропустить" is a small link — not a button — so it reads as
              secondary without creating a visual distraction.
            -->
            <div v-else-if="wizardStep === 'split'" class="p-6 space-y-5">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <h3 class="text-[19px] font-semibold">Раздельный туннель</h3>
                  <p class="text-[12.5px] text-ink-500 mt-0.5">Только нужные сайты — через VPN</p>
                </div>
                <IconButton size="sm" title="Закрыть" @click="closeWizard">
                  <X :size="16" />
                </IconButton>
              </div>

              <div class="rounded-2xl bg-ink-100 dark:bg-ink-200/40 p-4 space-y-3">
                <p class="text-[13px] text-ink-700 dark:text-ink-600 leading-relaxed">
                  Скачайте список заблокированных ресурсов и импортируйте в AmneziaVPN —
                  тогда через туннель пойдёт только нужное, а остальное без VPN.
                </p>
                <ol class="space-y-1 text-[12px] text-ink-500 list-decimal list-inside marker:text-ink-300 leading-relaxed">
                  <li>Скачайте файл ниже.</li>
                  <li>В AmneziaVPN: ключ → <strong class="text-ink-700 dark:text-ink-500">Раздельное туннелирование</strong>.</li>
                  <li>Нажмите <span class="mono font-semibold">•••</span> → <strong class="text-ink-700 dark:text-ink-500">Импорт</strong> → выберите файл.</li>
                </ol>
              </div>

              <a
                :href="IPLIST_URL"
                target="_blank"
                rel="noopener"
                class="btn-primary flex items-center justify-center gap-2 h-12 text-[14px] w-full">
                <Download :size="16" />
                Скачать список сайтов
                <ExternalLink :size="13" class="opacity-60" />
              </a>

              <div class="flex items-center gap-2">
                <Button variant="secondary" size="lg" @click="wizardStep = 'config'">
                  <ChevronLeft :size="16" />
                  Назад
                </Button>
                <Button variant="accent" size="lg" block @click="createDevice">
                  Создать ключ
                </Button>
              </div>

              <!-- Step indicator — 3 dots, last active -->
              <div class="flex items-center justify-center gap-1.5 pt-1">
                <span class="w-1 h-1 rounded-full bg-ink-300" />
                <span class="w-1 h-1 rounded-full bg-ink-300" />
                <span class="w-5 h-1 rounded-full bg-amber-400" />
              </div>
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

            <!-- ── Step: Done ──
              Copy is the primary action: one tap, paste in AmneziaVPN.
              QR removed — chunked QRs at 260px are too dense for most
              phone cameras; copy+paste is more reliable.
              Download stays as a fallback for desktop users.
            -->
            <div v-else-if="wizardStep === 'done' && justAdded" class="p-6 space-y-5 animate-fade-in">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0 flex-1">
                  <div class="inline-flex items-center gap-1.5 text-[10.5px] uppercase tracking-[0.14em] font-semibold text-success bg-success/12 px-2 py-1 rounded-full">
                    <Check :size="11" />
                    Ключ готов
                  </div>
                  <h3 class="text-[22px] font-semibold mt-2.5 leading-tight truncate">{{ justAdded.name }}</h3>
                  <p class="text-[11.5px] text-ink-500 mt-0.5 mono">{{ justAdded.address }}</p>
                </div>
                <IconButton size="sm" title="Закрыть" @click="closeWizard">
                  <X :size="16" />
                </IconButton>
              </div>

              <!-- How-to card -->
              <div class="rounded-2xl bg-ink-100/70 dark:bg-ink-200/30 p-4 space-y-1.5">
                <p class="text-[12px] font-semibold text-ink-700 dark:text-ink-500 uppercase tracking-[0.10em]">Как подключиться</p>
                <ol class="space-y-1 text-[12.5px] text-ink-600 dark:text-ink-500 list-decimal list-inside marker:text-ink-400 leading-relaxed">
                  <li>Нажмите <strong class="text-ink-800 dark:text-ink-400">«Скопировать ключ»</strong> ниже.</li>
                  <li>Откройте <strong class="text-ink-800 dark:text-ink-400">AmneziaVPN</strong> → «+» → «Вставить конфигурацию».</li>
                  <li>Или скачайте <span class="mono font-semibold">.vpn</span> и откройте через приложение.</li>
                </ol>
              </div>

              <!-- Primary: Copy — large amber button -->
              <button
                class="w-full h-13 flex items-center justify-center gap-2.5 rounded-2xl text-[15px] font-semibold transition-all active:scale-[0.98]"
                :class="justCopied
                  ? 'bg-success/15 text-success ring-2 ring-success/25'
                  : 'bg-amber-400 text-amber-950 hover:bg-amber-500'"
                @click="copyJustAddedVpn">
                <Check v-if="justCopied" :size="17" />
                <Copy  v-else :size="17" />
                {{ justCopied ? 'Скопировано!' : 'Скопировать ключ' }}
              </button>

              <!-- Secondary row: .vpn download + add more -->
              <div class="flex items-center justify-between gap-2 text-[12px]">
                <a
                  :href="amneziaVpn(justAdded.deviceId)"
                  :download="`${justAdded.name}.vpn`"
                  class="inline-flex items-center gap-1.5 px-2 py-1.5 rounded-lg text-ink-500 hover:text-ink-900 transition-colors">
                  <Download :size="13" />
                  Скачать .vpn
                </a>
                <button
                  class="inline-flex items-center gap-1.5 px-2 py-1.5 rounded-lg text-ink-500 hover:text-ink-900 transition-colors"
                  @click="openWizard()">
                  <Plus :size="13" />
                  Добавить ещё
                </button>
              </div>
            </div>

          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- ─── Delete confirm — shared ConfirmDialog ──────────────────── -->
    <ConfirmDialog
      :open="!!deleteFor"
      title="Удалить устройство?"
      :message="deleteFor ? `«${deleteFor.name}» сразу потеряет подключение. Восстановить нельзя — нужно создать заново.` : ''"
      confirm-text="Удалить"
      tone="danger"
      :loading="deleteBusy"
      @confirm="confirmDelete"
      @cancel="deleteFor = null"
    />

    <!-- ─── Sites list — shared Modal ───────────────────────────────── -->
    <Modal :open="sitesOpen" size="md" @close="sitesOpen = false">
      <template #title>
        <span class="inline-flex items-center gap-2.5">
          <span class="w-8 h-8 rounded-xl bg-amber-400/15 flex items-center justify-center shrink-0">
            <Globe :size="15" class="text-amber-500" />
          </span>
          Заблокированные сайты
        </span>
      </template>

      <div class="space-y-4">
        <p class="text-[12.5px] text-ink-500 leading-relaxed">
          Через туннель пойдут только заблокированные ресурсы. Импорт делается в приложении AmneziaVPN.
        </p>

        <a
          :href="IPLIST_URL"
          target="_blank"
          rel="noopener"
          class="btn-primary flex items-center justify-center gap-2 h-12 text-[14px] w-full">
          <Download :size="16" />
          Скачать список сайтов
          <ExternalLink :size="13" class="opacity-60" />
        </a>

        <div class="rounded-2xl bg-ink-100/60 dark:bg-ink-200/30 p-4">
          <p class="eyebrow mb-3">Как импортировать</p>
          <ol class="space-y-2 text-[12.5px] text-ink-700 dark:text-ink-600 leading-relaxed list-decimal list-inside marker:text-ink-400">
            <li>Откройте <span class="font-semibold">AmneziaVPN</span> и выберите устройство.</li>
            <li>Перейдите в <span class="font-semibold">Раздельное туннелирование</span>.</li>
            <li>Выберите <span class="font-semibold">Раздельное туннелирование сайтов</span>.</li>
            <li>Нажмите <span class="mono font-semibold">•••</span> → <span class="font-semibold">Импорт</span> → <span class="font-semibold">Заменить список сайтами</span>.</li>
            <li>Выберите скачанный файл — готово.</li>
          </ol>
        </div>

        <p class="text-[11px] text-ink-500 leading-relaxed">
          Источник:
          <a
            href="https://github.com/rekryt/iplist"
            target="_blank" rel="noopener"
            class="text-ink-700 dark:text-ink-600 hover:text-ink-900 underline decoration-ink-300 underline-offset-2">iplist</a>
          — открытый реестр, обновляется регулярно. Если что-то не уходит в туннель, просто перекачайте файл и повторите импорт.
        </p>
      </div>
    </Modal>

  </div>
</template>
