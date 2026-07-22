<script setup lang="ts">
import { ref, computed } from 'vue'
import { useClientsStore } from '@/stores/clients'
import { useStatsStore } from '@/stores/stats'
import { useSubscribersStore } from '@/stores/subscribers'
import { useToastStore } from '@/stores/toasts'
import { useInterval } from '@/composables/useInterval'
import { useTitle } from '@/composables/useTitle'
import { bytes, handshakeFreshness, relativeTime } from '@/lib/format'

import TopBar from '@/components/organisms/TopBar.vue'
import MetricCard from '@/components/molecules/MetricCard.vue'
import Segmented from '@/components/atoms/Segmented.vue'
import SubscriberModal from '@/components/organisms/SubscriberModal.vue'
import ConfirmDialog from '@/components/molecules/ConfirmDialog.vue'
import EmptyState from '@/components/molecules/EmptyState.vue'
import Input from '@/components/atoms/Input.vue'
import Button from '@/components/atoms/Button.vue'
import Spinner from '@/components/atoms/Spinner.vue'
import Icon from '@/components/atoms/Icon.vue'
import Badge from '@/components/atoms/Badge.vue'
import { ArrowDown, ArrowUp } from 'lucide-vue-next'
import type { BillingRole, Subscriber } from '@/types'

const clients = useClientsStore()
const stats   = useStatsStore()
const subs    = useSubscribersStore()
const toasts  = useToastStore()

useTitle(() => 'Клиенты · Amnezia Panel')

// SSE (lib/stream.ts) refetches `clients` immediately on any client.* audit
// event (create/delete/rename/enable/disable/expired/reset). Polling here is
// only needed to refresh handshake timestamps (not streamed) — 10s is enough.
// `subs` has no SSE coverage; `stats.overview` aggregates aren't in the tick.
useInterval(() => clients.fetch(true), 10000, { immediate: true, pauseHidden: true })
useInterval(() => stats.fetch(),        5000, { immediate: true, pauseHidden: true })
useInterval(() => subs.fetch(true),     5000, { immediate: true, pauseHidden: true })

// ── Per-subscriber live metrics ────────────────────────────────────────
// Build one Map<subscriberId, {online, devices, lastSeen}> per data update,
// so list rendering is O(N+M) instead of O(N*M) per render. Matters at scale:
// 100 clients × 20 subs × 3 functions × render-on-every-poll was ~6k ops/tick.
// Traffic field is the lifetime rx+tx across all the subscriber's peers.
// We prefer total{Rx,Tx} (persistent counters that survive rekey) and
// fall back to transfer{Rx,Tx} (running counters since last rekey) when
// the long-form totals aren't available — typical for fresh peers.
interface SubMetrics { online: number; devices: number; lastSeenMs: number; traffic: number }
const metricsBySub = computed<Map<string, SubMetrics>>(() => {
  const m = new Map<string, SubMetrics>()
  for (const c of clients.items) {
    if (!c.subscriberId) continue
    const cur = m.get(c.subscriberId) ?? { online: 0, devices: 0, lastSeenMs: 0, traffic: 0 }
    cur.devices += 1
    if (handshakeFreshness(c.latestHandshakeAt) === 'online') cur.online += 1
    if (c.latestHandshakeAt) {
      const ts = new Date(c.latestHandshakeAt).getTime()
      if (ts > cur.lastSeenMs) cur.lastSeenMs = ts
    }
    const rx = c.totalRx ?? c.transferRx ?? 0
    const tx = c.totalTx ?? c.transferTx ?? 0
    cur.traffic += rx + tx
    m.set(c.subscriberId, cur)
  }
  return m
})

function onlineOf(sub: Subscriber): number {
  return metricsBySub.value.get(sub.id)?.online ?? 0
}
function deviceCountOf(sub: Subscriber): number {
  return metricsBySub.value.get(sub.id)?.devices || sub.deviceCount || 0
}
function lastSeenOf(sub: Subscriber): string | null {
  const ts = metricsBySub.value.get(sub.id)?.lastSeenMs ?? 0
  return ts ? new Date(ts).toISOString() : null
}
function trafficOf(sub: Subscriber): number {
  return metricsBySub.value.get(sub.id)?.traffic ?? 0
}

// ── Totals ─────────────────────────────────────────────────────────────
const onlineNow = computed(() =>
  clients.items.filter(c => c.enabled && handshakeFreshness(c.latestHandshakeAt) === 'online').length
)

const hasActivity = computed(() =>
  (stats.overview?.rxToday ?? 0) > 0 || (stats.overview?.txToday ?? 0) > 0
)

// ── Period selector ─────────────────────────────────────────────────────
type Period = 'today' | '7d' | '30d' | 'total'
const period = ref<Period>('today')

const periods: { value: Period; label: string }[] = [
  { value: 'today', label: 'Сегодня' },
  { value: '7d',    label: '7 дней' },
  { value: '30d',   label: '30 дней' },
  { value: 'total', label: 'Всё время' },
]

const rxForPeriod = computed(() => {
  const o = stats.overview
  if (!o) return 0
  if (period.value === 'today') return o.rxToday
  if (period.value === '7d')    return o.rx7d
  if (period.value === '30d')   return o.rx30d
  return o.rxTotal
})

const txForPeriod = computed(() => {
  const o = stats.overview
  if (!o) return 0
  if (period.value === 'today') return o.txToday
  if (period.value === '7d')    return o.tx7d
  if (period.value === '30d')   return o.tx30d
  return o.txTotal
})

const periodLabel = computed(() => periods.find(p => p.value === period.value)?.label ?? '')

// ── Create subscriber ───────────────────────────────────────────────────
const subModalOpen = ref(false)
const subModalBusy = ref(false)
const createdSub   = ref<Subscriber | null>(null)

function openCreate() { createdSub.value = null; subModalOpen.value = true }

async function onSubSubmit(body: { name: string; notes?: string; billingRole: BillingRole }) {
  subModalBusy.value = true
  try {
    createdSub.value = await subs.create(body)
    try {
      await navigator.clipboard.writeText(createdSub.value.url)
      toasts.success('Ссылка скопирована')
    } catch {
      toasts.info('Скопируйте ссылку вручную — буфер обмена недоступен')
    }
  } catch { /* toast in store */ }
  finally { subModalBusy.value = false }
}

function closeSubModal() { subModalOpen.value = false; createdSub.value = null }

// ── Subscriber actions ──────────────────────────────────────────────────
const subDelFor = ref<Subscriber | null>(null)
const regenFor  = ref<Subscriber | null>(null)

async function copyCabinetUrl(url: string) {
  try { await navigator.clipboard.writeText(url); toasts.success('Ссылка скопирована') }
  catch { toasts.error('Не удалось скопировать') }
}

// ── Search + "empty-only" filter ──────────────────────────────────────
// "Empty" = subscriber with zero peers yet. Flagged separately because
// after server migrations or first onboarding these are the rows the
// admin needs to chase (cabinet link → user → import .vpn).
const search = ref('')
const onlyEmpty = ref(false)
function isEmpty(s: Subscriber): boolean {
  return deviceCountOf(s) === 0
}
const emptyCount = computed(() => subs.items.filter(isEmpty).length)
const filteredSubs = computed(() => {
  const q = search.value.trim().toLowerCase()
  let list = subs.items
  if (onlyEmpty.value) list = list.filter(isEmpty)
  if (q) {
    list = list.filter(s =>
      s.name.toLowerCase().includes(q) ||
      (s.notes ?? '').toLowerCase().includes(q),
    )
  }
  // Within the visible list, put empties first — they're what the admin
  // needs to act on. Stable sort keeps everything else in incoming order.
  return [...list].sort((a, b) => Number(isEmpty(b)) - Number(isEmpty(a)))
})

async function doDeleteSub() {
  if (!subDelFor.value) return
  await subs.remove(subDelFor.value.id)
  subDelFor.value = null
}

async function doRegen() {
  if (!regenFor.value) return
  await subs.regenerateToken(regenFor.value.id)
  regenFor.value = null
}
</script>

<template>
  <div class="min-h-full">
    <TopBar>
      <template #actions>
        <Button variant="secondary" size="sm" @click="openCreate">
          <Icon name="plus" :size="15" />
          <span class="hidden sm:inline">Новый клиент</span>
        </Button>
      </template>
    </TopBar>

    <main class="max-w-5xl mx-auto px-4 sm:px-6 pt-10 pb-16 space-y-10">

      <!-- ── Page header ── -->
      <header class="space-y-2 animate-rise">
        <p class="eyebrow">Клиенты</p>
        <h1 class="num-display text-ink-900 text-[44px] sm:text-[56px]">
          {{ subs.items.length }}&thinsp;<span class="text-ink-300">аккаунт{{ subs.items.length === 1 ? '' : subs.items.length < 5 ? 'а' : 'ов' }}</span>
        </h1>
        <p class="text-[13.5px] text-ink-500 flex items-center gap-2">
          <template v-if="onlineNow > 0">
            <span class="live-dot" />
            <span class="text-success font-medium">{{ onlineNow }} онлайн</span>
            <span class="text-ink-300">·</span>
          </template>
          {{ clients.items.length }} устройств всего
        </p>
      </header>

      <!-- ── Stats strip ── -->
      <section class="space-y-6 animate-rise delay-1">
        <!-- Period selector — uses the Segmented atom -->
        <Segmented v-model="period" :options="periods" />

        <!-- Metrics — clean grid, no card backgrounds -->
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-8 sm:gap-6">
          <MetricCard
            eyebrow="Устройств онлайн"
            kind="ratio"
            size="normal"
            :numerator="onlineNow"
            :denominator="clients.items.length"
          />
          <MetricCard
            :eyebrow="`↓ Входящий · ${periodLabel}`"
            kind="bytes"
            size="normal"
            :value="rxForPeriod"
          />
          <MetricCard
            :eyebrow="`↑ Исходящий · ${periodLabel}`"
            kind="bytes"
            size="normal"
            :value="txForPeriod"
          />
        </div>
      </section>

      <!-- ── Subscriber list ── -->
      <section class="space-y-4 animate-rise delay-2">
        <div class="flex items-center gap-3 flex-wrap">
          <h2 class="eyebrow">
            Все клиенты
            <template v-if="subs.items.length"> · {{ filteredSubs.length }}<span v-if="search || onlyEmpty">/{{ subs.items.length }}</span></template>
          </h2>

          <!--
            Filter chip — "X без устройств". Toggle-pressed binds to
            onlyEmpty so the click acts like a chip-style filter.
            Hidden when there's nothing to chase (emptyCount === 0).
          -->
          <button
            v-if="emptyCount > 0"
            type="button"
            :aria-pressed="onlyEmpty"
            class="inline-flex items-center gap-1.5 h-6 px-2 rounded-full text-[11px] font-medium tracking-wide transition-colors"
            :class="onlyEmpty
              ? 'bg-warning text-white'
              : 'bg-warning/12 text-warning hover:bg-warning/20'"
            :title="onlyEmpty ? 'Показать всех' : 'Показать только без устройств'"
            @click="onlyEmpty = !onlyEmpty">
            <Icon name="alert-triangle" :size="11" />
            {{ emptyCount }} без устройств
          </button>

          <div class="hairline flex-1" />
        </div>

        <!-- Search — visible only when there's something worth filtering -->
        <div v-if="subs.items.length > 5" class="relative">
          <Input
            v-model="search"
            size="sm"
            placeholder="Поиск по имени или заметке…"
            aria-label="Поиск клиентов"
          />
          <button
            v-if="search"
            type="button"
            class="absolute right-2 top-1/2 -translate-y-1/2 h-7 w-7 flex items-center justify-center rounded-lg text-ink-400 hover:text-ink-900 hover:bg-ink-200 transition-colors"
            title="Очистить"
            aria-label="Очистить поиск"
            @click="search = ''"
          >
            <Icon name="x" :size="14" />
          </button>
        </div>

        <!-- Loading skeleton -->
        <div v-if="subs.loading && !subs.items.length" class="card p-12 grid place-items-center">
          <Spinner :size="22" />
        </div>

        <!-- Empty -->
        <EmptyState
          v-else-if="!subs.items.length"
          title="Клиентов пока нет"
          description="Создайте первого — получите ссылку кабинета и передайте клиенту. Он самостоятельно добавит свои устройства.">
          <template #action>
            <Button variant="primary" size="sm" @click="openCreate">
              <Icon name="plus" :size="15" /> Новый клиент
            </Button>
          </template>
        </EmptyState>

        <!-- No matches in active search -->
        <div v-else-if="!filteredSubs.length" class="card p-8 text-center text-[12.5px] text-ink-500">
          Ничего не нашлось по «{{ search }}»
        </div>

        <!-- List -->
        <div v-else class="card divide-y divide-ink-900/5 overflow-hidden">
          <router-link
            v-for="s in filteredSubs"
            :key="s.id"
            :to="{ name: 'subscriber', params: { id: s.id } }"
            class="px-5 py-4 flex items-center gap-4 transition-colors group"
            :class="onlineOf(s) > 0 ? 'hover:bg-success/4' : 'hover:bg-ink-100/40'">

            <!--
              Status dot — three states:
                · online (≥1 peer with fresh handshake) → live-dot
                · empty  (0 peers — no device added yet) → amber warning dot
                · idle   (peers exist but none online)  → ink-200 mute
            -->
            <div class="shrink-0 flex items-center">
              <template v-if="onlineOf(s) > 0">
                <span class="live-dot" />
              </template>
              <template v-else-if="isEmpty(s)">
                <span class="w-2.5 h-2.5 rounded-full bg-warning block" />
              </template>
              <template v-else>
                <span class="w-2.5 h-2.5 rounded-full bg-ink-200 block" />
              </template>
            </div>

            <!-- Info -->
            <div class="flex-1 min-w-0">
              <div class="flex items-baseline gap-2 flex-wrap">
                <span class="text-[15px] font-semibold text-ink-900 group-hover:text-ink-950 transition-colors">
                  {{ s.name }}
                </span>
                <!--
                  Empty-state badge replaces the "X устр." caption for
                  subscribers with zero peers. Calls out the row that
                  needs action (cabinet link sent? user imported config?)
                -->
                <Badge v-if="isEmpty(s)" tone="warning" size="xs">
                  <Icon name="alert-triangle" :size="10" />
                  нет устройств
                </Badge>
                <span v-else class="text-[11.5px] text-ink-500 mono">
                  {{ deviceCountOf(s) }} устр.
                  <template v-if="onlineOf(s) > 0">
                    · <span class="text-success font-medium">{{ onlineOf(s) }} онлайн</span>
                  </template>
                </span>
								<Badge v-if="s.billingRole === 'payer'" tone="warning" size="xs">плательщик</Badge>
								<Badge v-else-if="s.billingRole === 'owner'" tone="success" size="xs">владелец</Badge>
              </div>
              <div class="text-[11.5px] text-ink-400 mt-0.5 truncate">
                <template v-if="s.notes">{{ s.notes }} · </template>
                <template v-if="isEmpty(s)">кабинет ещё не использован</template>
                <template v-else>{{ lastSeenOf(s) ? relativeTime(lastSeenOf(s)) : 'не подключались' }}</template>
              </div>
            </div>

            <!--
              Lifetime traffic for this subscriber — sum of all peers'
              total{Rx,Tx} (falling back to transfer{Rx,Tx}). Tabular
              numerals so the column aligns vertically across rows.
              Hidden on phones to keep the row tight; reappears at sm+.
            -->
            <div
              v-if="trafficOf(s) > 0"
              class="flex flex-col items-end shrink-0 mr-1"
              :title="`Всего трафика: ${bytes(trafficOf(s))}`">
              <span class="hidden sm:block text-[10.5px] uppercase tracking-[0.12em] text-ink-400 font-medium leading-tight">Трафик</span>
              <span class="text-[12.5px] mono tnum text-ink-700 dark:text-ink-600 leading-tight sm:mt-0.5">{{ bytes(trafficOf(s)) }}</span>
            </div>

            <!-- Hover actions — visible on touch, fade-on-hover on pointer devices -->
            <div class="flex items-center gap-1 shrink-0">
              <button
                type="button"
                class="h-8 w-8 flex items-center justify-center rounded-lg hover:bg-ink-200/70 focus-ring transition-colors"
                :title="`Скопировать ссылку кабинета ${s.name}`"
                :aria-label="`Скопировать ссылку кабинета ${s.name}`"
                @click.prevent="copyCabinetUrl(s.url)">
                <Icon name="copy" :size="13" class="text-ink-500" />
              </button>
              <button
                type="button"
                class="h-8 w-8 flex items-center justify-center rounded-lg hover:bg-ink-200/70 focus-ring transition-colors"
                :title="`Обновить ссылку ${s.name}`"
                :aria-label="`Обновить ссылку кабинета ${s.name}`"
                @click.prevent="regenFor = s">
                <Icon name="refresh" :size="13" class="text-ink-500" />
              </button>
              <button
                type="button"
                class="h-8 w-8 flex items-center justify-center rounded-lg hover:bg-danger/10 focus-ring transition-colors"
                :title="`Удалить ${s.name}`"
                :aria-label="`Удалить клиента ${s.name}`"
                @click.prevent="subDelFor = s">
                <Icon name="trash" :size="13" class="text-ink-400 hover:text-danger transition-colors" />
              </button>
            </div>

            <Icon name="chevron-right" :size="14" class="text-ink-300 group-hover:text-ink-500 transition-colors shrink-0" />
          </router-link>
        </div>
      </section>

      <!-- Footer -->
      <footer class="eyebrow flex flex-wrap items-center gap-3 pt-2 border-t border-ink-900/5">
        <span>AmneziaWG · Панель</span>
        <span class="text-ink-300">·</span>
        <span class="mono normal-case tracking-normal text-ink-500">
          {{ onlineNow }} / {{ clients.items.length }} онлайн
          <template v-if="rxForPeriod > 0 || txForPeriod > 0">
            · <ArrowDown :size="10" class="inline-block align-middle" /> {{ bytes(rxForPeriod) }} · <ArrowUp :size="10" class="inline-block align-middle" /> {{ bytes(txForPeriod) }}
          </template>
        </span>
      </footer>

    </main>

    <!-- Modals -->
    <SubscriberModal
      :open="subModalOpen"
      :busy="subModalBusy"
      :created="createdSub"
      @close="closeSubModal"
      @submit="onSubSubmit"
    />

    <ConfirmDialog
      :open="subDelFor !== null"
      title="Удалить клиента?"
      :message="`«${subDelFor?.name ?? ''}» и ВСЕ его устройства будут удалены. Интерфейсы остановятся, ссылка перестанет работать. Необратимо.`"
      confirm-text="Удалить аккаунт"
      tone="danger"
      @cancel="subDelFor = null"
      @confirm="doDeleteSub"
    />

    <ConfirmDialog
      :open="regenFor !== null"
      title="Обновить ссылку кабинета?"
      :message="`Старая ссылка «${regenFor?.name ?? ''}» сразу перестанет работать — клиенту нужно передать новую. Устройства сохранятся.`"
      confirm-text="Обновить"
      tone="neutral"
      @cancel="regenFor = null"
      @confirm="doRegen"
    />
  </div>
</template>
