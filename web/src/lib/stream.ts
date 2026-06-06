import { useStatsStore } from '@/stores/stats'
import { useClientsStore } from '@/stores/clients'
import type { AppEvent } from '@/types'

/*
  Тонкий SSE-клиент. Один EventSource на сессию, держится открытым до
  логаута/закрытия вкладки. EventSource сам делает reconnect с backoff —
  ничего своего не пишем, только обрабатываем три события: hello, tick, audit.

  Не подключаемся, пока пользователь не залогинен — иначе на стейдже логина
  получим 401 и постоянные реконнекты.
*/

let es: EventSource | null = null

export function startStream() {
  if (es) return
  const stats = useStatsStore()
  const clients = useClientsStore()

  es = new EventSource('/api/stream', { withCredentials: true })

  es.addEventListener('open', () => stats.setStreamConnected(true))
  es.addEventListener('error', () => stats.setStreamConnected(false))

  es.addEventListener('hello', () => stats.setStreamConnected(true))

  es.addEventListener('tick', (e: MessageEvent) => {
    try {
      stats.applyTick(JSON.parse(e.data))
    } catch { /* ignore */ }
  })

  es.addEventListener('audit', (e: MessageEvent) => {
    try {
      const ev = JSON.parse(e.data) as AppEvent
      stats.pushEvent(ev)
      // Лайв-события клиентов: создание/удаление меняют список — дёрнем
      // его сразу, чтобы UI отразил без 3-секундной задержки.
      if (
        ev.kind === 'client.created' ||
        ev.kind === 'client.deleted' ||
        ev.kind === 'client.enabled' ||
        ev.kind === 'client.disabled' ||
        ev.kind === 'client.renamed' ||
        ev.kind === 'client.expired' ||
        ev.kind === 'server.reset_clients'
      ) {
        void clients.fetch(true)
      }
    } catch { /* ignore */ }
  })
}

export function stopStream() {
  if (!es) return
  es.close()
  es = null
  try { useStatsStore().setStreamConnected(false) } catch { /* ignore */ }
}
