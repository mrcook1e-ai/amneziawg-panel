const UNITS = ['B', 'KB', 'MB', 'GB', 'TB']
export function bytes(n: number): string {
  if (!n || n < 0) return '0 B'
  const i = Math.min(UNITS.length - 1, Math.floor(Math.log(n) / Math.log(1024)))
  const v = n / Math.pow(1024, i)
  return `${v < 10 ? v.toFixed(2) : v < 100 ? v.toFixed(1) : Math.round(v)} ${UNITS[i]}`
}

export function relativeTime(iso: string | null): string {
  if (!iso) return 'never'
  const t = new Date(iso).getTime()
  if (Number.isNaN(t)) return 'never'
  const diff = Math.max(0, (Date.now() - t) / 1000)
  if (diff < 5) return 'just now'
  if (diff < 60) return `${Math.floor(diff)}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}

export function handshakeFreshness(iso: string | null): 'online' | 'stale' | 'offline' {
  if (!iso) return 'offline'
  const diff = (Date.now() - new Date(iso).getTime()) / 1000
  if (diff < 180) return 'online'
  if (diff < 900) return 'stale'
  return 'offline'
}
