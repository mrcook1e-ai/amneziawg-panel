<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Client } from '@/types'
import { bytes, relativeTime, handshakeFreshness } from '@/lib/format'
import Avatar from '@/components/atoms/Avatar.vue'
import Switch from '@/components/atoms/Switch.vue'
import IconButton from '@/components/atoms/IconButton.vue'
import Icon from '@/components/atoms/Icon.vue'

const props = defineProps<{ client: Client; showDivider?: boolean }>()
const emit = defineEmits<{
  (e: 'toggle', enabled: boolean): void
  (e: 'remove'): void
  (e: 'rename', name: string): void
  (e: 'show-config'): void
  (e: 'show-qr'): void
}>()

const state = computed(() => props.client.enabled ? handshakeFreshness(props.client.latestHandshakeAt) : 'offline')
const subtitle = computed(() => {
  const parts = [props.client.address]
  if (props.client.enabled) {
    parts.push(`handshake ${relativeTime(props.client.latestHandshakeAt)}`)
  } else {
    parts.push('disabled')
  }
  return parts.join(' · ')
})

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
  <div class="group relative">
    <div class="flex items-center gap-3.5 px-4 py-3">
      <Avatar :name="client.name" :state="state" :size="40" />

      <div class="flex-1 min-w-0">
        <div v-if="!editing" class="flex items-center gap-1.5">
          <span class="text-[15px] font-semibold text-ink-900 truncate">{{ client.name }}</span>
          <button
            class="opacity-0 group-hover:opacity-100 text-ink-400 hover:text-ink-700 transition"
            @click="editing = true; draft = client.name"
            aria-label="Rename"
          >
            <Icon name="edit" :size="13" />
          </button>
        </div>
        <input
          v-else
          v-model="draft"
          @keydown.enter="commit"
          @keydown.escape="editing = false; draft = client.name"
          @blur="commit"
          class="h-7 px-2 text-[14px] font-semibold rounded-md border border-ink-300 focus-ring -ml-2"
          autofocus
        />
        <div class="mt-0.5 text-[12px] text-ink-500 truncate font-mono">{{ subtitle }}</div>
      </div>

      <div class="hidden sm:block text-right tabular-nums shrink-0">
        <div class="text-[12.5px] text-ink-800">↓ {{ bytes(client.transferRx) }}</div>
        <div class="text-[11px] text-ink-500 mt-0.5">↑ {{ bytes(client.transferTx) }}</div>
      </div>

      <div class="flex items-center gap-0.5 shrink-0 pl-1">
        <IconButton size="sm" title="QR code" @click="emit('show-qr')"><Icon name="qrcode" :size="15" /></IconButton>
        <IconButton size="sm" title="Show config" @click="emit('show-config')"><Icon name="download" :size="15" /></IconButton>
        <div class="px-1.5"><Switch :model-value="client.enabled" @update:model-value="v => emit('toggle', v)" /></div>
        <IconButton size="sm" tone="danger" title="Delete" @click="emit('remove')"><Icon name="trash" :size="15" /></IconButton>
      </div>
    </div>

    <div v-if="showDivider" class="ml-[68px] mr-4 border-t border-ink-100/80" />
  </div>
</template>
