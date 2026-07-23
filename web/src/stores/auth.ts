import { defineStore } from 'pinia'
import { api, ApiError } from '@/lib/api'
import { startStream, stopStream } from '@/lib/stream'

const DEV_BYPASS =
  import.meta.env.DEV && import.meta.env.VITE_DEV_BYPASS_AUTH === '1'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    ready: false,
    requiresPassword: false,
    authenticated: false,
  }),
  actions: {
    async refresh() {
      if (DEV_BYPASS) {
        this.requiresPassword = false
        this.authenticated = true
        this.ready = true
        return
      }
      try {
        const s = await api.session()
        this.requiresPassword = s.requiresPassword
        this.authenticated = s.authenticated
      } catch {
        this.authenticated = false
      } finally {
        this.ready = true
        if (this.authenticated) startStream()
      }
    },
    async login(password: string) {
      if (DEV_BYPASS) {
        this.authenticated = true
        return true
      }
      try {
        await api.login(password)
        this.authenticated = true
        startStream()
        return true
      } catch (e) {
        if (e instanceof ApiError) return false
        throw e
      }
    },
    async logout() {
      if (DEV_BYPASS) {
        this.authenticated = true
        return
      }
      try { await api.logout() } catch { /* ignore */ }
      this.authenticated = false
      stopStream()
    },
  },
})
