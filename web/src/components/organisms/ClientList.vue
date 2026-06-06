<script setup lang="ts">
import { computed } from 'vue'
import type { Client } from '@/types'
import { handshakeFreshness } from '@/lib/format'
import ClientItem from './ClientItem.vue'

const props = defineProps<{ clients: Client[] }>()
defineEmits<{
  (e: 'toggle', id: string, enabled: boolean): void
  (e: 'remove', id: string): void
  (e: 'rename', id: string, name: string): void
  (e: 'show-config', id: string): void
  (e: 'show-qr', id: string): void
}>()

type Group = { key: 'online' | 'offline'; title: string; items: Client[] }

const groups = computed<Group[]>(() => {
  const online: Client[] = []
  const offline: Client[] = []
  for (const c of props.clients) {
    const fresh = c.enabled ? handshakeFreshness(c.latestHandshakeAt) : 'offline'
    if (fresh === 'online' || fresh === 'stale') online.push(c)
    else offline.push(c)
  }
  const out: Group[] = []
  if (online.length)  out.push({ key: 'online',  title: `Active · ${online.length}`,  items: online })
  if (offline.length) out.push({ key: 'offline', title: `Inactive · ${offline.length}`, items: offline })
  return out
})
</script>

<template>
  <div class="space-y-6">
    <section v-for="g in groups" :key="g.key">
      <div class="section-title">{{ g.title }}</div>
      <div class="card overflow-hidden">
        <ClientItem
          v-for="(c, i) in g.items" :key="c.id"
          :client="c"
          :show-divider="i < g.items.length - 1"
          @toggle="v => $emit('toggle', c.id, v)"
          @remove="$emit('remove', c.id)"
          @rename="n => $emit('rename', c.id, n)"
          @show-config="$emit('show-config', c.id)"
          @show-qr="$emit('show-qr', c.id)"
        />
      </div>
    </section>
  </div>
</template>
