import { defineStore } from 'pinia'
import { api } from '@/lib/api'
import type { Overview, Series, AppEvent } from '@/types'

// Per-client live throughput from the SSE tick. Updates ~1×/sec.
export interface LiveClient {
  rxRate: number
  txRate: number
  online: boolean
  ts: number
}

/*
  Stats store.

  fetch() — раз в N секунд достаём агрегаты (overview / series24h / events).
            Эти запросы дёшевы, но и не мгновенные — это «медленный слой».
  Realtime — SSE-тик (см. lib/stream.ts) обновляет live.* мгновенно. Top-bar и
  страница клиента читают `live`, а не сырое overview, чтобы цифры не лагали.
*/
export const useStatsStore = defineStore('stats', {
  state: () => ({
    overview:   null as Overview | null,
    series24h:  null as Series | null,
    events:     [] as AppEvent[],
    loaded:     false,
    lastError:  '' as string,

    // SSE live state. liveTs = 0 значит "ещё нет тика".
    liveTs:     0,
    liveRxRate: 0,
    liveTxRate: 0,
    liveOnline: 0,
    liveByClient: {} as Record<string, LiveClient>,
    streamConnected: false,
  }),
  actions: {
    async fetch() {
      try {
        const [ov, ser, ev] = await Promise.all([
          api.overview(),
          api.series('24h'),
          api.events(20),
        ])
        this.overview  = ov
        this.series24h = ser
        this.events    = ev ?? []
        this.loaded    = true
        this.lastError = ''
      } catch (e: any) {
        this.lastError = e?.message || 'stats failed'
      }
    },

    applyTick(t: {
      ts: number; rxRate: number; txRate: number; online: number
      clients: { id: string; rxRate: number; txRate: number; online: boolean }[]
    }) {
      this.liveTs = t.ts
      this.liveRxRate = t.rxRate
      this.liveTxRate = t.txRate
      this.liveOnline = t.online
      const next: Record<string, LiveClient> = {}
      for (const c of t.clients) {
        next[c.id] = { rxRate: c.rxRate, txRate: c.txRate, online: c.online, ts: t.ts }
      }
      this.liveByClient = next
    },

    pushEvent(e: AppEvent) {
      // dedupe по id — иначе при первом подключении и быстром polled fetch
      // можем словить дубль.
      if (this.events.some(x => x.id === e.id)) return
      this.events = [e, ...this.events].slice(0, 50)
    },

    setStreamConnected(v: boolean) { this.streamConnected = v },
  },
})
