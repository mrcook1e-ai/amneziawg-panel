<script setup lang="ts">
/*
  Страница клиента.

  Содержит: идентификация, телеметрия, 24-часовой график, редактируемые
  поля (заметка, срок действия, DNS / AllowedIPs / MTU override), журнал
  событий, опасная зона.
*/

import { computed, onMounted, ref, watch, h, defineComponent } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/lib/api'
import { useClientsStore } from '@/stores/clients'
import { useInterval } from '@/composables/useInterval'
import { useToastStore } from '@/stores/toasts'
import { bytes, bytesParts, relativeTime, handshakeFreshness, stateLabelRu } from '@/lib/format'
import type { ClientStats, AppEvent } from '@/types'

import TopBar from '@/components/organisms/TopBar.vue'
import Sparkline from '@/components/molecules/Sparkline.vue'
import Section from '@/components/molecules/Section.vue'
import InfoRow from '@/components/molecules/InfoRow.vue'
import CopyButton from '@/components/molecules/CopyButton.vue'
import EventRow from '@/components/molecules/EventRow.vue'
import ConfirmDialog from '@/components/molecules/ConfirmDialog.vue'
import QrModal from '@/components/organisms/QrModal.vue'
import ConfigModal from '@/components/organisms/ConfigModal.vue'
import Button from '@/components/atoms/Button.vue'
import Input from '@/components/atoms/Input.vue'
import DatePicker from '@/components/molecules/DatePicker.vue'
import Switch from '@/components/atoms/Switch.vue'
import Spinner from '@/components/atoms/Spinner.vue'
import Skeleton from '@/components/atoms/Skeleton.vue'
import Icon from '@/components/atoms/Icon.vue'

// Локальная подкомпонента для одной метрики (eyebrow + большая цифра + ед.).
const Stat = defineComponent({
  props: {
    eyebrow: { type: String, required: true },
    value:   { type: Number, default: 0 },
    raw:     { type: String, default: '' },
  },
  setup(p) {
    return () => {
      const v = p.raw ? { value: p.raw, unit: '' } : bytesParts(p.value || 0)
      return h('div', { class: 'space-y-1' }, [
        h('div', { class: 'eyebrow truncate' }, p.eyebrow),
        h('div', { class: 'flex items-baseline gap-1.5' }, [
          h('span', { class: 'num-display-soft tnum text-ink-900 text-[34px] sm:text-[40px]' }, v.value),
          v.unit ? h('span', { class: 'mono text-[10.5px] text-ink-500 uppercase tracking-wider' }, v.unit) : null,
        ]),
      ])
    }
  },
})

const route   = useRoute()
const router  = useRouter()
const clients = useClientsStore()
const toasts  = useToastStore()

const id = computed(() => route.params.id as string)
const client = computed(() => clients.byId(id.value))

const cs       = ref<ClientStats | null>(null)
const events   = ref<AppEvent[]>([])
const loading  = ref(true)
const savingId = ref<string | null>(null)

const form = ref({
  notes:              '',
  expiresAt:          '' as string,
  dnsOverride:        '',
  allowedIPsOverride: '',
  mtuOverride:        0,
})
function syncFormFromClient() {
  const c = client.value
  if (!c) return
  form.value.notes              = c.notes ?? ''
  form.value.expiresAt          = c.expiresAt ? new Date(c.expiresAt).toISOString().slice(0, 10) : ''
  form.value.dnsOverride        = c.dnsOverride ?? ''
  form.value.allowedIPsOverride = c.allowedIPsOverride ?? ''
  form.value.mtuOverride        = c.mtuOverride ?? 0
}
// Sync form ONLY when navigating to a different client. Polling updates
// (every 5s) must not blow away whatever the user is currently typing into
// notes / DNS / MTU / etc.
watch(() => client.value?.id, syncFormFromClient, { immediate: true })

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
  await loadAll()
})
useInterval(() => clients.fetch(true), 5000, { pauseHidden: true })
useInterval(loadAll, 8000, { pauseHidden: true })

const live = computed(() => {
  if (!cs.value) return 0
  return (cs.value.rxLast + cs.value.txLast) / Math.max(1, cs.value.windowSeconds)
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

const qrOpen  = ref(false)
const cfgOpen = ref(false)
const delOpen = ref(false)

async function toggleEnabled(v: boolean) {
  if (!client.value) return
  await clients.setEnabled(client.value.id, v)
}

async function saveOverrides() {
  if (!client.value) return
  if (savingId.value) return
  savingId.value = client.value.id
  try {
    const c = client.value
    await clients.patch(c.id, {
      notes:              form.value.notes !== (c.notes ?? '') ? form.value.notes : undefined,
      dnsOverride:        form.value.dnsOverride !== (c.dnsOverride ?? '') ? form.value.dnsOverride : undefined,
      allowedIPsOverride: form.value.allowedIPsOverride !== (c.allowedIPsOverride ?? '') ? form.value.allowedIPsOverride : undefined,
      mtuOverride:        form.value.mtuOverride !== (c.mtuOverride ?? 0) ? Number(form.value.mtuOverride) : undefined,
      expiresAt:          form.value.expiresAt
                            ? new Date(form.value.expiresAt + 'T23:59:59Z').toISOString()
                            : undefined,
      clearExpiresAt:     form.value.expiresAt === '' && !!c.expiresAt,
    })
    // clients.patch() already toasts success/error — don't double-toast.
  } finally {
    savingId.value = null
  }
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
              <span class="inline-block h-1.5 w-1.5 rounded-full" :class="stateDot" />
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
        <section class="grid grid-cols-2 sm:grid-cols-4 gap-6 sm:gap-4 animate-rise delay-1">
          <Stat eyebrow="За 5 минут"   :value="cs?.rxLast || 0" />
          <Stat eyebrow="За 24 часа"   :value="cs?.rx24h || 0" />
          <Stat eyebrow="За 7 дней"    :value="cs?.rx7d || 0" />
          <Stat eyebrow="Доступен · 7 дн" :raw="Math.round((cs?.onlineRatio7d || 0) * 100) + '%'" />
        </section>

        <!-- График 24ч -->
        <section class="space-y-3 animate-rise delay-2">
          <div class="flex items-center gap-4">
            <div class="eyebrow">Трафик за 24 часа</div>
            <div class="hairline flex-1" />
            <div class="text-[11px] mono tnum flex items-center gap-1.5">
              <span
                v-if="live > 1024"
                class="inline-block h-1.5 w-1.5 rounded-full bg-success animate-pulse"
              />
              <span :class="live > 1024 ? 'text-success' : 'text-ink-500'">
                {{ live > 1024 ? bytes(live) + '/с' : 'нет активности' }}
              </span>
            </div>
          </div>
          <div class="card p-5 sm:p-7">
            <Sparkline v-if="cs && cs.series.length" :points="cs.series" :height="140" />
            <Skeleton v-else-if="loading" height="140" rounded="lg" />
            <div v-else class="h-[140px] grid place-items-center text-[12px] text-ink-500">
              <span>Трафика за последние 24 часа не было.</span>
            </div>
          </div>
        </section>

        <!-- Идентификация -->
        <Section title="Идентификация" footer="Нажмите значение, чтобы скопировать. Публичный ключ — то, что сервер проверяет при каждом подключении.">
          <InfoRow label="IP-адрес" :value="client.address" mono show-divider>
            <CopyButton :value="client.address" />
          </InfoRow>
          <InfoRow label="Публичный ключ" :value="client.publicKey" mono show-divider>
            <CopyButton :value="client.publicKey" />
          </InfoRow>
          <InfoRow label="Добавлен" :value="new Date(client.createdAt).toLocaleString('ru-RU')" />
        </Section>

        <!-- Настройки -->
        <Section title="Настройки" footer="Описание видно только в панели. Переопределения попадают в выданный конфиг; пусто — берётся значение сервера.">
          <div class="px-4 py-4 space-y-4">
            <div>
              <label class="eyebrow block mb-2">Описание</label>
              <Input v-model="form.notes" placeholder="например: рабочий ноут, отозвать после командировки" />
            </div>

            <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label class="eyebrow block mb-2">Срок действия</label>
                <DatePicker v-model="form.expiresAt" />
                <p class="mt-1 text-[11px] text-ink-500">Когда наступит — доступ отключится автоматически.</p>
              </div>
              <div>
                <label class="eyebrow block mb-2">MTU</label>
                <Input v-model.number="form.mtuOverride" type="number" placeholder="по умолчанию" />
              </div>
            </div>

            <div>
              <label class="eyebrow block mb-2">DNS</label>
              <Input v-model="form.dnsOverride" placeholder="по умолчанию" mono />
            </div>

            <div>
              <label class="eyebrow block mb-2">Allowed IPs</label>
              <Input v-model="form.allowedIPsOverride" placeholder="по умолчанию" mono />
            </div>
          </div>
          <div class="hairline mx-4" />
          <div class="px-4 py-3 flex items-center justify-between gap-3">
            <span class="text-[11.5px] text-ink-500">Применится при следующей загрузке конфига.</span>
            <div class="flex items-center gap-2">
              <Button size="sm" variant="ghost" @click="syncFormFromClient">Отмена</Button>
              <Button size="sm" variant="primary" :loading="savingId === client.id" @click="saveOverrides">Сохранить</Button>
            </div>
          </div>
        </Section>

        <!-- Доступ -->
        <Section title="Доступ">
          <InfoRow label="Включён" show-divider>
            <Switch :model-value="client.enabled" @update:model-value="toggleEnabled" />
          </InfoRow>
          <InfoRow label="Скачать конфиг" show-divider>
            <Button size="sm" @click="cfgOpen = true"><Icon name="download" :size="14" /> .conf / .vpn</Button>
          </InfoRow>
          <InfoRow label="QR-код">
            <Button size="sm" @click="qrOpen = true"><Icon name="qrcode" :size="14" /> Показать</Button>
          </InfoRow>
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

    <QrModal     :open="qrOpen"  :client-id="client?.id || null" :client-name="client?.name" @close="qrOpen = false" />
    <ConfigModal :open="cfgOpen" :client-id="client?.id || null" :client-name="client?.name" @close="cfgOpen = false" />

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
