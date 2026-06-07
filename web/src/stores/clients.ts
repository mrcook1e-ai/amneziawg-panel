import { defineStore } from 'pinia'
import { api } from '@/lib/api'
import type { Client, ClientPatch, CreateClientArgs } from '@/types'
import { useToastStore } from '@/stores/toasts'

export const useClientsStore = defineStore('clients', {
  state: () => ({
    items: [] as Client[],
    loading: false,
    error: '' as string,
    lastFetchedAt: 0,
  }),
  getters: {
    byId: (s) => (id: string) => s.items.find((c) => c.id === id) || null,
  },
  actions: {
    async fetch(silent = false) {
      if (!silent) this.loading = true
      try {
        this.items = await api.listClients()
        this.error = ''
        this.lastFetchedAt = Date.now()
      } catch (e: any) {
        this.error = e?.message || 'Failed to load'
      } finally {
        this.loading = false
      }
    },
    async create(args: CreateClientArgs) {
      const t = useToastStore()
      try { await api.createClient(args); t.success('Клиент создан'); await this.fetch(true) }
      catch (e: any) { t.error(e?.message || 'Ошибка создания'); throw e }
    },
    async move(id: string, profileId: string) {
      const t = useToastStore()
      try { await api.moveClient(id, profileId); t.success('Клиент перемещён'); await this.fetch(true) }
      catch (e: any) { t.error(e?.message || 'Не удалось переместить'); throw e }
    },
    async remove(id: string) {
      const t = useToastStore()
      // Optimistic: drop from local list immediately so the UI feels instant.
      // On server error, put it back at the original index.
      const idx = this.items.findIndex(c => c.id === id)
      const backup = idx >= 0 ? this.items[idx] : null
      if (idx >= 0) this.items.splice(idx, 1)
      try {
        await api.deleteClient(id)
        t.success('Клиент удалён')
        this.fetch(true) // background reconcile, no await
      } catch (e: any) {
        if (backup) this.items.splice(idx, 0, backup)
        t.error(e?.message || 'Ошибка удаления')
      }
    },
    async setEnabled(id: string, enabled: boolean) {
      const t = useToastStore()
      try {
        enabled ? await api.enableClient(id) : await api.disableClient(id)
        await this.fetch(true)
      } catch (e: any) { t.error(e?.message || 'Ошибка') }
    },
    async rename(id: string, name: string) {
      const t = useToastStore()
      try {
        await api.renameClient(id, name)
        // Optimistic local update so the new name shows before refetch lands.
        const i = this.items.findIndex(c => c.id === id)
        if (i >= 0) this.items[i] = { ...this.items[i], name }
        t.success('Переименовано')
        this.fetch(true)
      } catch (e: any) {
        t.error(e?.message || 'Не удалось переименовать')
      }
    },
    async patch(id: string, patch: ClientPatch) {
      const t = useToastStore()
      try {
        const updated = await api.patchClient(id, patch)
        const i = this.items.findIndex(c => c.id === id)
        if (i >= 0) this.items[i] = { ...this.items[i], ...updated }
        t.success('Сохранено')
        return updated
      } catch (e: any) {
        t.error(e?.message || 'Ошибка'); throw e
      }
    },
  },
})
