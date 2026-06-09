<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/lib/api'
import { useClientsStore } from '@/stores/clients'
import { useSubscribersStore } from '@/stores/subscribers'
import { useToastStore } from '@/stores/toasts'
import { useInterval } from '@/composables/useInterval'
import { useTitle } from '@/composables/useTitle'
import { handshakeFreshness, relativeTime } from '@/lib/format'
import type { Subscriber, ClientStats } from '@/types'

import { Download, QrCode, ArrowDown, ArrowUp } from 'lucide-vue-next'
import TopBar from '@/components/organisms/TopBar.vue'
import QrModal from '@/components/organisms/QrModal.vue'
import ConfigModal from '@/components/organisms/ConfigModal.vue'
import ConfirmDialog from '@/components/molecules/ConfirmDialog.vue'
import Section from '@/components/molecules/Section.vue'
import Button from '@/components/atoms/Button.vue'
import Input from '@/components/atoms/Input.vue'
import Badge from '@/components/atoms/Badge.vue'
import Skeleton from '@/components/atoms/Skeleton.vue'
import Spinner from '@/components/atoms/Spinner.vue'
import StatusDot from '@/components/atoms/StatusDot.vue'
import Icon from '@/components/atoms/Icon.vue'
import StatBlock from '@/components/molecules/StatBlock.vue'
import Sparkline from '@/components/molecules/Sparkline.vue'

const route   = useRoute()
const router  = useRouter()
const clients = useClientsStore()
const subs    = useSubscribersStore()
const toasts  = useToastStore()

const id      = computed(() => route.params.id as string)
const sub     = ref<Subscriber | null>(null)
const loading = ref(true)
const cs      = ref<ClientStats | null>(null)
const statsLoading = ref(true)

useTitle(() => sub.value ? `${sub.value.name} · Amnezia Panel` : 'Клиент · Amnezia Panel')

// ── Data loading ────────────────────────────────────────────────────────────
// `redirectOnFail` is true only for the FIRST load — polling errors (transient
// 404 from a parallel admin edit, network blip) must not kick the user off
// the page mid-session.
async function loadSub(opts: { redirectOnFail?: boolean } = {}) {
  try {
    sub.value = await api.getSubscriber(id.value)
  } catch (e: any) {
    if (opts.redirectOnFail) {
      toasts.error(e?.message || 'Клиент не найден')
      router.replace({ name: 'clients' })
    }
  } finally {
    loading.value = false
  }
}

async function loadStats() {
  try {
    cs.value = await api.subscriberStats(id.value)
  } catch { /* stats are non-critical */ } finally {
    statsLoading.value = false
  }
}

onMounted(async () => {
  await Promise.all([clients.fetch(), loadSub({ redirectOnFail: true }), loadStats()])
})
useInterval(() => Promise.all([clients.fetch(true), loadSub(), loadStats()]), 8000, { pauseHidden: true })

// ── Devices ─────────────────────────────────────────────────────────────────
const devices = computed(() => sub.value?.devices ?? [])

// Live data from clients store (has handshake metrics)
function liveOf(devId: string) { return clients.byId(devId) }
function freshnessOf(devId: string) {
  return handshakeFreshness(liveOf(devId)?.latestHandshakeAt ?? null)
}
function lastSeenOf(devId: string): string | null {
  return liveOf(devId)?.latestHandshakeAt ?? null
}

const onlineCount = computed(() =>
  devices.value.filter(d => freshnessOf(d.id) === 'online').length
)

// ── Inline name edit ─────────────────────────────────────────────────────────
const renaming     = ref(false)
const renameDraft  = ref('')
const savingName   = ref(false)

function startRename() {
  if (!sub.value) return
  renameDraft.value = sub.value.name
  renaming.value = true
}

async function commitRename() {
  if (!sub.value) return
  const v = renameDraft.value.trim()
  renaming.value = false
  if (!v || v === sub.value.name) return
  savingName.value = true
  try { await subs.patch(sub.value.id, { name: v }); await loadSub() }
  finally { savingName.value = false }
}

// ── Cabinet link ─────────────────────────────────────────────────────────────
async function copyCabinetUrl() {
  if (!sub.value) return
  try {
    await navigator.clipboard.writeText(sub.value.url)
    toasts.success('Ссылка скопирована')
  } catch { toasts.error('Не удалось скопировать') }
}

// ── Inline notes edit ────────────────────────────────────────────────────────
const editingNotes = ref(false)
const notesDraft = ref('')
function startEditNotes() {
  notesDraft.value = sub.value?.notes ?? ''
  editingNotes.value = true
}
async function commitNotes() {
  if (!sub.value) { editingNotes.value = false; return }
  const v = notesDraft.value.trim()
  editingNotes.value = false
  if (v === (sub.value.notes ?? '')) return
  await subs.patch(sub.value.id, { notes: v })
  await loadSub()
}

// ── Device modals ─────────────────────────────────────────────────────────────
const qrFor     = ref<string | null>(null)
const cfgFor    = ref<string | null>(null)
const devDelFor = ref<{ id: string; name: string } | null>(null)

function nameOf(devId: string | null) {
  if (!devId) return undefined
  return devices.value.find(d => d.id === devId)?.name ?? liveOf(devId)?.name
}

async function doDeleteDevice() {
  if (!devDelFor.value) return
  await clients.remove(devDelFor.value.id)
  devDelFor.value = null
  await loadSub()
}

// ── Subscriber actions ────────────────────────────────────────────────────────
const subDelOpen  = ref(false)
const regenOpen   = ref(false)

async function doDeleteSub() {
  if (!sub.value) return
  await subs.remove(sub.value.id)
  router.push({ name: 'clients' })
}

async function doRegen() {
  if (!sub.value) return
  await subs.regenerateToken(sub.value.id)
  regenOpen.value = false
  await loadSub()
}
</script>

<template>
  <div class="min-h-full">
    <TopBar />

    <main class="max-w-5xl mx-auto px-4 sm:px-6 pt-10 pb-16 space-y-10">

      <!-- Breadcrumb -->
      <router-link
        :to="{ name: 'clients' }"
        class="inline-flex items-center gap-1.5 eyebrow hover:text-ink-900 transition-colors"
      >
        <Icon name="chevron-left" :size="14" />
        Все клиенты
      </router-link>

      <!-- ── Header ── -->
      <header class="space-y-4">

        <!-- Skeleton while loading -->
        <template v-if="loading">
          <Skeleton width="180" height="13" />
          <Skeleton width="55%" height="52" rounded="lg" class="mt-2" />
          <Skeleton width="220" height="12" />
        </template>

        <template v-else-if="sub">
          <div class="flex items-start justify-between gap-4 flex-wrap">
            <!-- Name + meta -->
            <div class="space-y-2 min-w-0 animate-rise">
              <div class="eyebrow tnum flex items-center gap-2">
                <span>ID · {{ sub.id }}</span>
                <span class="text-ink-300">·</span>
                <span class="mono normal-case tracking-normal">{{ devices.length }} устр.</span>
                <template v-if="onlineCount > 0">
                  <span class="text-ink-300">·</span>
                  <span class="flex items-center gap-1 text-success">
                    <StatusDot state="online" size="sm" />
                    {{ onlineCount }} онлайн
                  </span>
                </template>
              </div>

              <!-- Editable name -->
              <div v-if="!renaming" class="flex items-baseline gap-3 flex-wrap">
                <h1 class="num-display text-[40px] sm:text-[52px] text-ink-900">{{ sub.name }}</h1>
                <button
                  class="eyebrow text-ink-400 hover:text-ink-900 transition-colors"
                  @click="startRename"
                >
                  <Icon name="edit" :size="13" /> Переименовать
                </button>
              </div>
              <input
                v-else
                v-model="renameDraft"
                class="num-display text-[40px] sm:text-[52px] bg-transparent outline-none border-b-2 border-ink-900 text-ink-900 w-full max-w-md"
                autofocus
                @keydown.enter="commitRename"
                @keydown.escape="renaming = false"
                @blur="commitRename"
              />

              <!-- Notes — click anywhere on the text to edit; explicit button only when empty. -->
              <div v-if="!editingNotes" class="flex items-baseline gap-2 flex-wrap">
                <button
                  v-if="sub.notes"
                  type="button"
                  class="text-left text-[13.5px] text-ink-500 hover:text-ink-900 underline decoration-dotted decoration-ink-300 underline-offset-4 transition-colors"
                  :aria-label="`Изменить заметку: ${sub.notes}`"
                  :title="'Нажмите, чтобы изменить'"
                  @click="startEditNotes"
                >{{ sub.notes }}</button>
                <button
                  v-else
                  type="button"
                  class="eyebrow text-ink-400 hover:text-ink-900 transition-colors"
                  aria-label="Добавить заметку"
                  @click="startEditNotes"
                >
                  <Icon name="edit" :size="12" /> добавить заметку
                </button>
              </div>
              <Input
                v-else
                v-model="notesDraft"
                size="sm"
                placeholder="@vasya · до конца квартала"
                autofocus
                @keydown.enter="commitNotes"
                @keydown.escape="editingNotes = false"
                @blur="commitNotes"
              />
            </div>

            <!-- Actions -->
            <div class="flex items-center gap-2 shrink-0 animate-rise delay-1">
              <Button size="sm" variant="ghost" @click="copyCabinetUrl">
                <Icon name="copy" :size="13" /> Ссылка
              </Button>
              <Button size="sm" variant="ghost" @click="regenOpen = true">
                <Icon name="refresh" :size="13" />
              </Button>
              <Button size="sm" variant="danger" @click="subDelOpen = true">
                Удалить
              </Button>
            </div>
          </div>

          <!-- Cabinet URL — full text, monospace, wraps. Click to copy. -->
          <button
            type="button"
            class="group block w-full text-left mt-2 p-3.5 rounded-xl bg-ink-100 hover:bg-ink-200 transition-colors animate-rise delay-2"
            :title="`Скопировать ${sub.url}`"
            :aria-label="`Скопировать ссылку кабинета ${sub.name}`"
            @click="copyCabinetUrl"
          >
            <div class="flex items-start gap-3">
              <span class="mono text-[11.5px] text-ink-700 break-all leading-relaxed flex-1 min-w-0">{{ sub.url }}</span>
              <Icon name="copy" :size="13" class="text-ink-400 group-hover:text-ink-900 transition-colors shrink-0 mt-0.5" />
            </div>
          </button>
        </template>
      </header>

      <!-- ── Статистика пользователя ── -->
      <section class="space-y-6 animate-rise delay-2">
        <div class="flex items-center gap-4">
          <h2 class="eyebrow">Статистика · все устройства</h2>
          <div class="hairline flex-1" />
        </div>

        <!-- Метрики -->
        <template v-if="statsLoading">
          <div class="grid grid-cols-3 gap-6 sm:gap-4">
            <div v-for="i in 6" :key="i" class="space-y-2">
              <Skeleton width="80" height="10" />
              <Skeleton width="70%" height="36" rounded="lg" />
            </div>
          </div>
        </template>
        <template v-else>
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
            <div class="eyebrow text-ink-500">Онлайн · 7 дн</div>
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
        </template>

        <!-- Sparkline 24ч -->
        <div class="card p-5 sm:p-7">
          <div class="eyebrow mb-4 text-ink-500">Трафик за 24 часа</div>
          <Sparkline v-if="cs && cs.series?.length" :points="cs.series" :height="100" />
          <Skeleton v-else-if="statsLoading" height="100" rounded="lg" />
          <div v-else class="h-[100px] grid place-items-center text-[12px] text-ink-500">
            Трафика за последние 24 часа не было.
          </div>
        </div>
      </section>

      <!-- ── Devices ── -->
      <section class="space-y-4 animate-rise delay-3">
        <div class="flex items-center gap-4">
          <h2 class="eyebrow">
            Устройства · {{ devices.length }}
          </h2>
          <div class="hairline flex-1" />
        </div>

        <!-- Loading -->
        <div v-if="loading" class="card p-10 grid place-items-center">
          <Spinner :size="20" />
        </div>

        <!-- Empty -->
        <div v-else-if="!devices.length" class="card p-8 text-center space-y-2">
          <p class="text-[14px] text-ink-700 font-medium">Устройств пока нет</p>
          <p class="text-[12.5px] text-ink-500 leading-relaxed max-w-sm mx-auto">
            Клиент добавляет устройства самостоятельно через личный кабинет. Отправьте ему ссылку — она указана выше.
          </p>
        </div>

        <!-- Device list -->
        <div v-else class="card divide-y divide-ink-900/5 overflow-hidden">
          <div
            v-for="d in devices"
            :key="d.id"
            class="px-5 py-4 flex items-center gap-3 group"
          >
            <!-- Status dot — shared atom -->
            <span class="shrink-0 mt-0.5">
              <StatusDot :state="freshnessOf(d.id)" size="md" />
            </span>

            <!-- Info — navigates to device detail -->
            <router-link
              :to="{ name: 'client', params: { id: d.id } }"
              class="flex-1 min-w-0 hover:underline"
            >
              <div class="flex items-center gap-2">
                <span class="text-[14px] font-semibold text-ink-900 truncate">{{ d.name }}</span>
                <Badge v-if="!d.enabled" tone="danger" size="xs">выкл</Badge>
              </div>
              <div class="text-[11.5px] text-ink-500 mono mt-0.5">
                {{ d.address }}
                · {{ lastSeenOf(d.id) ? relativeTime(lastSeenOf(d.id)) : 'не подключался' }}
              </div>
            </router-link>

            <!-- Actions — Amnezia-native format primary -->
            <div class="flex items-center gap-1">
              <!-- .vpn download — primary Amnezia format -->
              <a
                :href="api.amneziaVpnUrl(d.id)"
                :download="`${d.name}.vpn`"
                class="h-7 px-2.5 flex items-center gap-1 rounded-lg bg-ink-100 text-ink-700 text-[11.5px] font-medium hover:bg-ink-200 transition-colors"
                title="Скачать .vpn для AmneziaVPN">
                <Download :size="13" /> .vpn
              </a>
              <!-- Amnezia QR — opens 768px QR in new tab -->
              <a
                :href="api.amneziaQrUrl(d.id)"
                target="_blank"
                rel="noopener"
                class="h-7 w-7 flex items-center justify-center rounded-lg text-ink-500 hover:bg-ink-100 transition-colors"
                title="QR-код для AmneziaVPN">
                <QrCode :size="14" />
              </a>
              <!-- .conf — secondary, for WireGuard clients -->
              <Button size="sm" variant="ghost" @click="cfgFor = d.id" title="WireGuard .conf">
                <Icon name="download" :size="12" />
              </Button>
              <Button size="sm" variant="ghost" class="hover:text-danger hover:bg-danger/10" @click="devDelFor = { id: d.id, name: d.name }">
                <Icon name="trash" :size="13" />
              </Button>
            </div>

            <!-- Chevron to detail -->
            <router-link :to="{ name: 'client', params: { id: d.id } }">
              <Icon name="chevron-right" :size="14" class="text-ink-300 group-hover:text-ink-600 transition-colors" />
            </router-link>
          </div>
        </div>

        <p class="text-[11.5px] text-ink-400">
          Устройства добавляются клиентом через кабинет — ссылка выше.
          Нажмите на устройство для подробной статистики.
        </p>
      </section>

    </main>

    <!-- Modals -->
    <QrModal
      :open="!!qrFor"
      :client-id="qrFor"
      :client-name="nameOf(qrFor)"
      @close="qrFor = null"
    />
    <ConfigModal
      :open="!!cfgFor"
      :client-id="cfgFor"
      :client-name="nameOf(cfgFor)"
      @close="cfgFor = null"
    />

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
      :open="subDelOpen"
      title="Удалить клиента?"
      :message="`«${sub?.name ?? ''}» и ВСЕ его устройства будут удалены. Интерфейсы остановятся, ссылка кабинета перестанет работать. Необратимо.`"
      confirm-text="Удалить аккаунт"
      tone="danger"
      @cancel="subDelOpen = false"
      @confirm="doDeleteSub"
    />

    <ConfirmDialog
      :open="regenOpen"
      title="Обновить ссылку кабинета?"
      :message="`Старая ссылка «${sub?.name ?? ''}» сразу перестанет работать — клиенту нужно передать новую. Устройства и подключения сохранятся.`"
      confirm-text="Обновить"
      tone="neutral"
      @cancel="regenOpen = false"
      @confirm="doRegen"
    />
  </div>
</template>
