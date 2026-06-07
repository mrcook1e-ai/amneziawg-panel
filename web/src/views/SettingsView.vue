<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { api } from '@/lib/api'
import { useToastStore } from '@/stores/toasts'
import { useClientsStore } from '@/stores/clients'
import { useStatsStore } from '@/stores/stats'
import TopBar from '@/components/organisms/TopBar.vue'
import Section from '@/components/molecules/Section.vue'
import InfoRow from '@/components/molecules/InfoRow.vue'
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

const busyAction = ref<{kind: string} | null>(null)
const confirmAction = ref<{kind: 'reset' | 'factory'} | null>(null)

const events = computed(() => statsStore.events)
const eventsLoading = ref(true)

const importOpen = ref(false)
const importBusy = ref(false)

const restoreInput = ref<HTMLInputElement | null>(null)
const restoreBusy = ref(false)
const restoreConfirmFile = ref<File | null>(null)

async function load() {
  eventsLoading.value = true
  try { await statsStore.fetch() }
  catch (e: any) { toasts.error(e?.message || 'Ошибка загрузки') }
  finally { eventsLoading.value = false }
}
onMounted(load)

const TITLES: Record<'reset' | 'factory', string> = {
  reset:   'Удалить всех клиентов?',
  factory: 'Сброс до заводских настроек?',
}
const MESSAGES: Record<'reset' | 'factory', string> = {
  reset:   'Все клиенты будут удалены, доступ отозван. Профили и инвайты сохранятся. Действие необратимо.',
  factory: 'Будут удалены все клиенты, профили, ссылки и метрики. Сервер вернётся в исходное состояние. Необратимо.',
}
const TEXTS: Record<'reset' | 'factory', string> = {
  reset: 'Удалить всех', factory: 'Сбросить всё',
}
const confirmTitle = computed(() => confirmAction.value ? TITLES[confirmAction.value.kind] : '')
const confirmMessage = computed(() => confirmAction.value ? MESSAGES[confirmAction.value.kind] : '')
const confirmText = computed(() => confirmAction.value ? TEXTS[confirmAction.value.kind] : '')

async function doConfirm() {
  const a = confirmAction.value
  if (!a) return
  busyAction.value = { kind: a.kind }
  confirmAction.value = null
  try {
    if (a.kind === 'reset')   { await api.resetClients(); toasts.success('Все клиенты удалены'); await clients.fetch(true) }
    if (a.kind === 'factory') { await api.factoryReset(); toasts.success('Сброс выполнен'); await load(); await clients.fetch(true) }
  } catch (e: any) {
    toasts.error(e?.message || 'Ошибка действия')
  } finally {
    busyAction.value = null
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

function downloadBackup() { window.location.href = api.backupUrl() }
function pickRestore() { restoreInput.value?.click() }
function onRestoreFile(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  restoreConfirmFile.value = f
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
          Оформление, бэкапы, опасные действия и журнал событий.
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

      <Section title="Восстановление клиента" footer="Если у клиента уже есть готовый .conf и нужно вернуть его в панель — импортируйте по публичному ключу. Профиль и интерфейс будут созданы автоматически, как при онбординге через ссылку.">
        <InfoRow label="Импорт по публичному ключу">
          <Button size="sm" @click="importOpen = true">
            <Icon name="plus" :size="14" /> Импортировать
          </Button>
        </InfoRow>
      </Section>

      <Section title="Резервная копия" footer="Архив: state.json (профили + клиенты + инвайты) + .conf каждого интерфейса + база метрик. Восстановление перезаписывает текущее состояние и перезапускает интерфейсы.">
        <InfoRow label="Скачать архив" show-divider>
          <Button size="sm" @click="downloadBackup">
            <Icon name="download" :size="14" /> Скачать .tar.gz
          </Button>
        </InfoRow>
        <InfoRow label="Восстановить из архива">
          <input ref="restoreInput" type="file" accept=".gz,.tgz,application/gzip" class="hidden" @change="onRestoreFile" />
          <Button size="sm" :loading="restoreBusy" @click="pickRestore">
            <Icon name="upload" :size="14" /> Выбрать файл
          </Button>
        </InfoRow>
      </Section>

      <Section title="Опасные действия" footer="Эти действия затрагивают живой трафик и необратимы.">
        <InfoRow label="Удалить всех клиентов" show-divider>
          <Button variant="danger" size="sm" :loading="busyAction?.kind === 'reset'"
            @click="confirmAction = { kind: 'reset' }">
            <Icon name="trash" :size="14" /> Удалить всех
          </Button>
        </InfoRow>
        <InfoRow label="Сброс до заводских настроек">
          <Button variant="danger" size="sm" :loading="busyAction?.kind === 'factory'"
            @click="confirmAction = { kind: 'factory' }">
            <Icon name="refresh" :size="14" /> Сбросить всё
          </Button>
        </InfoRow>
      </Section>

      <Section title="История событий" footer="Последние 50 действий. Хранятся 30 дней.">
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
      :open="confirmAction !== null"
      :title="confirmTitle"
      :message="confirmMessage"
      :confirm-text="confirmText"
      tone="danger"
      @cancel="confirmAction = null"
      @confirm="doConfirm"
    />

    <ConfirmDialog
      :open="restoreConfirmFile !== null"
      title="Восстановить из архива?"
      :message="`Текущее состояние будет перезаписано файлом «${restoreConfirmFile?.name ?? ''}» и интерфейсы перезапустятся.`"
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
