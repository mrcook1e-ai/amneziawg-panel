<script setup lang="ts">
import { ref, computed } from 'vue'
import { useClientsStore } from '@/stores/clients'
import { useStatsStore } from '@/stores/stats'
import { useSubscribersStore } from '@/stores/subscribers'
import { useToastStore } from '@/stores/toasts'
import { useInterval } from '@/composables/useInterval'
import { bytes, handshakeFreshness, relativeTime } from '@/lib/format'

import TopBar from '@/components/organisms/TopBar.vue'
import MetricCard from '@/components/molecules/MetricCard.vue'
import SubscriberModal from '@/components/organisms/SubscriberModal.vue'
import ConfirmDialog from '@/components/molecules/ConfirmDialog.vue'
import EmptyState from '@/components/molecules/EmptyState.vue'
import Button from '@/components/atoms/Button.vue'
import Spinner from '@/components/atoms/Spinner.vue'
import Icon from '@/components/atoms/Icon.vue'
import type { Subscriber } from '@/types'

const clients = useClientsStore()
const stats   = useStatsStore()
const subs    = useSubscribersStore()
const toasts  = useToastStore()

useInterval(() => clients.fetch(true), 3000, { immediate: true, pauseHidden: true })
useInterval(() => stats.fetch(),       5000, { immediate: true, pauseHidden: true })
useInterval(() => subs.fetch(true),    5000, { immediate: true, pauseHidden: true })

// ── Per-subscriber live metrics ────────────────────────────────────────
function onlineOf(sub: Subscriber): number {
  return clients.items.filter(
    c => c.subscriberId === sub.id && handshakeFreshness(c.latestHandshakeAt) === 'online'
  ).length
}

function deviceCountOf(sub: Subscriber): number {
  const live = clients.items.filter(c => c.subscriberId === sub.id).length
  return live || sub.deviceCount || 0
}

function lastSeenOf(sub: Subscriber): string | null {
  const times = clients.items
    .filter(c => c.subscriberId === sub.id && c.latestHandshakeAt)
    .map(c => new Date(c.latestHandshakeAt!).getTime())
  return times.length ? new Date(Math.max(...times)).toISOString() : null
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
            <template v-if="subs.items.length"> · {{ subs.items.length }}</template>
          </h2>
          <div class="hairline flex-1" />
          <Button variant="ghost" size="sm" @click="openCreate">
            <Icon name="plus" :size="13" /> Добавить
          </Button>
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

        <!-- List -->
        <div v-else class="card divide-y divide-ink-900/5 overflow-hidden">
          <router-link
            v-for="s in subs.items"
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

            <!-- Hover actions -->
            <div class="flex items-center gap-1 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
              <button
                class="h-7 w-7 flex items-center justify-center rounded-lg hover:bg-ink-200/70 transition-colors"
                title="Скопировать ссылку кабинета"
                @click.prevent="copyCabinetUrl(s.url)">
                <Icon name="copy" :size="13" class="text-ink-500" />
              </button>
              <button
                class="h-7 w-7 flex items-center justify-center rounded-lg hover:bg-ink-200/70 transition-colors"
                title="Обновить ссылку"
                @click.prevent="regenFor = s">
                <Icon name="refresh" :size="13" class="text-ink-500" />
              </button>
              <button
                class="h-7 w-7 flex items-center justify-center rounded-lg hover:bg-danger/10 transition-colors"
                title="Удалить клиента"
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
            · ↓ {{ bytes(stats.overview!.rxToday) }} · ↑ {{ bytes(stats.overview!.txToday) }}
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
