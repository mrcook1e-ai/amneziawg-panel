import { defineStore } from 'pinia'
import { api } from '@/lib/api'
import type { ProfileInfo, ProfileCreateBody, ProfilePatchBody } from '@/types'
import { useToastStore } from '@/stores/toasts'

const LAST_KEY = 'amneziaPanel.lastProfileId'

export const useProfilesStore = defineStore('profiles', {
  state: () => ({
    items: [] as ProfileInfo[],
    loading: false,
    error: '',
    lastUsedId: localStorage.getItem(LAST_KEY) || '',
  }),
  getters: {
    byId: (s) => (id: string) => s.items.find(p => p.id === id) || null,
    defaultId(): string {
      if (this.lastUsedId && this.items.some(p => p.id === this.lastUsedId)) return this.lastUsedId
      return this.items[0]?.id || ''
    },
  },
  actions: {
    async fetch(silent = false) {
      if (!silent) this.loading = true
      try {
        this.items = await api.listProfiles()
        this.error = ''
      } catch (e: any) {
        this.error = e?.message || 'Failed to load profiles'
      } finally {
        this.loading = false
      }
    },
    rememberLastUsed(id: string) {
      this.lastUsedId = id
      try { localStorage.setItem(LAST_KEY, id) } catch { /* ignore */ }
    },
    async create(body: ProfileCreateBody) {
      const t = useToastStore()
      try {
        const p = await api.createProfile(body)
        t.success(`Профиль «${p.name}» создан`)
        await this.fetch(true)
        return p
      } catch (e: any) {
        t.error(e?.message || 'Не удалось создать профиль'); throw e
      }
    },
    async patch(id: string, body: ProfilePatchBody) {
      const t = useToastStore()
      try {
        const p = await api.patchProfile(id, body)
        t.success('Профиль сохранён')
        await this.fetch(true)
        return p
      } catch (e: any) {
        t.error(e?.message || 'Не удалось сохранить'); throw e
      }
    },
    async remove(id: string) {
      const t = useToastStore()
      try {
        await api.deleteProfile(id)
        t.success('Профиль удалён')
        await this.fetch(true)
      } catch (e: any) {
        t.error(e?.message || 'Не удалось удалить'); throw e
      }
    },
    async restart(id: string) {
      const t = useToastStore()
      try {
        await api.restartProfile(id)
        t.success('Интерфейс перезапущен')
      } catch (e: any) {
        t.error(e?.message || 'Ошибка'); throw e
      }
    },
  },
})
