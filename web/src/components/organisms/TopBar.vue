<script setup lang="ts">
import { computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useClientsStore } from '@/stores/clients'
import { useStatsStore } from '@/stores/stats'
import { useThemeStore } from '@/stores/theme'
import { bytes, handshakeFreshness } from '@/lib/format'
import IconButton from '@/components/atoms/IconButton.vue'
import Icon from '@/components/atoms/Icon.vue'

/*
  Functional top bar. Left: wordmark + live status (онлайн X из Y · ↓rate ↑rate).
  Right: theme toggle, settings / back, logout.

  Live numbers come from the stats store (5s polling) — fed by the
  collector's 5-minute windowed rate. Connected count comes from the
  clients store handshake freshness.
*/

const auth = useAuthStore()
const clients = useClientsStore()
const stats = useStatsStore()
const theme = useThemeStore()
const router = useRouter()
const route = useRoute()

const total = computed(() => clients.items.length)
const online = computed(() =>
  clients.items.filter(c => c.enabled && handshakeFreshness(c.latestHandshakeAt) === 'online').length
)

// Скорость берём из SSE-тика (раз в секунду). Если стрим ещё не успел
// прислать ни одного кадра — падаем на 5-минутный средний из overview, чтобы
// при первом заходе цифра не была пустой.
const rxRate = computed(() => {
  if (stats.liveTs > 0) return stats.liveRxRate
  const o = stats.overview
  return o && o.windowSeconds ? o.rxLast / o.windowSeconds : 0
})
const txRate = computed(() => {
  if (stats.liveTs > 0) return stats.liveTxRate
  const o = stats.overview
  return o && o.windowSeconds ? o.txLast / o.windowSeconds : 0
})
const idle = computed(() => rxRate.value < 1024 && txRate.value < 1024)

async function logout() {
  await auth.logout()
  router.push({ name: 'login' })
}
function toClients()  { router.push({ name: 'clients' }) }
function toSettings() { router.push({ name: 'settings' }) }

const nextTheme = computed(() => ({
  auto:  { icon: 'sun'      as const, next: 'light' as const, title: 'Светлая тема' },
  light: { icon: 'moon'     as const, next: 'dark'  as const, title: 'Тёмная тема' },
  dark:  { icon: 'sparkles' as const, next: 'auto'  as const, title: 'Авто' },
}[theme.mode]))
function cycleTheme() { theme.set(nextTheme.value.next) }
</script>

<template>
  <header class="sticky top-3 z-30 px-4">
    <div class="max-w-5xl mx-auto">
      <div class="glass rounded-2xl h-14 px-3 sm:px-4 flex items-center gap-3 sm:gap-4">
        <!-- Wordmark -->
        <button
          class="flex items-center gap-2.5 -mx-1 px-1 rounded-lg focus-ring shrink-0"
          @click="toClients"
          title="На главную"
        >
          <!--
            Лого в один файл (dark-ink). Под тёмной темой инвертируем CSS-
            фильтром: рисунок монохромный, поэтому invert даёт чистый
            «светлый» вариант без второго ассета.
          -->
          <img
            src="/logo.png"
            alt="Amnezia"
            class="h-7 w-7 rounded-lg object-contain invert dark:invert-0"
            draggable="false"
          />
          <div class="leading-tight text-left hidden sm:block">
            <div class="text-[13.5px] text-ink-900 font-semibold tracking-tight">Amnezia</div>
            <div class="text-[10px] text-ink-500 uppercase tracking-[0.14em] -mt-px">Panel</div>
          </div>
        </button>

        <!-- Live status — the bar's reason to exist -->
        <div class="flex-1 min-w-0 flex items-center justify-center sm:justify-start gap-2 sm:gap-4">
          <!-- Онлайн-счётчик. Зелёная точка пульсирует когда есть живые клиенты,
               серая когда никого. Кликом ведёт обратно на список клиентов. -->
          <button
            class="group h-8 px-2.5 rounded-lg flex items-center gap-2 hover:bg-ink-100/60 transition-colors"
            @click="toClients"
            title="К клиентам"
          >
            <span class="relative inline-flex h-2 w-2">
              <span
                v-if="online > 0"
                class="absolute inset-0 rounded-full bg-success animate-ping opacity-50"
              />
              <span
                class="relative h-2 w-2 rounded-full"
                :class="online > 0 ? 'bg-success' : 'bg-ink-300'"
              />
            </span>
            <span class="text-[12.5px] text-ink-900 tnum font-medium whitespace-nowrap">
              {{ online }}<span class="text-ink-500"> / {{ total }}</span>
            </span>
            <span class="hidden sm:inline text-[10.5px] uppercase tracking-[0.12em] text-ink-500 whitespace-nowrap">онлайн</span>
          </button>

          <div class="hidden sm:block w-px h-5 bg-ink-900/10" />

          <!-- Скорость в моменте. Серый когда тишина, ink когда что-то идёт. -->
          <div class="hidden sm:flex items-center gap-3 tnum">
            <span class="flex items-baseline gap-1">
              <span class="text-[10.5px] uppercase tracking-[0.12em] text-ink-500">↓</span>
              <span
                class="text-[12.5px] font-medium mono"
                :class="idle ? 'text-ink-400' : 'text-ink-900'"
              >{{ idle ? '—' : bytes(rxRate) + '/с' }}</span>
            </span>
            <span class="flex items-baseline gap-1">
              <span class="text-[10.5px] uppercase tracking-[0.12em] text-ink-500">↑</span>
              <span
                class="text-[12.5px] font-medium mono"
                :class="idle ? 'text-ink-400' : 'text-ink-900'"
              >{{ idle ? '—' : bytes(txRate) + '/с' }}</span>
            </span>
          </div>
        </div>

        <!-- Actions -->
        <div class="flex items-center gap-1 shrink-0">
          <slot name="actions" />
          <IconButton :title="nextTheme.title" @click="cycleTheme">
            <Icon :name="nextTheme.icon" :size="18" />
          </IconButton>
          <IconButton
            :title="route.name === 'settings' ? 'На главную' : 'Настройки'"
            @click="route.name === 'settings' ? toClients() : toSettings()"
          >
            <Icon :name="route.name === 'settings' ? 'x' : 'settings'" :size="18" />
          </IconButton>
          <IconButton v-if="auth.requiresPassword && auth.authenticated" title="Выйти" @click="logout">
            <Icon name="logout" :size="18" />
          </IconButton>
        </div>
      </div>
    </div>
  </header>
</template>
