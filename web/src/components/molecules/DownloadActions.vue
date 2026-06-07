<script setup lang="ts">
/*
  DownloadActions — admin-side row of "get this config to the device".

  Layout: .vpn download (primary) + show-QR + copy vpn:// + a collapsed
  <details> for the legacy .conf format. Rationale: .conf is only useful
  for standard WireGuard clients (which don't carry AmneziaWG's obfuscation
  parameters and therefore won't punch DPI). It's a debug/inspection
  artifact, not an end-user delivery channel — so it lives one click away.
*/
import { ref } from 'vue'
import { api } from '@/lib/api'
import { copy } from '@/lib/clipboard'
import { useToastStore } from '@/stores/toasts'
import Button from '@/components/atoms/Button.vue'
import Icon from '@/components/atoms/Icon.vue'
import QrCarousel from '@/components/molecules/QrCarousel.vue'

const props = defineProps<{
  clientId: string
  clientName?: string
}>()

const toasts = useToastStore()
const qrOpen   = ref(false)
const copied   = ref(false)
const cfgOpen  = ref(false)
const cfgText  = ref('')
const cfgBusy  = ref(false)

async function copyVpn() {
  try {
    const res = await fetch(api.amneziaVpnUrl(props.clientId))
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const text = await res.text()
    if (await copy(text)) {
      copied.value = true
      toasts.success('vpn:// в буфере')
      setTimeout(() => (copied.value = false), 1500)
    }
  } catch (e: any) {
    toasts.error('Не удалось получить vpn://')
  }
}

async function loadConf() {
  if (cfgText.value || cfgBusy.value) return
  cfgBusy.value = true
  try {
    cfgText.value = await api.clientConfig(props.clientId)
  } catch (e: any) {
    toasts.error('Ошибка загрузки .conf')
  } finally {
    cfgBusy.value = false
  }
}

function onCfgToggle(e: Event) {
  const el = e.target as HTMLDetailsElement
  cfgOpen.value = el.open
  if (el.open) loadConf()
}
</script>

<template>
  <div class="space-y-3">
    <!-- Primary actions: .vpn + QR + copy -->
    <div class="flex items-center gap-2">
      <a
        :href="api.amneziaVpnUrl(clientId)"
        :download="`${clientName || 'client'}.vpn`"
        class="btn-primary flex-1 inline-flex items-center justify-center gap-1.5 h-10 text-[13px]">
        <Icon name="download" :size="14" />
        Скачать .vpn
      </a>
      <Button
        variant="secondary"
        size="md"
        class="!h-10 !rounded-2xl !px-3.5"
        :title="`QR-код для ${clientName || 'клиента'}`"
        @click="qrOpen = !qrOpen">
        <Icon name="qrcode" :size="14" />
        {{ qrOpen ? 'Скрыть QR' : 'QR' }}
      </Button>
      <button
        class="h-10 w-10 flex items-center justify-center rounded-2xl transition-all shrink-0"
        :class="copied
          ? 'bg-success/12 text-success'
          : 'text-ink-500 hover:text-ink-900 hover:bg-ink-100 dark:hover:bg-ink-200/50'"
        :title="copied ? 'Скопировано' : 'Скопировать vpn://'"
        :aria-label="copied ? 'Скопировано' : 'Скопировать vpn:// ссылку'"
        @click="copyVpn">
        <Icon :name="copied ? 'check' : 'copy'" :size="15" />
      </button>
    </div>

    <!-- Inline QR carousel — collapses without modal weight. -->
    <Transition
      enter-active-class="transition-[opacity,max-height] duration-200 ease-out overflow-hidden"
      leave-active-class="transition-[opacity,max-height] duration-150 ease-in overflow-hidden"
      enter-from-class="opacity-0 max-h-0"
      enter-to-class="opacity-100 max-h-[480px]"
      leave-from-class="opacity-100 max-h-[480px]"
      leave-to-class="opacity-0 max-h-0">
      <div v-if="qrOpen" class="flex justify-center pt-1">
        <QrCarousel :device-id="clientId" :device-name="clientName" :size="220" />
      </div>
    </Transition>

    <!--
      .conf as debug artifact. Closed by default; we lazy-load the body
      so a card with several DownloadActions doesn't pre-fetch N configs.
    -->
    <details class="rounded-xl bg-ink-100/60 group" @toggle="onCfgToggle">
      <summary class="cursor-pointer list-none flex items-center justify-between px-4 py-2.5 select-none text-[11.5px] text-ink-500 hover:text-ink-700 transition-colors">
        <span class="inline-flex items-center gap-1.5">
          <Icon name="settings" :size="11" />
          .conf для отладки (стандартный WireGuard)
        </span>
        <Icon name="chevron-right" :size="11" class="transition-transform group-open:rotate-90" />
      </summary>
      <div class="px-4 pb-4 pt-1 space-y-2">
        <p class="text-[11px] text-ink-500 leading-snug">
          .conf не содержит обфускации AmneziaWG — может не пробить DPI.
          Полезен только для отладки или нестандартных клиентов.
        </p>
        <div class="flex items-center gap-2">
          <a
            :href="api.configDownloadUrl(clientId)"
            :download="`${clientName || 'client'}.conf`"
            class="btn-secondary inline-flex items-center gap-1.5 h-9 px-3 text-[12px]">
            <Icon name="download" :size="12" />
            Скачать .conf
          </a>
          <span v-if="cfgBusy" class="text-[11px] text-ink-500">загрузка…</span>
        </div>
        <pre
          v-if="cfgText"
          class="text-[10.5px] mono leading-relaxed text-ink-700 bg-ink-50 dark:bg-ink-100 rounded-lg p-3 max-h-48 overflow-auto whitespace-pre-wrap break-all">{{ cfgText }}</pre>
      </div>
    </details>
  </div>
</template>
