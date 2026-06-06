<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Client } from '@/types'
import { bytes, relativeTime, handshakeFreshness } from '@/lib/format'
import StatusDot from '@/components/atoms/StatusDot.vue'
import Switch from '@/components/atoms/Switch.vue'
import IconButton from '@/components/atoms/IconButton.vue'
import Icon from '@/components/atoms/Icon.vue'
import CopyButton from '@/components/molecules/CopyButton.vue'

const props = defineProps<{ client: Client }>()
const emit = defineEmits<{
  (e: 'toggle', enabled: boolean): void
  (e: 'remove'): void
  (e: 'rename', name: string): void
  (e: 'show-config'): void
  (e: 'show-qr'): void
}>()

const state = computed(() => props.client.enabled ? handshakeFreshness(props.client.latestHandshakeAt) : 'offline')
const stateLabel = computed(() => ({
  online: 'Online',
  stale:  'Stale',
  offline: props.client.enabled ? 'Offline' : 'Disabled',
}[state.value]))

const editing = ref(false)
const draft = ref(props.client.name)
function commit() {
  editing.value = false
  const v = draft.value.trim()
  if (v && v !== props.client.name) emit('rename', v)
  else draft.value = props.client.name
}
</script>

<template>
  <tr class="border-t border-ink-100 hover:bg-ink-50/50 transition">
    <td class="px-4 py-3">
      <div class="flex items-center gap-3">
        <StatusDot :state="state" />
        <div class="min-w-0">
          <div v-if="!editing" class="flex items-center gap-2">
            <span class="text-[14px] font-medium text-ink-900 truncate">{{ client.name }}</span>
            <IconButton size="sm" title="Rename" @click="editing = true; draft = client.name"><Icon name="edit" :size="13" /></IconButton>
          </div>
          <input
            v-else
            v-model="draft"
            @keydown.enter="commit"
            @keydown.escape="editing = false; draft = client.name"
            @blur="commit"
            class="h-7 px-2 text-[13px] rounded-md border border-ink-300 focus-ring"
            autofocus
          />
          <div class="text-[11px] text-ink-500 mt-0.5">{{ stateLabel }} · last handshake {{ relativeTime(client.latestHandshakeAt) }}</div>
        </div>
      </div>
    </td>
    <td class="px-4 py-3 whitespace-nowrap">
      <div class="inline-flex items-center gap-1.5 font-mono text-[12.5px] text-ink-700">
        {{ client.address }}
        <CopyButton :value="client.address" title="Copy IP" />
      </div>
    </td>
    <td class="px-4 py-3 text-[12.5px] text-ink-700 tabular-nums whitespace-nowrap">
      <div>↓ {{ bytes(client.transferRx) }}</div>
      <div class="text-ink-500">↑ {{ bytes(client.transferTx) }}</div>
    </td>
    <td class="px-4 py-3 text-right whitespace-nowrap">
      <div class="inline-flex items-center gap-1">
        <IconButton size="sm" title="Show config" @click="emit('show-config')"><Icon name="download" :size="15" /></IconButton>
        <IconButton size="sm" title="Show QR code" @click="emit('show-qr')"><Icon name="qrcode" :size="15" /></IconButton>
        <Switch :model-value="client.enabled" @update:model-value="v => emit('toggle', v)" />
        <IconButton size="sm" tone="danger" title="Delete" @click="emit('remove')"><Icon name="trash" :size="15" /></IconButton>
      </div>
    </td>
  </tr>
</template>
