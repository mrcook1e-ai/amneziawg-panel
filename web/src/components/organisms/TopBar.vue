<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useClientsStore } from '@/stores/clients'
import { useStatsStore } from '@/stores/stats'
import { useSubscribersStore } from '@/stores/subscribers'
import { useThemeStore } from '@/stores/theme'
import { bytes, handshakeFreshness } from '@/lib/format'
import IconButton from '@/components/atoms/IconButton.vue'
import Icon from '@/components/atoms/Icon.vue'
import { ArrowDown, ArrowUp, ReceiptText } from 'lucide-vue-next'

/*
  Functional top bar. Left: wordmark + live status (онлайн X из Y · ↓rate ↑rate).
  Right: theme toggle (sun/moon), settings / back, logout.

  Live numbers come from the stats store (5s polling) — fed by the
  collector's 5-minute windowed rate. Connected count comes from the
  clients store handshake freshness.
*/

const auth = useAuthStore()
const clients = useClientsStore()
const stats = useStatsStore()
const subs = useSubscribersStore()
const theme = useThemeStore()
const router = useRouter()
const route = useRoute()

// Manual refresh — triggers all admin store refetches in parallel. Useful
// when the admin just made a change on the server outside the panel and
// doesn't want to wait for the next polling tick.
const refreshing = ref(false)
async function refreshAll() {
  if (refreshing.value) return
  refreshing.value = true
  try {
    await Promise.all([
      clients.fetch(true),
      subs.fetch(true),
      stats.fetch(),
    ])
  } finally {
    refreshing.value = false
  }
}

const total = computed(() => clients.items.length)
const online = computed(() =>
  clients.items.filter(c => c.enabled && handshakeFreshness(c.latestHandshakeAt) === 'online').length
)

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
function toBilling()  { router.push({ name: 'billing' }) }

// Simple two-way toggle: light ↔ dark. Auto mode is available via Settings.
// 'auto' is treated as its currently-resolved theme for icon purposes.
const isDark = computed(() => {
  if (theme.mode === 'auto') return theme.resolved === 'dark'
  return theme.mode === 'dark'
})
function toggleTheme() { theme.set(isDark.value ? 'light' : 'dark') }
</script>

<template>
  <!-- Ambient amber glow — dark only, matches cabinet visual language. -->
  <div
    class="pointer-events-none fixed inset-x-0 top-0 h-[280px] opacity-0 dark:opacity-100 transition-opacity duration-700 z-10"
    style="background: radial-gradient(ellipse 70% 100% at 50% 0%, rgba(232,160,65,0.07) 0%, transparent 70%)"
    aria-hidden="true"
  />

  <header class="sticky top-3 z-30 px-3 sm:px-4">
    <div class="max-w-5xl mx-auto">
      <div class="glass rounded-2xl h-14 px-2.5 sm:px-4 flex items-center gap-2 sm:gap-4">
        <!-- Wordmark -->
        <button
          class="flex items-center gap-2.5 -mx-1 px-1 rounded-lg focus-ring shrink-0"
          @click="toClients"
          title="На главную"
          aria-label="На главную"
        >
          <img
            src="/logo.png"
            alt="Amnezia"
            class="h-7 w-7 rounded-lg object-contain invert dark:invert-0"
            draggable="false"
          />
          <div class="leading-tight text-left hidden md:block">
            <div class="text-[13.5px] text-ink-900 font-semibold tracking-tight">Amnezia</div>
            <div class="text-[10px] text-ink-500 uppercase tracking-[0.14em] -mt-px">Panel</div>
          </div>
        </button>

        <!-- Live status — collapses gracefully on mobile -->
        <div class="flex-1 min-w-0 flex items-center justify-start gap-2 sm:gap-4">
          <button
            class="group h-8 px-2 sm:px-2.5 rounded-lg flex items-center gap-2 hover:bg-ink-100/60 transition-colors min-w-0"
            @click="toClients"
            title="К клиентам"
            aria-label="К списку клиентов"
          >
            <span class="relative inline-flex items-center justify-center w-2.5 h-2.5 shrink-0">
              <span
                v-if="online > 0"
                class="absolute inset-0 rounded-full bg-success opacity-25 animate-ping-slow"
              />
              <span
                class="relative block w-2 h-2 rounded-full transition-colors"
                :class="online > 0 ? 'bg-success' : 'bg-ink-300'"
              />
            </span>
            <span class="text-[12.5px] text-ink-900 tnum font-medium whitespace-nowrap">
              {{ online }}<span class="text-ink-400"> / {{ total }}</span>
            </span>
            <span class="hidden md:inline text-[10.5px] uppercase tracking-[0.12em] text-ink-500 whitespace-nowrap">онлайн</span>
          </button>

          <div class="hidden md:block w-px h-5 bg-ink-900/10" />

          <!-- Speed — only on md+ to avoid wrap -->
          <div class="hidden md:flex items-center gap-3 tnum">
            <span class="flex items-center gap-1">
              <ArrowDown :size="11" class="text-ink-500 shrink-0" />
              <span
                class="text-[12.5px] font-medium mono"
                :class="idle ? 'text-ink-400' : 'text-ink-900'"
              >{{ idle ? '—' : bytes(rxRate) + '/с' }}</span>
            </span>
            <span class="flex items-center gap-1">
              <ArrowUp :size="11" class="text-ink-500 shrink-0" />
              <span
                class="text-[12.5px] font-medium mono"
                :class="idle ? 'text-ink-400' : 'text-ink-900'"
              >{{ idle ? '—' : bytes(txRate) + '/с' }}</span>
            </span>
          </div>
        </div>

        <!-- Actions -->
        <div class="flex items-center gap-0.5 sm:gap-1 shrink-0">
          <slot name="actions" />
          <IconButton
            title="Обновить данные"
            aria-label="Обновить данные сейчас"
            :disabled="refreshing"
            @click="refreshAll"
          >
            <Icon name="refresh" :size="17" :class="refreshing && 'animate-spin'" />
          </IconButton>
          <IconButton
            :title="isDark ? 'Светлая тема' : 'Тёмная тема'"
            :aria-label="isDark ? 'Включить светлую тему' : 'Включить тёмную тему'"
            @click="toggleTheme"
          >
            <Icon :name="isDark ? 'sun' : 'moon'" :size="18" />
          </IconButton>
          <IconButton
				title="Расходы"
				aria-label="Расходы на хостинг"
				:class="route.name === 'billing' && 'bg-amber-400/15 text-amber-600'"
				@click="toBilling"
			>
				<ReceiptText :size="18" />
			</IconButton>
			<IconButton
            :title="route.name === 'settings' ? 'На главную' : 'Настройки'"
            :aria-label="route.name === 'settings' ? 'На главную' : 'Настройки'"
            @click="route.name === 'settings' ? toClients() : toSettings()"
          >
            <Icon :name="route.name === 'settings' ? 'x' : 'settings'" :size="18" />
          </IconButton>
          <IconButton
            v-if="auth.requiresPassword && auth.authenticated"
            title="Выйти"
            aria-label="Выйти из панели"
            @click="logout"
          >
            <Icon name="logout" :size="18" />
          </IconButton>
        </div>
      </div>
    </div>
  </header>
</template>
