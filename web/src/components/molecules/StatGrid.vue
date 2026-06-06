<script setup lang="ts">
import { computed } from 'vue'
import type { Client } from '@/types'
import { bytes, handshakeFreshness } from '@/lib/format'

const props = defineProps<{ clients: Client[] }>()

const total = computed(() => props.clients.length)

const onlineNow = computed(() =>
  props.clients.filter(c => c.enabled && handshakeFreshness(c.latestHandshakeAt) === 'online').length
)

const transferIn  = computed(() => props.clients.reduce((s, c) => s + (c.transferRx || 0), 0))
const transferOut = computed(() => props.clients.reduce((s, c) => s + (c.transferTx || 0), 0))
</script>

<template>
  <div class="grid grid-cols-3 gap-2 sm:gap-3">
    <div class="card p-4">
      <div class="text-[10.5px] uppercase tracking-[0.06em] text-ink-500 font-medium">Online</div>
      <div class="mt-2 flex items-baseline gap-1.5">
        <span class="text-[26px] font-semibold tracking-tight text-ink-900 leading-none tabular-nums">{{ onlineNow }}</span>
        <span class="text-[12px] text-ink-500">of {{ total }}</span>
      </div>
    </div>

    <div class="card p-4">
      <div class="text-[10.5px] uppercase tracking-[0.06em] text-ink-500 font-medium">Inbound</div>
      <div class="mt-2 flex items-baseline gap-1.5">
        <span class="text-[20px] sm:text-[22px] font-semibold tracking-tight text-ink-900 leading-none tabular-nums">{{ bytes(transferIn) }}</span>
      </div>
      <div class="mt-1 text-[11px] text-ink-500">from clients</div>
    </div>

    <div class="card p-4">
      <div class="text-[10.5px] uppercase tracking-[0.06em] text-ink-500 font-medium">Outbound</div>
      <div class="mt-2 flex items-baseline gap-1.5">
        <span class="text-[20px] sm:text-[22px] font-semibold tracking-tight text-ink-900 leading-none tabular-nums">{{ bytes(transferOut) }}</span>
      </div>
      <div class="mt-1 text-[11px] text-ink-500">to clients</div>
    </div>
  </div>
</template>
