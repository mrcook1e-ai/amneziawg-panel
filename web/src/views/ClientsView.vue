<script setup lang="ts">
/*
  Главная — дашборд.

  Структура:
    1. TopBar (живые цифры + онлайн-счётчик).
    2. PulseStrip — 1px пульсация по реальному throughput.
    3. Заголовок страницы (дата + h1).
    4. Hero-метрики: входящий / исходящий / онлайн-соотношение.
    5. 24-часовой график.
    6. Топ-клиенты за 24ч.
    7. Список клиентов.
*/

import { ref, computed } from 'vue'
import { useClientsStore } from '@/stores/clients'
import { useStatsStore } from '@/stores/stats'
import { useInterval } from '@/composables/useInterval'
import { bytes, bytesParts, handshakeFreshness } from '@/lib/format'

import TopBar from '@/components/organisms/TopBar.vue'
import MetricCard from '@/components/molecules/MetricCard.vue'
import Sparkline from '@/components/molecules/Sparkline.vue'
import ClientList from '@/components/organisms/ClientList.vue'
import NewClientModal from '@/components/organisms/NewClientModal.vue'
import QrModal from '@/components/organisms/QrModal.vue'
import ConfigModal from '@/components/organisms/ConfigModal.vue'
import ConfirmDialog from '@/components/molecules/ConfirmDialog.vue'
import EmptyState from '@/components/molecules/EmptyState.vue'
import Button from '@/components/atoms/Button.vue'
import Input from '@/components/atoms/Input.vue'
import Spinner from '@/components/atoms/Spinner.vue'
import Icon from '@/components/atoms/Icon.vue'

const clients = useClientsStore()
const stats   = useStatsStore()

useInterval(() => clients.fetch(true), 3000, { immediate: true, pauseHidden: true })
useInterval(() => stats.fetch(),       5000, { immediate: true, pauseHidden: true })

// ─── Производные ───
const query = ref('')
const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return clients.items
  return clients.items.filter(c =>
    c.name.toLowerCase().includes(q) ||
    c.address.includes(q) ||
    c.publicKey.toLowerCase().includes(q) ||
    (c.notes?.toLowerCase().includes(q) ?? false),
  )
})

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
  return top.map(t => ({ ...t, client: clients.byId(t.clientId) })).filter(t => t.client)
})

const todayDate = computed(() =>
  new Date().toLocaleDateString('ru-RU', {
    weekday: 'long', day: 'numeric', month: 'long',
  }).toUpperCase()
)

// ─── Модалки ───
const newOpen = ref(false)
const newBusy = ref(false)
const qrFor   = ref<string | null>(null)
const cfgFor  = ref<string | null>(null)
const delFor  = ref<string | null>(null)
const nameOf = (id: string | null) => id ? clients.items.find(c => c.id === id)?.name : undefined

async function onCreate(name: string) {
  newBusy.value = true
  try { await clients.create(name); newOpen.value = false }
  finally { newBusy.value = false }
}
async function confirmDelete() {
  if (!delFor.value) return
  await clients.remove(delFor.value)
  delFor.value = null
}
</script>

<template>
  <div class="min-h-full">
    <TopBar>
      <template #actions>
        <Button variant="secondary" size="sm" @click="newOpen = true">
          <Icon name="plus" :size="15" />
          <span class="hidden sm:inline">Новый клиент</span>
        </Button>
      </template>
    </TopBar>

    <main class="max-w-5xl mx-auto px-4 sm:px-6 pt-10 pb-16 space-y-12">
      <!-- Заголовок -->
      <header class="space-y-3 animate-rise">
        <div class="flex items-baseline justify-between gap-4 flex-wrap">
          <div class="eyebrow tnum">{{ todayDate }}</div>
          <div class="eyebrow">Обновление · 3 с</div>
        </div>
        <h1 class="num-display text-ink-900 text-[40px] sm:text-[52px]">
          Обзор
        </h1>
        <p class="text-[13.5px] text-ink-500 max-w-md leading-relaxed">
          Состояние сервера и клиентов. Живые показатели обновляются каждые 3 секунды.
        </p>
      </header>

      <!-- Hero-метрики — асимметрия 5/4/3 -->
      <section class="grid grid-cols-1 sm:grid-cols-12 gap-8 sm:gap-6">
        <div class="sm:col-span-5 animate-rise delay-1">
          <MetricCard
            eyebrow="Сегодня · Входящий"
            kind="bytes"
            size="hero"
            :value="stats.overview?.rxToday || 0"
          />
        </div>
        <div class="sm:col-span-4 animate-rise delay-2">
          <MetricCard
            eyebrow="Сегодня · Исходящий"
            kind="bytes"
            size="normal"
            :value="stats.overview?.txToday || 0"
          />
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
          <Sparkline
            v-if="stats.series24h && stats.series24h.points.length"
            :points="stats.series24h.points"
            :height="140"
          />
          <div v-else class="h-[140px] grid place-items-center text-[12px] text-ink-500">
            <span v-if="stats.loaded">За последние 24 часа трафика не было.</span>
            <Spinner v-else :size="18" />
          </div>
        </div>
      </section>

      <!-- Топ-клиенты -->
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
              <span class="mono text-[10.5px] text-ink-500">{{ t.client?.address }}</span>
            </div>
            <div class="mt-2 text-[20px] text-ink-900 font-semibold truncate tracking-tight">
              {{ t.client?.name }}
            </div>
            <div class="mt-3 flex items-baseline gap-3 tnum">
              <span class="num-display-soft text-[24px] text-ink-900">{{ bytesParts(t.rx + t.tx).value }}</span>
              <span class="text-[11px] text-ink-500 mono uppercase">{{ bytesParts(t.rx + t.tx).unit }}</span>
              <span class="text-[11px] text-ink-500 ml-auto mono">↓ {{ bytes(t.rx) }} · ↑ {{ bytes(t.tx) }}</span>
            </div>
          </router-link>
        </div>
      </section>

      <!-- Список клиентов -->
      <section class="space-y-6">
        <div class="flex items-baseline gap-4 flex-wrap">
          <h2 class="eyebrow">Клиенты</h2>
          <div class="hairline flex-1" />
          <div v-if="clients.items.length" class="w-full sm:w-72">
            <Input v-model="query" size="sm" placeholder="Поиск по имени, IP, ключу или описанию" />
          </div>
        </div>

        <div v-if="clients.loading && !clients.items.length" class="card p-10 grid place-items-center">
          <Spinner :size="22" />
        </div>

        <EmptyState
          v-else-if="!clients.items.length"
          title="Пока нет ни одного клиента"
          description="Добавьте первого клиента, чтобы получить конфиг и QR-код."
        >
          <template #action>
            <Button variant="primary" size="sm" @click="newOpen = true">
              <Icon name="plus" :size="15" /> Новый клиент
            </Button>
          </template>
        </EmptyState>

        <EmptyState
          v-else-if="!filtered.length"
          title="Ничего не найдено"
          :description="`По запросу «${query}» ничего нет.`"
        />

        <ClientList
          v-else
          :clients="filtered"
          @toggle="(id, v) => clients.setEnabled(id, v)"
          @remove="id => delFor = id"
          @show-config="id => cfgFor = id"
          @show-qr="id => qrFor = id"
        />
      </section>

      <footer class="pt-6 eyebrow flex flex-wrap items-center gap-3">
        <span>AmneziaWG · Панель</span>
        <span class="text-ink-300">·</span>
        <span class="mono normal-case tracking-normal text-ink-500">{{ onlineNow }} онлайн из {{ clients.items.length }}</span>
      </footer>
    </main>

    <NewClientModal :open="newOpen" :busy="newBusy" @close="newOpen = false" @submit="onCreate" />
    <QrModal     :open="!!qrFor"  :client-id="qrFor"  :client-name="nameOf(qrFor)"  @close="qrFor = null" />
    <ConfigModal :open="!!cfgFor" :client-id="cfgFor" :client-name="nameOf(cfgFor)" @close="cfgFor = null" />
    <ConfirmDialog
      :open="!!delFor"
      title="Удалить клиента?"
      :message="`Доступ для «${nameOf(delFor)}» сразу отзовётся. Восстановить нельзя.`"
      confirm-text="Удалить"
      tone="danger"
      @cancel="delFor = null"
      @confirm="confirmDelete"
    />
  </div>
</template>
