import { defineStore } from 'pinia'
import { api, ApiError } from '@/lib/api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    ready: false,
    requiresPassword: false,
    authenticated: false,
  }),
  actions: {
    async refresh() {
      try {
        const s = await api.session()
        this.requiresPassword = s.requiresPassword
        this.authenticated = s.authenticated
      } catch {
        this.authenticated = false
      } finally {
        this.ready = true
      }
    },
    async login(password: string) {
      try {
        await api.login(password)
        this.authenticated = true
        return true
      } catch (e) {
        if (e instanceof ApiError) return false
        throw e
      }
    },
    async logout() {
      try { await api.logout() } catch { /* ignore */ }
      this.authenticated = false
    },
  },
})
