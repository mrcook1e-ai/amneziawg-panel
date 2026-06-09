<script setup lang="ts">
/*
  Страница клиента.

  Содержит: идентификация, телеметрия, 24-часовой график, редактируемые
  поля (заметка, срок действия, DNS / AllowedIPs / MTU override), журнал
  событий, опасная зона.
*/

import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/lib/api'
import { useClientsStore } from '@/stores/clients'
import { useProfilesStore } from '@/stores/profiles'
import { useInterval } from '@/composables/useInterval'
import { useTitle } from '@/composables/useTitle'
import { useToastStore } from '@/stores/toasts'
import { ArrowDown, ArrowUp } from 'lucide-vue-next'
import { bytes, relativeTime, handshakeFreshness, stateLabelRu } from '@/lib/format'
import type { ClientStats, AppEvent } from '@/types'

import TopBar from '@/components/organisms/TopBar.vue'
import Sparkline from '@/components/molecules/Sparkline.vue'
import Section from '@/components/molecules/Section.vue'
import InfoRow from '@/components/molecules/InfoRow.vue'
import CopyButton from '@/components/molecules/CopyButton.vue'
import EventRow from '@/components/molecules/EventRow.vue'
import ConfirmDialog from '@/components/molecules/ConfirmDialog.vue'
import StatBlock from '@/components/molecules/StatBlock.vue'
import DownloadActions from '@/components/molecules/DownloadActions.vue'
import Button from '@/components/atoms/Button.vue'
import Switch from '@/components/atoms/Switch.vue'
import StatusDot from '@/components/atoms/StatusDot.vue'
import Spinner from '@/components/atoms/Spinner.vue'
import Skeleton from '@/components/atoms/Skeleton.vue'
import Icon from '@/components/atoms/Icon.vue'

const route    = useRoute()
const router   = useRouter()
const clients  = useClientsStore()
const profiles = useProfilesStore()
const toasts   = useToastStore()

const id = computed(() => route.params.id as string)
const client = computed(() => clients.byId(id.value))

useTitle(() => client.value ? `${client.value.name} · Amnezia Panel` : 'Клиент · Amnezia Panel')

const cs       = ref<ClientStats | null>(null)
const events   = ref<AppEvent[]>([])
const loading  = ref(true)

async function loadAll() {
  const myId = id.value
  if (!myId) return
  loading.value = true
  try {
    const [s, ev] = await Promise.all([
      api.clientStats(myId),
      api.clientEvents(myId, 20),
    ])
    // Guard against stale writes — if the user navigated to a different
    // client mid-flight, drop the late response on the floor.
    if (id.value !== myId) return
    cs.value = s
    events.value = ev ?? []
  } catch (e: any) {
    if (id.value !== myId) return
    toasts.error(e?.message || 'Ошибка загрузки')
  } finally {
    if (id.value === myId) loading.value = false
  }
}
onMounted(async () => {
  if (!clients.items.length) await clients.fetch()
  if (!profiles.items.length) profiles.fetch(true)
  await loadAll()
})

// SSE handles audit events for this client; poll only for handshake freshness.
useInterval(() => clients.fetch(true), 15000, { pauseHidden: true })
useInterval(loadAll, 8000, { pauseHidden: true })

const live = computed(() => {
  if (!cs.value) return 0
  return (cs.value.rxLast + cs.value.txLast) / Math.max(1, cs.value.windowSeconds)
})

// Peak per-bucket throughput across the 24h window — useful as a y-axis
// surrogate for the otherwise unlabeled Sparkline. We use total bytes per
// bucket; the bucket length lives in cs.windowSeconds (per-bucket window).
const peakBucket = computed(() => {
  const s = cs.value?.series
  if (!s?.length) return 0
  let max = 0
  for (const p of s) {
    const v = (p.rx || 0) + (p.tx || 0)
    if (v > max) max = v
  }
  return max
})
const avgBucket = computed(() => {
  const s = cs.value?.series
  if (!s?.length) return 0
  let total = 0
  for (const p of s) total += (p.rx || 0) + (p.tx || 0)
  return total / s.length
})

const expiringSoon = computed(() => {
  const c = client.value
  if (!c?.expiresAt) return false
  const days = (new Date(c.expiresAt).getTime() - Date.now()) / 86400000
  return days < 7 && days > 0
})

const stateKind = computed<'online' | 'stale' | 'offline' | 'disabled'>(() => {
  const c = client.value
  if (!c) return 'offline'
  if (!c.enabled) return 'disabled'
  return handshakeFreshness(c.latestHandshakeAt)
})

const stateText = computed(() => stateLabelRu(stateKind.value))

const stateAccent = computed(() => ({
  online:   'text-success',
  stale:    'text-warning',
  offline:  'text-ink-500',
  disabled: 'text-ink-500',
}[stateKind.value]))

const stateDot = computed(() => ({
  online:   'bg-success',
  stale:    'bg-warning',
  offline:  'bg-ink-300',
  disabled: 'bg-ink-300',
}[stateKind.value]))

// ─── Действия ───
const renaming = ref(false)
const renameDraft = ref('')
function startRename() {
  if (!client.value) return
  renameDraft.value = client.value.name
  renaming.value = true
}
async function commitRename() {
  if (!client.value) return
  const v = renameDraft.value.trim()
  renaming.value = false
  if (!v || v === client.value.name) return
  await clients.rename(client.value.id, v)
}

const delOpen = ref(false)

// Profile this peer is attached to. Useful for "Идентификация" — admins
// need to see where the peer lives (interface / port / endpoint host).
const profile = computed(() => {
  const c = client.value
  return c ? profiles.byId(c.profileId) : null
})

// Exact handshake timestamp for "Идентификация" — relative time lives in
// the header. Admin debugging wants the actual wall-clock value.
function fmtHandshakeFull(s: string | null | undefined): string {
  if (!s) return 'никогда'
  try { return new Date(s).toLocaleString('ru-RU', { dateStyle: 'medium', timeStyle: 'medium' }) }
  catch { return s }
}

async function toggleEnabled(v: boolean) {
  if (!client.value) return
  await clients.setEnabled(client.value.id, v)
}

async function confirmDelete() {
  if (!client.value) return
  await clients.remove(client.value.id)
  delOpen.value = false
  router.push({ name: 'clients' })
}
</script>

<template>
  <div class="min-h-full">
    <TopBar />

    <main class="max-w-5xl mx-auto px-4 sm:px-6 pt-10 pb-16 space-y-12">

      <!-- Назад + заголовок -->
      <header class="space-y-4">
        <!-- Breadcrumb: device belongs to a subscriber -->
        <div class="flex items-center gap-1.5 eyebrow flex-wrap">
          <router-link :to="{ name: 'clients' }" class="hover:text-ink-900 transition-colors">
            Клиенты
          </router-link>
          <template v-if="client?.subscriberName && client?.subscriberId">
            <Icon name="chevron-right" :size="12" class="text-ink-300" />
            <router-link
              :to="{ name: 'subscriber', params: { id: client.subscriberId } }"
              class="hover:text-ink-900 transition-colors"
            >
              {{ client.subscriberName }}
            </router-link>
          </template>
          <Icon name="chevron-right" :size="12" class="text-ink-300" />
          <span class="text-ink-900">{{ client?.name ?? '…' }}</span>
        </div>

        <div v-if="client" class="space-y-3">
          <div class="eyebrow tnum flex items-center gap-2 flex-wrap">
            <span>ID · {{ client.id.slice(0, 8) }}</span>
            <span class="text-ink-300">·</span>
            <span class="inline-flex items-center gap-1.5" :class="stateAccent">
              <StatusDot
                :state="stateKind === 'online' ? 'online' : stateKind === 'stale' ? 'stale' : 'offline'"
                size="sm"
              />
              {{ stateText }}
            </span>
          </div>

          <div v-if="!renaming" class="flex items-baseline gap-3 flex-wrap">
            <h1 class="num-display text-[40px] sm:text-[56px] text-ink-900 animate-rise">
              {{ client.name }}
            </h1>
            <button class="eyebrow text-ink-400 hover:text-ink-900 transition-colors" @click="startRename">
              <Icon name="edit" :size="13" /> Переименовать
            </button>
          </div>
          <input
            v-else
            v-model="renameDraft"
            @keydown.enter="commitRename"
            @keydown.escape="renaming = false"
            @blur="commitRename"
            class="num-display text-[40px] sm:text-[56px] bg-transparent outline-none border-b-2 border-ink-900 text-ink-900 w-full"
            autofocus
          />

          <div class="mt-1 flex items-center gap-3 text-[12.5px] text-ink-500 flex-wrap">
            <span class="mono">{{ client.address }}</span>
            <span>·</span>
            <span>был {{ relativeTime(client.latestHandshakeAt) }}</span>
            <span
              v-if="expiringSoon"
              class="ml-2 inline-flex items-center gap-1.5 text-[10.5px] uppercase tracking-[0.12em] font-medium text-warning bg-warning/10 border border-warning/30 px-2 py-0.5 rounded-full"
            >
              <span class="inline-block h-1.5 w-1.5 rounded-full bg-warning" />
              срок скоро истекает
            </span>
          </div>
        </div>

        <div v-else class="space-y-3">
          <Skeleton width="180" height="12" />
          <Skeleton width="60%" height="48" rounded="lg" />
          <Skeleton width="240" height="12" />
        </div>
      </header>

      <!-- Skeleton телеметрии пока клиент грузится. Держит layout, чтобы
           страница не «дёргалась» при появлении данных. -->
      <section v-if="!client" class="grid grid-cols-2 sm:grid-cols-4 gap-6 sm:gap-4">
        <div v-for="i in 4" :key="i" class="space-y-2">
          <Skeleton width="80" height="10" />
          <Skeleton width="70%" height="36" rounded="lg" />
        </div>
      </section>

      <template v-if="client">
        <!-- Телеметрия -->
        <section class="space-y-4 animate-rise delay-1">
          <!-- Входящий -->
          <div class="rounded-xl border-l-[3px] border border-success/25 border-l-success bg-success/5 px-5 py-4 space-y-3">
            <div class="eyebrow text-success flex items-center gap-1.5 font-semibold">
              <ArrowDown :size="11" />Входящий
            </div>
            <div class="grid grid-cols-3 gap-6 sm:gap-4">
              <StatBlock eyebrow="За 5 минут" :value="cs?.rxLast || 0" />
              <StatBlock eyebrow="За 24 часа" :value="cs?.rx24h || 0" />
              <StatBlock eyebrow="За 7 дней"  :value="cs?.rx7d || 0" />
            </div>
          </div>
          <!-- Исходящий -->
          <div class="rounded-xl border-l-[3px] border border-amber-200 border-l-amber-400 bg-amber-50/70 px-5 py-4 space-y-3">
            <div class="eyebrow text-amber-500 flex items-center gap-1.5 font-semibold">
              <ArrowUp :size="11" />Исходящий
            </div>
            <div class="grid grid-cols-3 gap-6 sm:gap-4">
              <StatBlock eyebrow="За 5 минут" :value="cs?.txLast || 0" />
              <StatBlock eyebrow="За 24 часа" :value="cs?.tx24h || 0" />
              <StatBlock eyebrow="За 7 дней"  :value="cs?.tx7d || 0" />
            </div>
          </div>
          <!-- Online ratio — progress bar -->
          <div class="space-y-2 pt-1 border-t border-ink-900/5">
            <div class="eyebrow text-ink-500">Доступен · 7 дн</div>
            <div class="flex items-center gap-4">
              <span class="num-display-soft tnum text-ink-900 text-[34px] sm:text-[40px] leading-none">
                {{ Math.round((cs?.onlineRatio7d || 0) * 100) }}<span class="mono text-[10.5px] text-ink-500 uppercase tracking-wider ml-1">%</span>
              </span>
              <div class="flex-1 h-1.5 bg-ink-200 rounded-full overflow-hidden">
                <div
                  class="h-full bg-success rounded-full transition-all duration-700"
                  :style="{ width: Math.round((cs?.onlineRatio7d || 0) * 100) + '%' }"
                />
              </div>
            </div>
          </div>
        </section>

        <!-- График 24ч -->
        <section class="space-y-3 animate-rise delay-2">
          <div class="flex items-center gap-4">
            <div class="eyebrow">Трафик за 24 часа</div>
            <div class="hairline flex-1" />
            <div class="text-[11px] mono tnum flex items-center gap-1.5">
              <StatusDot v-if="live > 1024" state="online" size="sm" />
              <span :class="live > 1024 ? 'text-success' : 'text-ink-500'">
                {{ live > 1024 ? bytes(live) + '/с' : 'нет активности' }}
              </span>
            </div>
          </div>
          <div class="card p-5 sm:p-7 space-y-3">
            <Sparkline v-if="cs && cs.series.length" :points="cs.series" :height="140" />
            <Skeleton v-else-if="loading" height="140" rounded="lg" />
            <div v-else class="h-[140px] grid place-items-center text-[12px] text-ink-500">
              <span>Трафика за последние 24 часа не было.</span>
            </div>
            <!--
              Sparkline lacks a y-axis intentionally (saves vertical space).
              These eyebrows give the magnitudes — peak / average — without
              cluttering the canvas.
            -->
            <div
              v-if="cs && cs.series.length"
              class="flex items-center gap-4 text-[10.5px] uppercase tracking-[0.12em] text-ink-500 font-medium">
              <span>пик · <span class="text-ink-700 mono tnum normal-case tracking-normal">{{ bytes(peakBucket) }}</span></span>
              <span>средн. · <span class="text-ink-700 mono tnum normal-case tracking-normal">{{ bytes(avgBucket) }}</span></span>
              <span class="ml-auto">за интервал ~{{ Math.round((cs.windowSeconds || 60) / 60) }} мин</span>
            </div>
          </div>
        </section>

        <!--
          Идентификация — split into "Сетевые параметры" (where + how)
          and "Криптография" (who) so admins scanning the page can find
          the bit they need without reading every row.
        -->
        <Section title="Сетевые параметры" footer="Что устройство видит как сервер. Сменить можно только через перевыпуск конфига.">
          <InfoRow v-if="client.subscriberName && client.subscriberId" label="Подписка" show-divider>
            <router-link
              :to="{ name: 'subscriber', params: { id: client.subscriberId } }"
              class="text-ink-900 hover:underline decoration-ink-300 underline-offset-2"
            >{{ client.subscriberName }}</router-link>
          </InfoRow>
          <InfoRow v-if="profile" label="Профиль" show-divider>
            <span class="text-ink-900">{{ profile.name }}</span>
            <span class="mono text-ink-500"> · {{ profile.iface }}:{{ profile.port }}</span>
          </InfoRow>
          <InfoRow label="IP-адрес" :value="client.address" mono show-divider>
            <CopyButton :value="client.address" />
          </InfoRow>
          <InfoRow v-if="profile?.endpoint" label="Endpoint" :value="profile.endpoint" mono show-divider>
            <CopyButton :value="profile.endpoint" />
          </InfoRow>
          <InfoRow label="Keepalive">
            <span class="mono text-ink-700">{{ client.persistentKeepalive || '—' }}</span>
          </InfoRow>
        </Section>

        <Section title="Криптография" footer="Публичный ключ — то, что сервер проверяет при каждом подключении.">
          <InfoRow label="Публичный ключ" :value="client.publicKey" mono show-divider>
            <CopyButton :value="client.publicKey" />
          </InfoRow>
          <InfoRow label="Последний handshake" show-divider>
            <span class="mono text-ink-700">{{ fmtHandshakeFull(client.latestHandshakeAt) }}</span>
          </InfoRow>
          <InfoRow label="Добавлен">
            <span class="mono text-ink-700">{{ new Date(client.createdAt).toLocaleString('ru-RU') }}</span>
          </InfoRow>
        </Section>

        <!--
          Доступ — single column. .vpn is the primary delivery channel
          (AmneziaWG-native, carries obfuscation); QR inlines under the
          download row instead of opening a modal; .conf is hidden under
          a <details> as a debug-only artifact.
        -->
        <Section title="Доступ" footer=".vpn — формат AmneziaVPN-приложения. Откройте QR или скопируйте vpn:// для импорта в один клик.">
          <InfoRow label="Включён" show-divider>
            <Switch :model-value="client.enabled" @update:model-value="toggleEnabled" />
          </InfoRow>
          <div class="px-4 py-4">
            <DownloadActions :client-id="client.id" :client-name="client.name" />
          </div>
        </Section>

        <!-- История -->
        <section v-if="events.length" class="space-y-3">
          <div class="flex items-center gap-4">
            <div class="eyebrow">История</div>
            <div class="hairline flex-1" />
          </div>
          <div class="card overflow-hidden">
            <template v-for="(e, i) in events" :key="e.id">
              <EventRow :event="e" />
              <div v-if="i < events.length - 1" class="hairline mx-4" />
            </template>
          </div>
        </section>

        <!-- Удаление -->
        <Section title="Удаление" footer="После удаления клиент сразу теряет доступ. Восстановить нельзя.">
          <InfoRow label="Удалить клиента">
            <Button size="sm" variant="danger" @click="delOpen = true">Удалить</Button>
          </InfoRow>
        </Section>
      </template>
    </main>

    <ConfirmDialog
      :open="delOpen"
      title="Удалить клиента?"
      :message="`Доступ для «${client?.name}» сразу отзовётся. Восстановить нельзя.`"
      confirm-text="Удалить"
      tone="danger"
      @cancel="delOpen = false"
      @confirm="confirmDelete"
    />
  </div>
</template>
