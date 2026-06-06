<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import type { ServerInfo } from '@/types'
import { api } from '@/lib/api'
import { useToastStore } from '@/stores/toasts'
import { useClientsStore } from '@/stores/clients'
import { useStatsStore } from '@/stores/stats'
import TopBar from '@/components/organisms/TopBar.vue'
import Section from '@/components/molecules/Section.vue'
import InfoRow from '@/components/molecules/InfoRow.vue'
import CopyButton from '@/components/molecules/CopyButton.vue'
import ConfirmDialog from '@/components/molecules/ConfirmDialog.vue'
import EventRow from '@/components/molecules/EventRow.vue'
import ImportClientModal from '@/components/organisms/ImportClientModal.vue'
import Button from '@/components/atoms/Button.vue'
import Segmented from '@/components/atoms/Segmented.vue'
import Icon from '@/components/atoms/Icon.vue'
import Skeleton from '@/components/atoms/Skeleton.vue'
import { useThemeStore, type ThemeMode } from '@/stores/theme'

const theme = useThemeStore()
const themeOptions: { value: ThemeMode; label: string }[] = [
  { value: 'auto',  label: 'Авто'    },
  { value: 'light', label: 'Светлая' },
  { value: 'dark',  label: 'Тёмная'  },
]

const toasts = useToastStore()
const clients = useClientsStore()
const statsStore = useStatsStore()

const info = ref<ServerInfo | null>(null)
const loading = ref(true)
const busy = ref<'regen' | 'restart' | 'reset' | null>(null)
const confirmKind = ref<'regen' | 'restart' | 'reset' | null>(null)

// События берём из общего stats store — туда же SSE-push кладёт новые.
const events = computed(() => statsStore.events)
const eventsLoading = ref(true)

const importOpen = ref(false)
const importBusy = ref(false)

const restoreInput = ref<HTMLInputElement | null>(null)
const restoreBusy = ref(false)
const restoreConfirmFile = ref<File | null>(null)

async function load() {
  loading.value = true
  eventsLoading.value = true
  try {
    const [srv] = await Promise.all([api.serverInfo(), statsStore.fetch()])
    info.value = srv
  } catch (e: any) { toasts.error(e?.message || 'Ошибка загрузки') }
  finally { loading.value = false; eventsLoading.value = false }
}
onMounted(load)

const confirmTitle = computed(() => ({
  regen:   'Обновить H1–H4?',
  restart: 'Перезапустить интерфейс?',
  reset:   'Удалить всех клиентов?',
}[confirmKind.value!] || ''))

const confirmMessage = computed(() => ({
  regen:   'Существующие конфиги перестанут работать — каждому клиенту нужно будет загрузить новый. Ключ сервера остаётся прежним.',
  restart: 'Активные соединения ненадолго прервутся, пока интерфейс поднимается. Правила iptables встанут заново автоматически.',
  reset:   'Все клиенты будут удалены, доступ отозван. Восстановить нельзя.',
}[confirmKind.value!] || ''))

const confirmText = computed(() => ({
  regen:   'Обновить',
  restart: 'Перезапустить',
  reset:   'Удалить всех',
}[confirmKind.value!] || ''))

async function doConfirm() {
  const k = confirmKind.value
  if (!k) return
  busy.value = k
  confirmKind.value = null
  try {
    if (k === 'regen')   { info.value = await api.regenerateMagic(); toasts.success('Ключи обновлены') }
    if (k === 'restart') { await api.restartInterface(); toasts.success('Интерфейс перезапущен'); await load() }
    if (k === 'reset')   { await api.resetClients();    toasts.success('Все клиенты удалены'); await clients.fetch(true) }
  } catch (e: any) {
    toasts.error(e?.message || 'Ошибка действия')
  } finally {
    busy.value = null
  }
}

async function onImport(body: Parameters<typeof api.importClient>[0]) {
  importBusy.value = true
  try {
    await api.importClient(body)
    toasts.success('Клиент импортирован')
    importOpen.value = false
    await clients.fetch(true)
  } catch (e: any) {
    toasts.error(e?.message || 'Ошибка импорта')
  } finally {
    importBusy.value = false
  }
}

function downloadBackup() {
  // Прямая навигация — браузер сам поднимет диалог сохранения и принесёт
  // cookie сессии. Никакого XHR-blob не нужно: файл может быть большим.
  window.location.href = api.backupUrl()
}

function pickRestore() { restoreInput.value?.click() }

function onRestoreFile(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  restoreConfirmFile.value = f
  // Reset так, чтобы один и тот же файл можно было выбрать повторно.
  ;(e.target as HTMLInputElement).value = ''
}

async function doRestore() {
  const f = restoreConfirmFile.value
  if (!f) return
  restoreConfirmFile.value = null
  restoreBusy.value = true
  try {
    const res = await api.restore(f)
    toasts.success(`Восстановлено: ${res.restored.join(', ')}`)
    await load()
    await clients.fetch(true)
  } catch (e: any) {
    toasts.error(e?.message || 'Ошибка восстановления')
  } finally {
    restoreBusy.value = false
  }
}
</script>

<template>
  <div class="min-h-full">
    <TopBar />

    <main class="max-w-5xl mx-auto px-4 sm:px-6 pt-10 pb-16 space-y-10">
      <header class="space-y-3 animate-rise">
        <div class="eyebrow">Сервер</div>
        <h1 class="num-display text-ink-900 text-[40px] sm:text-[48px]">Настройки</h1>
        <p class="text-[13.5px] text-ink-500 leading-relaxed max-w-md">
          Параметры сервера, обфускация AmneziaWG и действия, которые затрагивают живой трафик.
        </p>
      </header>

      <Section title="Оформление" :footer="`Сейчас активна ${ {light:'светлая', dark:'тёмная'}[theme.resolved as 'light'|'dark'] || 'системная' } тема.`">
        <InfoRow label="Тема">
          <Segmented
            :model-value="theme.mode"
            :options="themeOptions"
            @update:model-value="(v: ThemeMode) => theme.set(v)"
          />
        </InfoRow>
      </Section>

      <!-- Skeleton state -->
      <template v-if="loading">
        <Section title="Подключение">
          <div class="p-5 space-y-3">
            <Skeleton height="16" width="60%" />
            <Skeleton height="16" width="40%" />
            <Skeleton height="16" width="50%" />
          </div>
        </Section>
        <Section title="Ключ сервера">
          <div class="p-5 space-y-3">
            <Skeleton height="16" width="80%" />
            <Skeleton height="16" width="35%" />
          </div>
        </Section>
      </template>

      <template v-else-if="info">
        <Section title="Подключение">
          <InfoRow label="Внешний адрес" :value="info.endpoint" mono show-divider>
            <CopyButton :value="info.endpoint" />
          </InfoRow>
          <InfoRow label="Интерфейс"     :value="info.interface" mono show-divider />
          <InfoRow label="UDP-порт"      :value="String(info.port)" mono show-divider />
          <InfoRow label="Внешняя сетевая карта" :value="info.egressIface" mono />
        </Section>

        <Section title="Ключ сервера" footer="Этим ключом сервер представляется клиентам при подключении.">
          <InfoRow label="Публичный ключ" :value="info.publicKey" mono show-divider>
            <CopyButton :value="info.publicKey" />
          </InfoRow>
          <InfoRow label="IP в туннеле" :value="info.address" mono />
        </Section>

        <Section title="Обфускация" footer="H1–H4 — случайные числа, маскирующие WireGuard под обычный трафик. После обновления все клиенты должны переподключиться по новому конфигу.">
          <InfoRow label="Jc / Jmin / Jmax" :value="`${info.jc} · ${info.jmin} · ${info.jmax}`" mono show-divider />
          <InfoRow label="S1 / S2"          :value="`${info.s1} · ${info.s2}`" mono show-divider />
          <InfoRow label="H1" :value="info.h1" mono show-divider />
          <InfoRow label="H2" :value="info.h2" mono show-divider />
          <InfoRow label="H3" :value="info.h3" mono show-divider />
          <InfoRow label="H4" :value="info.h4" mono show-divider />
          <InfoRow label="Обновить ключи">
            <Button size="sm" :loading="busy === 'regen'" @click="confirmKind = 'regen'">
              <Icon name="refresh" :size="14" /> Обновить H1–H4
            </Button>
          </InfoRow>
        </Section>

        <Section title="Сеть по умолчанию" footer="Эти параметры подставляются в каждый новый конфиг клиента.">
          <InfoRow label="Подсеть"      :value="info.subnet" mono show-divider />
          <InfoRow label="DNS"          :value="info.dns" mono show-divider />
          <InfoRow label="MTU"          :value="info.mtu ? String(info.mtu) : '— (авто)'" mono show-divider />
          <InfoRow label="Allowed IPs"  :value="info.allowedIPs" mono show-divider />
          <InfoRow label="Keepalive"    :value="info.persistentKeepalive ? `${info.persistentKeepalive} с` : 'выключен'" mono />
        </Section>

        <Section title="Клиенты" footer="Импорт нужен, если у клиента уже есть конфиг и нужно вернуть его в панель по публичному ключу.">
          <InfoRow label="Импорт по публичному ключу" show-divider>
            <Button size="sm" @click="importOpen = true">
              <Icon name="plus" :size="14" /> Импортировать
            </Button>
          </InfoRow>
          <InfoRow label="Всего клиентов" :value="`${info.clientCount}`" mono />
        </Section>

        <Section title="Резервная копия" footer="Архив содержит JSON состояния, серверный .conf и базу метрик. Восстановление перезаписывает текущее состояние и перезапускает интерфейс.">
          <InfoRow label="Скачать архив" show-divider>
            <Button size="sm" @click="downloadBackup">
              <Icon name="download" :size="14" /> Скачать .tar.gz
            </Button>
          </InfoRow>
          <InfoRow label="Восстановить из архива">
            <input
              ref="restoreInput"
              type="file"
              accept=".gz,.tgz,application/gzip"
              class="hidden"
              @change="onRestoreFile"
            />
            <Button size="sm" :loading="restoreBusy" @click="pickRestore">
              <Icon name="upload" :size="14" /> Выбрать файл
            </Button>
          </InfoRow>
        </Section>

        <Section title="Опасные действия" footer="Эти действия затрагивают живой трафик. Если нужны только новые H1–H4 — используйте кнопку выше.">
          <InfoRow label="Перезапустить интерфейс" show-divider>
            <Button size="sm" :loading="busy === 'restart'" @click="confirmKind = 'restart'">
              <Icon name="power" :size="14" /> Перезапустить
            </Button>
          </InfoRow>
          <InfoRow label="Удалить всех клиентов">
            <Button variant="danger" size="sm" :loading="busy === 'reset'" @click="confirmKind = 'reset'">
              <Icon name="trash" :size="14" /> Удалить всех
            </Button>
          </InfoRow>
        </Section>
      </template>

      <!-- История событий — всегда показываем, после серверной секции. -->
      <Section title="История событий" footer="Последние 50 действий: создание, удаление, переименование, истечение, серверные операции. Хранятся 30 дней.">
        <template v-if="eventsLoading">
          <div class="p-4 space-y-3">
            <div v-for="i in 5" :key="i" class="flex items-center gap-3">
              <Skeleton width="6" height="6" rounded="full" />
              <Skeleton width="80" height="10" />
              <Skeleton height="14" />
            </div>
          </div>
        </template>
        <template v-else-if="events.length">
          <div class="divide-y divide-ink-900/5">
            <EventRow v-for="e in events" :key="e.id" :event="e" />
          </div>
        </template>
        <div v-else class="p-8 text-center text-[12.5px] text-ink-500">
          Пока ничего не происходило.
        </div>
      </Section>
    </main>

    <ConfirmDialog
      :open="confirmKind !== null"
      :title="confirmTitle"
      :message="confirmMessage"
      :confirm-text="confirmText"
      :tone="confirmKind === 'reset' ? 'danger' : 'neutral'"
      @cancel="confirmKind = null"
      @confirm="doConfirm"
    />

    <ConfirmDialog
      :open="restoreConfirmFile !== null"
      title="Восстановить из архива?"
      :message="`Текущее состояние будет перезаписано файлом «${restoreConfirmFile?.name ?? ''}» и интерфейс перезапустится. Активные соединения прервутся.`"
      confirm-text="Восстановить"
      tone="danger"
      @cancel="restoreConfirmFile = null"
      @confirm="doRestore"
    />

    <ImportClientModal
      :open="importOpen"
      :busy="importBusy"
      @close="importOpen = false"
      @submit="onImport"
    />
  </div>
</template>
