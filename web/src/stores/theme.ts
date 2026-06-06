import { defineStore } from 'pinia'

export type ThemeMode = 'auto' | 'light' | 'dark'

const KEY = 'awgp:theme'
const mql = typeof window !== 'undefined' ? window.matchMedia('(prefers-color-scheme: dark)') : null

function systemDark(): boolean { return mql?.matches ?? false }

function applyClass(mode: ThemeMode) {
  const dark = mode === 'dark' || (mode === 'auto' && systemDark())
  document.documentElement.classList.toggle('dark', dark)
}

function read(): ThemeMode {
  const v = localStorage.getItem(KEY)
  return v === 'light' || v === 'dark' ? v : 'auto'
}

// Run once at module load — eliminates the white-flash before App mounts.
if (typeof document !== 'undefined') applyClass(read())

export const useThemeStore = defineStore('theme', {
  state: () => ({ mode: read() as ThemeMode }),
  getters: {
    resolved: (s): 'light' | 'dark' =>
      s.mode === 'dark' || (s.mode === 'auto' && systemDark()) ? 'dark' : 'light',
  },
  actions: {
    set(mode: ThemeMode) {
      this.mode = mode
      localStorage.setItem(KEY, mode)
      applyClass(mode)
    },
    /** Subscribe to OS theme changes so `auto` follows them live. */
    bindSystem() {
      if (!mql) return
      const handler = () => { if (this.mode === 'auto') applyClass('auto') }
      mql.addEventListener('change', handler)
    },
  },
})
