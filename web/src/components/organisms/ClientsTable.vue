<script setup lang="ts">
import type { Client } from '@/types'
import ClientRow from './ClientRow.vue'

defineProps<{ clients: Client[] }>()
defineEmits<{
  (e: 'toggle', id: string, enabled: boolean): void
  (e: 'remove', id: string): void
  (e: 'rename', id: string, name: string): void
  (e: 'show-config', id: string): void
  (e: 'show-qr', id: string): void
}>()
</script>

<template>
  <div class="card overflow-hidden">
    <table class="w-full">
      <thead>
        <tr class="text-left text-[11px] uppercase tracking-wider text-ink-500 bg-ink-50/60">
          <th class="px-4 py-2.5 font-medium">Client</th>
          <th class="px-4 py-2.5 font-medium">Address</th>
          <th class="px-4 py-2.5 font-medium">Transfer</th>
          <th class="px-4 py-2.5 font-medium text-right">Actions</th>
        </tr>
      </thead>
      <tbody>
        <ClientRow
          v-for="c in clients" :key="c.id" :client="c"
          @toggle="v => $emit('toggle', c.id, v)"
          @remove="$emit('remove', c.id)"
          @rename="n => $emit('rename', c.id, n)"
          @show-config="$emit('show-config', c.id)"
          @show-qr="$emit('show-qr', c.id)"
        />
      </tbody>
    </table>
  </div>
</template>
