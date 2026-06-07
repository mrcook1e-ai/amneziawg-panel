import { defineStore } from 'pinia'
import { api } from '@/lib/api'
import type { OnboardToken } from '@/types'
import { useToastStore } from '@/stores/toasts'

export const useTokensStore = defineStore('tokens', {
  state: () => ({
    items: [] as OnboardToken[],
    loading: false,
    error: '',
  }),
  actions: {
    async fetch(silent = false) {
      if (!silent) this.loading = true
      try {
        this.items = (await api.listTokens()) || []
        this.error = ''
      } catch (e: any) {
        this.error = e?.message || 'Failed to load invites'
      } finally {
        this.loading = false
      }
    },
    async create(body: { name: string; expiresIn: number }) {
      const t = useToastStore()
      try {
        const tok = await api.createToken(body)
        t.success('Инвайт создан')
        await this.fetch(true)
        return tok
      } catch (e: any) {
        t.error(e?.message || 'Не удалось создать инвайт'); throw e
      }
    },
    async revoke(id: string) {
      const t = useToastStore()
      try {
        await api.revokeToken(id)
        t.success('Инвайт отозван')
        await this.fetch(true)
      } catch (e: any) {
        t.error(e?.message || 'Не удалось отозвать'); throw e
      }
    },
  },
})
