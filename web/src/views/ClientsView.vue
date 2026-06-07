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

// ── Per-subscriber live metrics ────────────────────────────────────────────
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

// ── Total online ────────────────────────────────────────────────────────────
const onlineNow = computed(() =>
  clients.items.filter(c => c.enabled && handshakeFreshness(c.latestHandshakeAt) === 'online').length
)

// ── Create subscriber ───────────────────────────────────────────────────────
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

// ── Subscriber actions ──────────────────────────────────────────────────────
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

      <!-- Header -->
      <header class="space-y-1 animate-rise">
        <h1 class="num-display text-ink-900 text-[40px] sm:text-[52px]">Клиенты</h1>
        <p class="text-[13.5px] text-ink-500">
          {{ subs.items.length }} аккаунт{{ subs.items.length === 1 ? '' : subs.items.length < 5 ? 'а' : 'ов' }}
          · {{ onlineNow }} устройств онлайн
        </p>
      </header>

      <!-- Compact stats -->
      <section class="grid grid-cols-1 sm:grid-cols-3 gap-4 animate-rise delay-1">
        <MetricCard
          eyebrow="Онлайн"
          kind="ratio"
          size="normal"
          :numerator="onlineNow"
          :denominator="clients.items.length"
        />
        <MetricCard eyebrow="Входящий · сегодня" kind="bytes" size="normal" :value="stats.overview?.rxToday || 0" />
        <MetricCard eyebrow="Исходящий · сегодня" kind="bytes" size="normal" :value="stats.overview?.txToday || 0" />
      </section>

      <!-- Subscribers -->
      <section class="space-y-4 animate-rise delay-2">
        <div class="flex items-center gap-4">
          <h2 class="eyebrow">Все клиенты · {{ subs.items.length }}</h2>
          <div class="hairline flex-1" />
          <Button variant="ghost" size="sm" @click="openCreate">
            <Icon name="plus" :size="13" /> Добавить
          </Button>
        </div>

        <!-- Loading -->
        <div v-if="subs.loading && !subs.items.length" class="card p-12 grid place-items-center">
          <Spinner :size="22" />
        </div>

        <!-- Empty -->
        <EmptyState
          v-else-if="!subs.items.length"
          title="Клиентов пока нет"
          description="Создайте первого — получите ссылку кабинета и передайте клиенту. Он сам добавит свои устройства."
        >
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
            class="px-5 py-4 flex items-center gap-4 hover:bg-ink-100/40 transition-colors group"
          >
            <!-- Status dot -->
            <span
              class="w-2 h-2 rounded-full shrink-0 mt-px"
              :class="onlineOf(s) > 0 ? 'bg-success animate-pulse' : 'bg-ink-300'"
            />

            <!-- Info -->
            <div class="flex-1 min-w-0">
              <div class="flex items-baseline gap-2 flex-wrap">
                <span class="text-[15px] font-semibold text-ink-900">{{ s.name }}</span>
                <span class="text-[11px] text-ink-500 mono">
                  {{ deviceCountOf(s) }} устр.
                  <template v-if="onlineOf(s) > 0">
                    · <span class="text-success">{{ onlineOf(s) }} онлайн</span>
                  </template>
                </span>
              </div>
              <div class="text-[11.5px] text-ink-400 mt-0.5">
                <template v-if="s.notes">{{ s.notes }} · </template>
                {{ lastSeenOf(s) ? relativeTime(lastSeenOf(s)) : 'не подключались' }}
              </div>
            </div>

            <!-- Cabinet link copy (stops propagation from router-link) -->
            <button
              class="opacity-0 group-hover:opacity-100 transition-opacity"
              title="Скопировать ссылку кабинета"
              @click.prevent="copyCabinetUrl(s.url)"
            >
              <Icon name="copy" :size="15" class="text-ink-500 hover:text-ink-900" />
            </button>

            <!-- Regen (stops propagation) -->
            <button
              class="opacity-0 group-hover:opacity-100 transition-opacity"
              title="Обновить ссылку"
              @click.prevent="regenFor = s"
            >
              <Icon name="refresh" :size="15" class="text-ink-500 hover:text-ink-900" />
            </button>

            <!-- Delete (stops propagation) -->
            <button
              class="opacity-0 group-hover:opacity-100 transition-opacity"
              title="Удалить клиента"
              @click.prevent="subDelFor = s"
            >
              <Icon name="trash" :size="15" class="text-ink-400 hover:text-danger transition-colors" />
            </button>

            <!-- Chevron -->
            <Icon name="chevron-right" :size="14" class="text-ink-300 group-hover:text-ink-600 transition-colors shrink-0" />
          </router-link>
        </div>
      </section>

      <footer class="eyebrow flex flex-wrap items-center gap-3 pt-4">
        <span>AmneziaWG · Панель</span>
        <span class="text-ink-300">·</span>
        <span class="mono normal-case tracking-normal text-ink-500">
          {{ onlineNow }} / {{ clients.items.length }} онлайн
          <template v-if="stats.overview?.rxToday">· ↓ {{ bytes(stats.overview.rxToday) }} сегодня</template>
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
