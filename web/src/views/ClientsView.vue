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
import SubscriberModal from '@/components/organisms/SubscriberModal.vue'
import ConfirmDialog from '@/components/molecules/ConfirmDialog.vue'
import EmptyState from '@/components/molecules/EmptyState.vue'
import Input from '@/components/atoms/Input.vue'
import Button from '@/components/atoms/Button.vue'
import Spinner from '@/components/atoms/Spinner.vue'
import Icon from '@/components/atoms/Icon.vue'
import { ArrowDown, ArrowUp } from 'lucide-vue-next'
import type { Subscriber } from '@/types'

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
interface SubMetrics { online: number; devices: number; lastSeenMs: number }
const metricsBySub = computed<Map<string, SubMetrics>>(() => {
  const m = new Map<string, SubMetrics>()
  for (const c of clients.items) {
    if (!c.subscriberId) continue
    const cur = m.get(c.subscriberId) ?? { online: 0, devices: 0, lastSeenMs: 0 }
    cur.devices += 1
    if (handshakeFreshness(c.latestHandshakeAt) === 'online') cur.online += 1
    if (c.latestHandshakeAt) {
      const ts = new Date(c.latestHandshakeAt).getTime()
      if (ts > cur.lastSeenMs) cur.lastSeenMs = ts
    }
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

// ── Totals ─────────────────────────────────────────────────────────────
const onlineNow = computed(() =>
  clients.items.filter(c => c.enabled && handshakeFreshness(c.latestHandshakeAt) === 'online').length
)

const hasActivity = computed(() =>
  (stats.overview?.rxToday ?? 0) > 0 || (stats.overview?.txToday ?? 0) > 0
)

// ── Create subscriber ───────────────────────────────────────────────────
const subModalOpen = ref(false)
const subModalBusy = ref(false)
const createdSub   = ref<Subscriber | null>(null)

function openCreate() { createdSub.value = null; subModalOpen.value = true }

async function onSubSubmit(body: { name: string; notes?: string }) {
  subModalBusy.value = true
  try {
    createdSub.value = await subs.create(body)
    try { await navigator.clipboard.writeText(createdSub.value.url) } catch { /* ignore */ }
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

// ── Search ───────────────────────────────────────────────────────────────
const search = ref('')
const filteredSubs = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return subs.items
  return subs.items.filter(s =>
    s.name.toLowerCase().includes(q) ||
    (s.notes ?? '').toLowerCase().includes(q),
  )
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
      <section class="grid grid-cols-1 sm:grid-cols-3 gap-4 animate-rise delay-1">
        <MetricCard
          eyebrow="Устройств онлайн"
          kind="ratio"
          size="normal"
          :numerator="onlineNow"
          :denominator="clients.items.length"
        />
        <MetricCard
          eyebrow="Входящий · сегодня"
          kind="bytes"
          size="normal"
          :value="stats.overview?.rxToday || 0"
        />
        <MetricCard
          eyebrow="Исходящий · сегодня"
          kind="bytes"
          size="normal"
          :value="stats.overview?.txToday || 0"
        />
      </section>

      <!-- ── Subscriber list ── -->
      <section class="space-y-4 animate-rise delay-2">
        <div class="flex items-center gap-4">
          <h2 class="eyebrow">
            Все клиенты
            <template v-if="subs.items.length"> · {{ filteredSubs.length }}<span v-if="search">/{{ subs.items.length }}</span></template>
          </h2>
          <div class="hairline flex-1" />
          <Button variant="ghost" size="sm" @click="openCreate">
            <Icon name="plus" :size="13" /> Добавить
          </Button>
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

            <!-- Online pulse / dot -->
            <div class="shrink-0 flex items-center">
              <template v-if="onlineOf(s) > 0">
                <span class="live-dot" />
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
                <span class="text-[11.5px] text-ink-500 mono">
                  {{ deviceCountOf(s) }} устр.
                  <template v-if="onlineOf(s) > 0">
                    · <span class="text-success font-medium">{{ onlineOf(s) }} онлайн</span>
                  </template>
                </span>
              </div>
              <div class="text-[11.5px] text-ink-400 mt-0.5 truncate">
                <template v-if="s.notes">{{ s.notes }} · </template>
                {{ lastSeenOf(s) ? relativeTime(lastSeenOf(s)) : 'не подключались' }}
              </div>
            </div>

            <!-- Hover actions — visible on touch, fade-on-hover on pointer devices -->
            <div class="flex items-center gap-1 shrink-0 sm:opacity-0 sm:group-hover:opacity-100 sm:focus-within:opacity-100 transition-opacity">
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
          <template v-if="hasActivity">
            · <ArrowDown :size="10" class="inline-block align-middle" /> {{ bytes(stats.overview!.rxToday) }} · <ArrowUp :size="10" class="inline-block align-middle" /> {{ bytes(stats.overview!.txToday) }}
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
