import { defineStore } from 'pinia'
import type { Toast, ToastKind } from '@/types'

let nextId = 1

export const useToastStore = defineStore('toasts', {
  state: () => ({ items: [] as Toast[], timers: {} as Record<number, ReturnType<typeof setTimeout>> }),
  actions: {
    push(message: string, kind: ToastKind = 'info', ttl = 3500) {
      const id = nextId++
      this.items.push({ id, kind, message })
      this.timers[id] = setTimeout(() => this.dismiss(id), ttl)
    },
    dismiss(id: number) {
      clearTimeout(this.timers[id])
      delete this.timers[id]
      this.items = this.items.filter(t => t.id !== id)
    },
    success(m: string) { this.push(m, 'success') },
    error(m: string)   { this.push(m, 'danger', 5000) },
    info(m: string)    { this.push(m, 'info') },
  },
})
