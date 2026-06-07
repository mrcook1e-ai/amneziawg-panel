import { defineStore } from 'pinia'
import { api } from '@/lib/api'
import type { Subscriber } from '@/types'
import { useToastStore } from '@/stores/toasts'

export const useSubscribersStore = defineStore('subscribers', {
  state: () => ({
    items: [] as Subscriber[],
    loading: false,
    error: '',
  }),
  getters: {
    byId: (s) => (id: string) => s.items.find(x => x.id === id) || null,
  },
  actions: {
    async fetch(silent = false) {
      if (!silent) this.loading = true
      try {
        this.items = (await api.listSubscribers()) || []
        this.error = ''
      } catch (e: any) {
        this.error = e?.message || 'Failed to load subscribers'
      } finally {
        this.loading = false
      }
    },
    async create(body: { name: string; notes?: string }) {
      const t = useToastStore()
      try {
        const s = await api.createSubscriber(body)
        t.success(`Клиент «${s.name}» создан`)
        await this.fetch(true)
        return s
      } catch (e: any) {
        t.error(e?.message || 'Не удалось создать клиента'); throw e
      }
    },
    async patch(id: string, body: { name?: string; notes?: string }) {
      const t = useToastStore()
      try {
        const s = await api.patchSubscriber(id, body)
        t.success('Сохранено')
        await this.fetch(true)
        return s
      } catch (e: any) {
        t.error(e?.message || 'Не удалось сохранить'); throw e
      }
    },
    async regenerateToken(id: string) {
      const t = useToastStore()
      try {
        const s = await api.regenerateSubscriberToken(id)
        t.success('Ссылка кабинета обновлена — старая больше не работает')
        await this.fetch(true)
        return s
      } catch (e: any) {
        t.error(e?.message || 'Ошибка'); throw e
      }
    },
    async remove(id: string) {
      const t = useToastStore()
      // Optimistic remove — restore on failure.
      const idx = this.items.findIndex(x => x.id === id)
      const backup = idx >= 0 ? this.items[idx] : null
      if (idx >= 0) this.items.splice(idx, 1)
      try {
        await api.deleteSubscriber(id)
        t.success('Клиент и все его устройства удалены')
        this.fetch(true)
      } catch (e: any) {
        if (backup) this.items.splice(idx, 0, backup)
        t.error(e?.message || 'Не удалось удалить')
        throw e
      }
    },
  },
})
