<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import type { Client } from '@/types'
import { bytes, relativeTime, handshakeFreshness, stateLabelRu } from '@/lib/format'
import StatusDot from '@/components/atoms/StatusDot.vue'
import Switch from '@/components/atoms/Switch.vue'
import IconButton from '@/components/atoms/IconButton.vue'
import Icon from '@/components/atoms/Icon.vue'
import { ArrowDown, ArrowUp } from 'lucide-vue-next'

const props = defineProps<{ client: Client; showDivider?: boolean }>()
const emit = defineEmits<{
  (e: 'toggle', enabled: boolean): void
  (e: 'remove'): void
  (e: 'show-config'): void
  (e: 'show-qr'): void
}>()

const router = useRouter()
const state = computed(() => props.client.enabled ? handshakeFreshness(props.client.latestHandshakeAt) : 'offline')

const stateText = computed(() => {
  if (!props.client.enabled) return stateLabelRu('disabled')
  return stateLabelRu(state.value)
})

const subtitle = computed(() => {
  if (!props.client.enabled) return 'выключен'
  if (state.value === 'offline') return 'не выходил на связь'
  return `был ${relativeTime(props.client.latestHandshakeAt)}`
})

function open() {
  router.push({ name: 'client', params: { id: props.client.id } })
}
</script>

<template>
  <div
    class="group relative flex items-center gap-4 px-5 py-4 cursor-pointer transition-colors hover:bg-ink-100/60"
    @click="open"
  >
    <!-- Лево: имя + IP -->
    <div class="flex-1 min-w-0">
      <div class="flex items-center gap-2.5">
        <span class="text-[16px] text-ink-900 font-semibold tracking-tight truncate">{{ client.name }}</span>
        <span class="flex items-center gap-1.5 shrink-0">
          <StatusDot v-if="client.enabled" :state="state" />
          <span v-else class="inline-block h-2 w-2 rounded-full bg-ink-300" />
          <span class="eyebrow">{{ stateText }}</span>
        </span>
      </div>
      <div class="mt-1 flex items-center gap-3 text-[11.5px] text-ink-500">
        <span class="mono">{{ client.address }}</span>
        <span class="hidden sm:inline">·</span>
        <span class="hidden sm:inline truncate">{{ subtitle }}</span>
      </div>
    </div>

    <!-- Право: трафик -->
    <div class="hidden sm:flex flex-col items-end text-right shrink-0 mr-1 tnum">
      <span class="text-[13.5px] text-ink-900 font-medium mono flex items-center gap-0.5"><ArrowDown :size="11" class="text-ink-400 shrink-0" /> {{ bytes(client.transferRx) }}</span>
      <span class="text-[11.5px] text-ink-500 mono mt-0.5 flex items-center gap-0.5"><ArrowUp :size="10" class="shrink-0" /> {{ bytes(client.transferTx) }}</span>
    </div>

    <!--
      Действия. На десктопе появляются по hover (чище визуально),
      на тач-устройствах всегда видны — иначе их просто не дотянуться.
    -->
    <div class="flex items-center gap-0.5 shrink-0" @click.stop>
      <div class="flex items-center gap-0.5 opacity-100 sm:opacity-0 sm:group-hover:opacity-100 transition-opacity">
        <IconButton size="sm" title="QR-код"   @click="emit('show-qr')"><Icon name="qrcode" :size="15" /></IconButton>
        <IconButton size="sm" title="Скачать"  @click="emit('show-config')"><Icon name="download" :size="15" /></IconButton>
        <IconButton size="sm" title="Удалить"  @click="emit('remove')"><Icon name="trash" :size="15" /></IconButton>
      </div>
      <div class="pl-1.5">
        <Switch :model-value="client.enabled" @update:model-value="v => emit('toggle', v)" />
      </div>
    </div>
  </div>

  <div v-if="showDivider" class="hairline mx-5" />
</template>
