<script setup lang="ts">
/*
  Главная — дашборд админа.

  Двухуровневая модель:
    Subscriber (клиент) → имеет много Device (устройств)
  Каждое устройство = свой awgN-интерфейс. Subscriber раскрывается inline,
  показывая список своих устройств с админ-действиями.
*/

import { ref, computed } from 'vue'
import { useClientsStore } from '@/stores/clients'
import { useStatsStore } from '@/stores/stats'
import { useSubscribersStore } from '@/stores/subscribers'
import { useToastStore } from '@/stores/toasts'
import { useInterval } from '@/composables/useInterval'
import { bytes, bytesParts, handshakeFreshness } from '@/lib/format'

import TopBar from '@/components/organisms/TopBar.vue'
import MetricCard from '@/components/molecules/MetricCard.vue'
import Sparkline from '@/components/molecules/Sparkline.vue'
import SubscriberModal from '@/components/organisms/SubscriberModal.vue'
import QrModal from '@/components/organisms/QrModal.vue'
import ConfigModal from '@/components/organisms/ConfigModal.vue'
import ConfirmDialog from '@/components/molecules/ConfirmDialog.vue'
import EmptyState from '@/components/molecules/EmptyState.vue'
import Button from '@/components/atoms/Button.vue'
import Spinner from '@/components/atoms/Spinner.vue'
import Icon from '@/components/atoms/Icon.vue'
import type { Subscriber, Client } from '@/types'

const clients = useClientsStore()
const stats   = useStatsStore()
const subs    = useSubscribersStore()
const toasts  = useToastStore()

useInterval(() => clients.fetch(true), 3000, { immediate: true, pauseHidden: true })
useInterval(() => stats.fetch(),       5000, { immediate: true, pauseHidden: true })
useInterval(() => subs.fetch(true),    5000, { immediate: true, pauseHidden: true })

// ─── Производные ───
const onlineNow = computed(() =>
  clients.items.filter(c => c.enabled && handshakeFreshness(c.latestHandshakeAt) === 'online').length
)

const liveRate = computed(() => {
  const o = stats.overview
  if (!o || !o.windowSeconds) return 0
  return (o.rxLast + o.txLast) / o.windowSeconds
})

const topTalkers = computed(() => {
  const top = stats.overview?.top ?? []
  return top.map(t => ({ ...t, device: clients.byId(t.clientId) })).filter(t => t.device)
})

const todayDate = computed(() =>
  new Date().toLocaleDateString('ru-RU', { weekday: 'long', day: 'numeric', month: 'long' }).toUpperCase()
)

// Per-subscriber device lookup (joined client-side)
function devicesOf(sub: Subscriber): Client[] {
  return clients.items.filter(c => c.subscriberId === sub.id)
}

// ─── Modals & confirms ───
const subModalOpen = ref(false)
const subModalBusy = ref(false)
const createdSub = ref<Subscriber | null>(null)

const expanded = ref<Set<string>>(new Set())
function toggleExpand(id: string) {
  if (expanded.value.has(id)) expanded.value.delete(id)
  else expanded.value.add(id)
  // Trigger reactivity (Set mutation isn't reactive on its own)
  expanded.value = new Set(expanded.value)
}

const qrFor   = ref<string | null>(null)
const cfgFor  = ref<string | null>(null)
const devDelFor = ref<{ id: string; name: string } | null>(null)
const subDelFor = ref<Subscriber | null>(null)
const regenFor = ref<Subscriber | null>(null)

const nameOf = (id: string | null) => id ? clients.items.find(c => c.id === id)?.name : undefined

function openCreate() {
  createdSub.value = null
  subModalOpen.value = true
}

async function onSubSubmit(body: { name: string; notes?: string }) {
  subModalBusy.value = true
  try {
    createdSub.value = await subs.create(body)
    try { await navigator.clipboard.writeText(createdSub.value.url) } catch { /* ignore */ }
  } catch { /* toast in store */ }
  finally { subModalBusy.value = false }
}

function closeSubModal() {
  subModalOpen.value = false
  createdSub.value = null
}

async function copyCabinetUrl(url: string) {
  try { await navigator.clipboard.writeText(url); toasts.success('Ссылка скопирована') }
  catch { toasts.error('Не удалось скопировать') }
}

async function doDeleteDevice() {
  if (!devDelFor.value) return
  await clients.remove(devDelFor.value.id)
  devDelFor.value = null
  await subs.fetch(true)
}

async function doDeleteSub() {
  if (!subDelFor.value) return
  await subs.remove(subDelFor.value.id)
  subDelFor.value = null
  await clients.fetch(true)
}

async function doRegen() {
  if (!regenFor.value) return
  await subs.regenerateToken(regenFor.value.id)
  regenFor.value = null
}

function relTime(s?: string | null) {
  if (!s) return 'никогда'
  try {
    const diff = Date.now() - new Date(s).getTime()
    if (diff < 60_000) return 'только что'
    if (diff < 3600_000) return Math.floor(diff / 60_000) + ' мин'
    if (diff < 86400_000) return Math.floor(diff / 3600_000) + ' ч'
    return Math.floor(diff / 86400_000) + ' д'
  } catch { return s }
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

    <main class="max-w-5xl mx-auto px-4 sm:px-6 pt-10 pb-16 space-y-12">
      <header class="space-y-3 animate-rise">
        <div class="flex items-baseline justify-between gap-4 flex-wrap">
          <div class="eyebrow tnum">{{ todayDate }}</div>
          <div class="eyebrow">Обновление · 3 с</div>
        </div>
        <h1 class="num-display text-ink-900 text-[40px] sm:text-[52px]">Обзор</h1>
        <p class="text-[13.5px] text-ink-500 max-w-md leading-relaxed">
          Состояние сервера и клиентов. Живые показатели обновляются каждые 3 секунды.
        </p>
      </header>

      <!-- Hero metrics -->
      <section class="grid grid-cols-1 sm:grid-cols-12 gap-8 sm:gap-6">
        <div class="sm:col-span-5 animate-rise delay-1">
          <MetricCard eyebrow="Сегодня · Входящий" kind="bytes" size="hero" :value="stats.overview?.rxToday || 0" />
        </div>
        <div class="sm:col-span-4 animate-rise delay-2">
          <MetricCard eyebrow="Сегодня · Исходящий" kind="bytes" size="normal" :value="stats.overview?.txToday || 0" />
        </div>
        <div class="sm:col-span-3 animate-rise delay-3 sm:border-l border-ink-900/10 sm:pl-6">
          <MetricCard
            eyebrow="Онлайн"
            kind="ratio"
            size="normal"
            :numerator="onlineNow"
            :denominator="clients.items.length"
            :sub="liveRate > 1024 ? bytes(liveRate) + '/с сейчас' : 'нет активности'"
            :sub-tone="liveRate > 1024 ? 'success' : undefined"
          />
        </div>
      </section>

      <!-- 24h sparkline -->
      <section class="space-y-3 animate-rise delay-4">
        <div class="flex items-center gap-4">
          <div class="eyebrow">Трафик за 24 часа</div>
          <div class="hairline flex-1" />
          <div class="text-[11px] text-ink-500 mono">
            <span class="text-ink-900">▍</span> входящий
            <span class="ml-3 text-ink-400">▍</span> исходящий
          </div>
        </div>
        <div class="card p-5 sm:p-7">
          <Sparkline v-if="stats.series24h && stats.series24h.points.length" :points="stats.series24h.points" :height="140" />
          <div v-else class="h-[140px] grid place-items-center text-[12px] text-ink-500">
            <span v-if="stats.loaded">За последние 24 часа трафика не было.</span>
            <Spinner v-else :size="18" />
          </div>
        </div>
      </section>

      <!-- Top talkers -->
      <section v-if="topTalkers.length" class="space-y-3 animate-rise delay-5">
        <div class="flex items-center gap-4">
          <div class="eyebrow">Самые активные · 24 часа</div>
          <div class="hairline flex-1" />
        </div>
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <router-link
            v-for="(t, i) in topTalkers" :key="t.clientId"
            :to="{ name: 'client', params: { id: t.clientId } }"
            class="card p-5 hover:bg-ink-100/40 transition-colors block"
          >
            <div class="flex items-baseline gap-2">
              <span class="eyebrow tnum">№{{ i + 1 }}</span>
              <span class="eyebrow text-ink-300">·</span>
              <span class="mono text-[10.5px] text-ink-500">{{ t.device?.address }}</span>
            </div>
            <div class="mt-2 text-[20px] text-ink-900 font-semibold truncate tracking-tight">
              {{ t.device?.subscriberName || '—' }}
              <span class="text-ink-500 font-normal text-[14px]"> · {{ t.device?.name }}</span>
            </div>
            <div class="mt-3 flex items-baseline gap-3 tnum">
              <span class="num-display-soft text-[24px] text-ink-900">{{ bytesParts(t.rx + t.tx).value }}</span>
              <span class="text-[11px] text-ink-500 mono uppercase">{{ bytesParts(t.rx + t.tx).unit }}</span>
              <span class="text-[11px] text-ink-500 ml-auto mono">↓ {{ bytes(t.rx) }} · ↑ {{ bytes(t.tx) }}</span>
            </div>
          </router-link>
        </div>
      </section>

      <!-- Subscribers list -->
      <section class="space-y-6">
        <div class="flex items-baseline gap-4 flex-wrap">
          <h2 class="eyebrow">Клиенты · {{ subs.items.length }}</h2>
          <div class="hairline flex-1" />
        </div>

        <div v-if="subs.loading && !subs.items.length" class="card p-10 grid place-items-center">
          <Spinner :size="22" />
        </div>

        <EmptyState
          v-else-if="!subs.items.length"
          title="Пока нет ни одного клиента"
          description="Создайте первого — выдадите ему ссылку на личный кабинет, где он сам подключит свои устройства."
        >
          <template #action>
            <Button variant="primary" size="sm" @click="openCreate">
              <Icon name="plus" :size="15" /> Новый клиент
            </Button>
          </template>
        </EmptyState>

        <div v-else class="card divide-y divide-ink-900/5">
          <template v-for="s in subs.items" :key="s.id">
            <div class="px-5 py-4">
              <div class="flex items-center gap-3 flex-wrap cursor-pointer" @click="toggleExpand(s.id)">
                <button class="text-ink-500 hover:text-ink-900 -ml-1" :title="expanded.has(s.id) ? 'Свернуть' : 'Развернуть'">
                  <Icon :name="expanded.has(s.id) ? 'chevron-down' : 'chevron-right'" :size="14" />
                </button>
                <div class="flex-1 min-w-0">
                  <div class="flex items-baseline gap-2 flex-wrap">
                    <span class="text-[15px] text-ink-900 font-semibold">{{ s.name }}</span>
                    <span class="text-[11px] text-ink-500 mono">{{ s.deviceCount }} {{ s.deviceCount === 1 ? 'устр.' : 'устр-в' }}</span>
                  </div>
                  <div v-if="s.notes" class="text-[11.5px] text-ink-500 mt-0.5">{{ s.notes }}</div>
                </div>
                <div class="flex items-center gap-1.5" @click.stop>
                  <Button size="sm" variant="ghost" @click="copyCabinetUrl(s.url)">
                    <Icon name="copy" :size="13" /> Ссылка
                  </Button>
                  <Button size="sm" variant="ghost" @click="regenFor = s">
                    <Icon name="refresh" :size="13" /> Обновить
                  </Button>
                  <Button size="sm" variant="ghost" @click="subDelFor = s">
                    <Icon name="trash" :size="13" /> Удалить
                  </Button>
                </div>
              </div>

              <!-- Expanded devices -->
              <div v-if="expanded.has(s.id)" class="mt-3 ml-6 space-y-2">
                <div v-if="!devicesOf(s).length" class="text-[12px] text-ink-500 px-2 py-3">
                  У клиента пока нет устройств. Когда он откроет кабинет и добавит устройство — оно появится здесь.
                </div>
                <div v-else class="space-y-1.5">
                  <div v-for="d in devicesOf(s)" :key="d.id"
                       class="flex items-center gap-2 flex-wrap px-3 py-2 rounded-lg bg-ink-100/30">
                    <router-link :to="{ name: 'client', params: { id: d.id } }" class="flex-1 min-w-0 hover:underline">
                      <div class="flex items-baseline gap-2 flex-wrap">
                        <span class="text-[13px] text-ink-900 font-medium">{{ d.name }}</span>
                        <span v-if="!d.enabled" class="text-[10px] uppercase tracking-[0.12em] text-danger px-1.5 py-0.5 rounded bg-danger/10">disabled</span>
                      </div>
                      <div class="text-[10.5px] text-ink-500 mono">{{ d.address }} · последнее: {{ relTime(d.latestHandshakeAt) }}</div>
                    </router-link>
                    <div class="flex items-center gap-1">
                      <Button size="sm" variant="ghost" @click="cfgFor = d.id">
                        <Icon name="settings" :size="13" />
                      </Button>
                      <Button size="sm" variant="ghost" @click="qrFor = d.id">
                        <Icon name="settings" :size="13" /> QR
                      </Button>
                      <Button size="sm" variant="ghost" @click="devDelFor = { id: d.id, name: d.name }">
                        <Icon name="trash" :size="13" />
                      </Button>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </template>
        </div>
      </section>

      <footer class="pt-6 eyebrow flex flex-wrap items-center gap-3">
        <span>AmneziaWG · Панель</span>
        <span class="text-ink-300">·</span>
        <span class="mono normal-case tracking-normal text-ink-500">{{ onlineNow }} онлайн из {{ clients.items.length }}</span>
      </footer>
    </main>

    <SubscriberModal
      :open="subModalOpen"
      :busy="subModalBusy"
      :created="createdSub"
      @close="closeSubModal"
      @submit="onSubSubmit"
    />

    <QrModal     :open="!!qrFor"  :client-id="qrFor"  :client-name="nameOf(qrFor)"  @close="qrFor = null" />
    <ConfigModal :open="!!cfgFor" :client-id="cfgFor" :client-name="nameOf(cfgFor)" @close="cfgFor = null" />

    <ConfirmDialog
      :open="devDelFor !== null"
      title="Удалить устройство?"
      :message="`«${devDelFor?.name ?? ''}» сразу потеряет подключение и интерфейс будет снят. Восстановить нельзя.`"
      confirm-text="Удалить"
      tone="danger"
      @cancel="devDelFor = null"
      @confirm="doDeleteDevice"
    />

    <ConfirmDialog
      :open="subDelFor !== null"
      title="Удалить клиента?"
      :message="`«${subDelFor?.name ?? ''}» и ВСЕ его устройства будут удалены. Интерфейсы остановятся, ссылка кабинета перестанет работать. Необратимо.`"
      confirm-text="Удалить аккаунт"
      tone="danger"
      @cancel="subDelFor = null"
      @confirm="doDeleteSub"
    />

    <ConfirmDialog
      :open="regenFor !== null"
      title="Обновить ссылку кабинета?"
      :message="`Старая ссылка «${regenFor?.name ?? ''}» сразу перестанет работать — клиенту нужно будет передать новую. Устройства и подключения сохранятся.`"
      confirm-text="Обновить"
      tone="neutral"
      @cancel="regenFor = null"
      @confirm="doRegen"
    />
  </div>
</template>
