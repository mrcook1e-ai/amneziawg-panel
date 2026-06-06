const UNITS = ['Б', 'КБ', 'МБ', 'ГБ', 'ТБ']

export function bytes(n: number): string {
  if (!n || n < 0) return '0 Б'
  const i = Math.min(UNITS.length - 1, Math.floor(Math.log(n) / Math.log(1024)))
  const v = n / Math.pow(1024, i)
  return `${v < 10 ? v.toFixed(2) : v < 100 ? v.toFixed(1) : Math.round(v)} ${UNITS[i]}`
}

export function bytesParts(n: number): { value: string; unit: string } {
  if (!n || n < 0) return { value: '0', unit: 'Б' }
  const i = Math.min(UNITS.length - 1, Math.floor(Math.log(n) / Math.log(1024)))
  const v = n / Math.pow(1024, i)
  return {
    value: v < 10 ? v.toFixed(2) : v < 100 ? v.toFixed(1) : Math.round(v).toString(),
    unit: UNITS[i],
  }
}

export function bytesPerSec(n: number): string {
  return bytes(n) + '/с'
}

// «5 мин назад», «3 часа назад», «вчера», «12 янв». Без слов-наполнителей.
export function relativeTime(iso: string | null): string {
  if (!iso) return 'никогда'
  const t = new Date(iso).getTime()
  if (Number.isNaN(t)) return 'никогда'
  const diff = Math.max(0, (Date.now() - t) / 1000)
  if (diff < 10) return 'сейчас'
  if (diff < 60) return `${Math.floor(diff)} сек назад`
  if (diff < 3600) return `${Math.floor(diff / 60)} мин назад`
  if (diff < 86400) return `${Math.floor(diff / 3600)} ч назад`
  if (diff < 86400 * 2) return 'вчера'
  if (diff < 86400 * 7) return `${Math.floor(diff / 86400)} дн назад`
  const d = new Date(iso)
  return d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' })
}

export function handshakeFreshness(iso: string | null): 'online' | 'stale' | 'offline' {
  if (!iso) return 'offline'
  const diff = (Date.now() - new Date(iso).getTime()) / 1000
  if (diff < 180) return 'online'
  if (diff < 900) return 'stale'
  return 'offline'
}

export const stateLabelRu = (s: 'online' | 'stale' | 'offline' | 'disabled'): string => ({
  online:   'онлайн',
  stale:    'давно был',
  offline:  'офлайн',
  disabled: 'выключен',
}[s])

export function eventTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const now = new Date()
  const sameDay = d.toDateString() === now.toDateString()
  const hm = d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', hour12: false })
  if (sameDay) return hm
  const md = d.toLocaleDateString('ru-RU', { month: 'short', day: 'numeric' })
  return `${md} · ${hm}`
}
