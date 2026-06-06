<script setup lang="ts">
import { ref, computed } from 'vue'
import { useClientsStore } from '@/stores/clients'
import { useInterval } from '@/composables/useInterval'
import TopBar from '@/components/organisms/TopBar.vue'
import ClientsTable from '@/components/organisms/ClientsTable.vue'
import NewClientModal from '@/components/organisms/NewClientModal.vue'
import QrModal from '@/components/organisms/QrModal.vue'
import ConfigModal from '@/components/organisms/ConfigModal.vue'
import ConfirmDialog from '@/components/molecules/ConfirmDialog.vue'
import EmptyState from '@/components/molecules/EmptyState.vue'
import Button from '@/components/atoms/Button.vue'
import Input from '@/components/atoms/Input.vue'
import Spinner from '@/components/atoms/Spinner.vue'
import Icon from '@/components/atoms/Icon.vue'

const store = useClientsStore()
useInterval(() => store.fetch(true), 3000, { immediate: true, pauseHidden: true })

const query = ref('')
const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return store.items
  return store.items.filter(c =>
    c.name.toLowerCase().includes(q) ||
    c.address.includes(q) ||
    c.publicKey.toLowerCase().includes(q),
  )
})

const newOpen = ref(false)
const newBusy = ref(false)

const qrFor   = ref<string | null>(null)
const cfgFor  = ref<string | null>(null)
const delFor  = ref<string | null>(null)

const nameOf = (id: string | null) => id ? store.items.find(c => c.id === id)?.name : undefined

async function onCreate(name: string) {
  newBusy.value = true
  try { await store.create(name); newOpen.value = false }
  finally { newBusy.value = false }
}
async function confirmDelete() {
  if (!delFor.value) return
  await store.remove(delFor.value)
  delFor.value = null
}
</script>

<template>
  <div class="min-h-full">
    <TopBar>
      <template #actions>
        <Button variant="primary" size="sm" @click="newOpen = true">
          <Icon name="plus" :size="15" /> New client
        </Button>
      </template>
    </TopBar>

    <main class="max-w-6xl mx-auto px-5 py-6 space-y-4">
      <div class="flex items-center justify-between gap-3">
        <div>
          <h1 class="text-[18px] font-semibold text-ink-900">Clients</h1>
          <p class="text-[12.5px] text-ink-500">{{ store.items.length }} total · auto-refreshes every 3s</p>
        </div>
        <div class="w-72">
          <Input v-model="query" size="sm" placeholder="Search by name, IP, or key" />
        </div>
      </div>

      <div v-if="store.loading && !store.items.length" class="card p-10 grid place-items-center">
        <Spinner :size="22" />
      </div>

      <EmptyState
        v-else-if="!store.items.length"
        title="No clients yet"
        description="Create a client to generate a downloadable AmneziaWG configuration and QR code."
      >
        <template #action>
          <Button variant="primary" size="sm" @click="newOpen = true">
            <Icon name="plus" :size="15" /> New client
          </Button>
        </template>
      </EmptyState>

      <EmptyState
        v-else-if="!filtered.length"
        title="Nothing matches"
        :description="`No clients match “${query}”.`"
      />

      <ClientsTable
        v-else
        :clients="filtered"
        @toggle="(id, v) => store.setEnabled(id, v)"
        @remove="id => delFor = id"
        @rename="(id, n) => store.rename(id, n)"
        @show-config="id => cfgFor = id"
        @show-qr="id => qrFor = id"
      />
    </main>

    <NewClientModal :open="newOpen" :busy="newBusy" @close="newOpen = false" @submit="onCreate" />
    <QrModal     :open="!!qrFor"  :client-id="qrFor"  :client-name="nameOf(qrFor)"  @close="qrFor = null" />
    <ConfigModal :open="!!cfgFor" :client-id="cfgFor" :client-name="nameOf(cfgFor)" @close="cfgFor = null" />
    <ConfirmDialog
      :open="!!delFor"
      title="Delete client?"
      :message="`This will revoke access for “${nameOf(delFor)}”. This cannot be undone.`"
      confirm-text="Delete"
      tone="danger"
      @cancel="delFor = null"
      @confirm="confirmDelete"
    />
  </div>
</template>
