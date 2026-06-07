<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { api } from '@/lib/api'
import { useToastStore } from '@/stores/toasts'
import { useClientsStore } from '@/stores/clients'
import { useStatsStore } from '@/stores/stats'
import { useProfilesStore } from '@/stores/profiles'
import TopBar from '@/components/organisms/TopBar.vue'
import Section from '@/components/molecules/Section.vue'
import InfoRow from '@/components/molecules/InfoRow.vue'
import CopyButton from '@/components/molecules/CopyButton.vue'
import ConfirmDialog from '@/components/molecules/ConfirmDialog.vue'
import EventRow from '@/components/molecules/EventRow.vue'
import ImportClientModal from '@/components/organisms/ImportClientModal.vue'
import ProfileModal from '@/components/organisms/ProfileModal.vue'
import Button from '@/components/atoms/Button.vue'
import Segmented from '@/components/atoms/Segmented.vue'
import Icon from '@/components/atoms/Icon.vue'
import Skeleton from '@/components/atoms/Skeleton.vue'
import { useThemeStore, type ThemeMode } from '@/stores/theme'
import type { ProfileInfo } from '@/types'

const theme = useThemeStore()
const themeOptions: { value: ThemeMode; label: string }[] = [
  { value: 'auto',  label: 'Авто'    },
  { value: 'light', label: 'Светлая' },
  { value: 'dark',  label: 'Тёмная'  },
]

const toasts = useToastStore()
const clients = useClientsStore()
const statsStore = useStatsStore()
const profiles = useProfilesStore()

const loading = ref(true)
const busyAction = ref<{kind: string; profileId?: string} | null>(null)
const confirmAction = ref<{kind: 'reset' | 'factory' | 'restart' | 'delete'; profileId?: string} | null>(null)

const events = computed(() => statsStore.events)
const eventsLoading = ref(true)

const importOpen = ref(false)
const importBusy = ref(false)

const profileModalOpen = ref(false)
const profileModalMode = ref<'create' | 'edit'>('create')
const editingProfile = ref<ProfileInfo | null>(null)
const profileModalBusy = ref(false)

const restoreInput = ref<HTMLInputElement | null>(null)
const restoreBusy = ref(false)
const restoreConfirmFile = ref<File | null>(null)

async function load() {
  loading.value = true
  eventsLoading.value = true
  try {
    await Promise.all([profiles.fetch(true), statsStore.fetch()])
  } catch (e: any) { toasts.error(e?.message || 'Ошибка загрузки') }
  finally { loading.value = false; eventsLoading.value = false }
}
onMounted(load)

const confirmTitle = computed(() => {
  const k = confirmAction.value?.kind
  return ({
    restart: 'Перезапустить интерфейс?',
    reset:   'Удалить всех клиентов?',
    factory: 'Сброс до заводских настроек?',
    delete:  'Удалить профиль?',
  }[k!] || '')
})
const confirmMessage = computed(() => {
  const k = confirmAction.value?.kind
  return ({
    restart: 'Активные соединения этого профиля ненадолго прервутся, пока интерфейс поднимается. Правила iptables встанут заново.',
    reset:   'Все клиенты во всех профилях будут удалены, доступ отозван. Действие необратимо.',
    factory: 'Будут удалены все профили и клиенты, очищены метрики и журнал. Сервер останется без профилей — нужно будет создать заново. Необратимо.',
    delete:  'Профиль будет удалён. Если в нём есть клиенты — сначала переместите или удалите их.',
  }[k!] || '')
})
const confirmText = computed(() => {
  const k = confirmAction.value?.kind
  return ({ restart: 'Перезапустить', reset: 'Удалить всех', factory: 'Сбросить', delete: 'Удалить' }[k!] || '')
})
const confirmTone = computed(() => {
  const k = confirmAction.value?.kind
  return (k === 'reset' || k === 'factory' || k === 'delete') ? 'danger' : 'neutral'
})

async function doConfirm() {
  const a = confirmAction.value
  if (!a) return
  busyAction.value = { kind: a.kind, profileId: a.profileId }
  confirmAction.value = null
  try {
    if (a.kind === 'restart' && a.profileId) await profiles.restart(a.profileId)
    if (a.kind === 'delete' && a.profileId)  await profiles.remove(a.profileId)
    if (a.kind === 'reset')   { await api.resetClients(); toasts.success('Все клиенты удалены'); await clients.fetch(true) }
    if (a.kind === 'factory') {
      await api.factoryReset(); toasts.success('Сброс выполнен')
      await load(); await clients.fetch(true)
    }
  } catch (e: any) {
    toasts.error(e?.message || 'Ошибка действия')
  } finally {
    busyAction.value = null
  }
}

function openCreateProfile() {
  profileModalMode.value = 'create'
  editingProfile.value = null
  profileModalOpen.value = true
}
function openEditProfile(p: ProfileInfo) {
  profileModalMode.value = 'edit'
  editingProfile.value = p
  profileModalOpen.value = true
}
async function onProfileSubmit(body: any) {
  profileModalBusy.value = true
  try {
    if (profileModalMode.value === 'create') await profiles.create(body)
    else if (editingProfile.value) await profiles.patch(editingProfile.value.id, body)
    profileModalOpen.value = false
  } catch { /* toast already shown */ }
  finally { profileModalBusy.value = false }
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
          Профили подключения, обфускация и действия, которые затрагивают живой трафик.
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

      <Section title="Профили подключения" footer="Каждый профиль — отдельный AmneziaWG-интерфейс на своём UDP-порту. Клиенты привязаны к одному профилю.">
        <template v-if="loading">
          <div class="p-5 space-y-3">
            <Skeleton height="16" width="60%" />
            <Skeleton height="16" width="40%" />
          </div>
        </template>
        <template v-else>
          <div v-if="!profiles.items.length" class="px-5 py-6 text-[12.5px] text-ink-500 leading-relaxed">
            Профилей нет. Создайте первый — обфускацию (Jc/J/S/H/I/Itime) сгенерируйте в
            <a class="underline" target="_blank" rel="noopener" href="https://vadim-khristenko.github.io/AmneziaWG-Architect/">AmneziaWG-Architect</a>
            и вставьте snippet в форме создания.
          </div>
          <div class="divide-y divide-ink-900/5">
            <div v-for="p in profiles.items" :key="p.id" class="px-5 py-4 space-y-3">
              <div class="flex items-baseline justify-between gap-3 flex-wrap">
                <div class="flex items-baseline gap-2">
                  <span class="text-[15px] text-ink-900 font-semibold">{{ p.name }}</span>
                  <span class="mono text-[11px] text-ink-500">{{ p.id }}</span>
                  <span v-if="p.hasMimicry" class="text-[10px] uppercase tracking-[0.12em] text-success px-1.5 py-0.5 rounded bg-success/10">CPS</span>
                  <span v-if="p.itime === 0" class="text-[10px] uppercase tracking-[0.12em] text-ink-500 px-1.5 py-0.5 rounded bg-ink-900/5">Itime off</span>
                </div>
                <div class="text-[11.5px] text-ink-500 tnum mono">
                  {{ p.iface }} · :{{ p.port }} · клиентов {{ p.clientCount }}
                </div>
              </div>
              <p v-if="p.description" class="text-[12.5px] text-ink-500">{{ p.description }}</p>

              <div class="grid grid-cols-1 sm:grid-cols-2 gap-1 text-[12px]">
                <div class="flex items-center gap-2">
                  <span class="text-ink-500 w-24 shrink-0">Endpoint</span>
                  <span class="mono text-ink-900 truncate">{{ p.endpoint }}</span>
                  <CopyButton :value="p.endpoint" />
                </div>
                <div class="flex items-center gap-2">
                  <span class="text-ink-500 w-24 shrink-0">PubKey</span>
                  <span class="mono text-ink-900 truncate">{{ p.publicKey }}</span>
                  <CopyButton :value="p.publicKey" />
                </div>
                <div class="flex items-center gap-2 sm:col-span-2">
                  <span class="text-ink-500 w-24 shrink-0">H1–H4</span>
                  <span class="mono text-ink-700 text-[11px] truncate" :title="`${p.h1} · ${p.h2} · ${p.h3} · ${p.h4}`">
                    {{ p.h1 }} · {{ p.h2 }} · {{ p.h3 }} · {{ p.h4 }}
                  </span>
                </div>
                <div class="flex items-center gap-2">
                  <span class="text-ink-500 w-24 shrink-0">Jc/Jmin-Jmax</span>
                  <span class="mono text-ink-700">{{ p.jc }} · {{ p.jmin }}–{{ p.jmax }}</span>
                </div>
                <div class="flex items-center gap-2">
                  <span class="text-ink-500 w-24 shrink-0">S1/S2/S3/S4</span>
                  <span class="mono text-ink-700">{{ p.s1 }}/{{ p.s2 }}/{{ p.s3 }}/{{ p.s4 }}</span>
                </div>
                <div class="flex items-center gap-2">
                  <span class="text-ink-500 w-24 shrink-0">Itime</span>
                  <span class="mono text-ink-700">{{ p.itime === 0 ? 'выкл' : `${p.itime}s` }}</span>
                </div>
              </div>

              <div class="flex items-center gap-2 flex-wrap pt-1">
                <Button size="sm" variant="ghost" @click="openEditProfile(p)">
                  <Icon name="settings" :size="13" /> Редактировать
                </Button>
                <Button size="sm" variant="ghost"
                  :loading="busyAction?.kind === 'restart' && busyAction?.profileId === p.id"
                  @click="confirmAction = { kind: 'restart', profileId: p.id }">
                  <Icon name="power" :size="13" /> Перезапустить
                </Button>
                <Button size="sm" variant="ghost"
                  :disabled="p.clientCount > 0"
                  :title="p.clientCount > 0 ? 'Сначала переместите или удалите клиентов' : ''"
                  @click="confirmAction = { kind: 'delete', profileId: p.id }">
                  <Icon name="trash" :size="13" /> Удалить
                </Button>
              </div>
            </div>
          </div>
          <div class="p-4 border-t border-ink-900/5">
            <Button size="sm" variant="primary" @click="openCreateProfile">
              <Icon name="plus" :size="14" /> Добавить профиль
            </Button>
          </div>
        </template>
      </Section>

      <Section title="Клиенты" footer="Импорт нужен, если у клиента уже есть конфиг и нужно вернуть его в панель по публичному ключу.">
        <InfoRow label="Импорт по публичному ключу">
          <Button size="sm" @click="importOpen = true">
            <Icon name="plus" :size="14" /> Импортировать
          </Button>
        </InfoRow>
      </Section>

      <Section title="Резервная копия" footer="Архив: state.json (профили + клиенты) + .conf каждого интерфейса + база метрик. Восстановление перезаписывает текущее состояние и перезапускает интерфейсы.">
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
      :tone="confirmTone"
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

    <ProfileModal
      :open="profileModalOpen"
      :mode="profileModalMode"
      :profile="editingProfile"
      :busy="profileModalBusy"
      @close="profileModalOpen = false"
      @submit="onProfileSubmit"
    />
  </div>
</template>
