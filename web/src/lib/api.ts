import type {
  Client, ServerInfo, SessionState,
  Overview, Series, ClientStats, AppEvent, ClientPatch,
} from '@/types'

export class ApiError extends Error {
  constructor(public status: number, message: string) { super(message) }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    credentials: 'same-origin',
    headers: {
      'Accept': 'application/json',
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
      ...init.headers,
    },
    ...init,
  })
  if (!res.ok) {
    let msg = res.statusText
    try { const data = await res.json(); if (data?.error) msg = data.error } catch { /* ignore */ }
    throw new ApiError(res.status, msg)
  }
  const ct = res.headers.get('content-type') || ''
  if (ct.includes('application/json')) return res.json() as Promise<T>
  return res.text() as unknown as Promise<T>
}

export const api = {
  // ─── Session ───
  session: () => request<SessionState>('/api/session'),
  login: (password: string) =>
    request<{ success: boolean }>('/api/session', { method: 'POST', body: JSON.stringify({ password }) }),
  logout: () => request<{ success: boolean }>('/api/session', { method: 'DELETE' }),

  // ─── Server ───
  serverInfo:       () => request<ServerInfo>('/api/wireguard/server/'),
  regenerateMagic:  () => request<ServerInfo>('/api/wireguard/server/regenerate-magic', { method: 'POST' }),
  restartInterface: () => request<{ success: boolean }>('/api/wireguard/server/restart', { method: 'POST' }),
  resetClients:     () => request<{ success: boolean }>('/api/wireguard/server/reset-clients', { method: 'POST' }),

  // ─── Clients ───
  listClients:  () => request<Client[]>('/api/wireguard/client/'),
  createClient: (name: string) =>
    request<{ success: boolean }>('/api/wireguard/client/', { method: 'POST', body: JSON.stringify({ name }) }),
  deleteClient:  (id: string) => request<{ success: boolean }>(`/api/wireguard/client/${enc(id)}`, { method: 'DELETE' }),
  enableClient:  (id: string) => request<{ success: boolean }>(`/api/wireguard/client/${enc(id)}/enable`, { method: 'POST' }),
  disableClient: (id: string) => request<{ success: boolean }>(`/api/wireguard/client/${enc(id)}/disable`, { method: 'POST' }),
  renameClient:  (id: string, name: string) =>
    request<{ success: boolean }>(`/api/wireguard/client/${enc(id)}/name`, { method: 'PUT', body: JSON.stringify({ name }) }),
  setAddress: (id: string, address: string) =>
    request<{ success: boolean }>(`/api/wireguard/client/${enc(id)}/address`, { method: 'PUT', body: JSON.stringify({ address }) }),
  patchClient: (id: string, patch: ClientPatch) =>
    request<Client>(`/api/wireguard/client/${enc(id)}`, { method: 'PATCH', body: JSON.stringify(patch) }),

  clientConfig: (id: string) => request<string>(`/api/wireguard/client/${enc(id)}/configuration`),
  clientVPN:    (id: string) => request<string>(`/api/wireguard/client/${enc(id)}/amnezia.vpn`),
  qrUrl:        (id: string) => `/api/wireguard/client/${enc(id)}/qrcode.svg`,
  vpnQrUrl:     (id: string) => `/api/wireguard/client/${enc(id)}/amnezia-qrcode.svg`,
  configDownloadUrl: (id: string) => `/api/wireguard/client/${enc(id)}/configuration`,
  vpnDownloadUrl:    (id: string) => `/api/wireguard/client/${enc(id)}/amnezia.vpn`,

  // ─── Stats / events ───
  overview:    () => request<Overview>('/api/stats/overview'),
  series:      (range = '24h') => request<Series>(`/api/stats/series?range=${encodeURIComponent(range)}`),
  clientStats: (id: string) => request<ClientStats>(`/api/wireguard/client/${enc(id)}/stats`),
  clientEvents:(id: string, limit = 30) =>
    request<AppEvent[] | null>(`/api/wireguard/client/${enc(id)}/events?limit=${limit}`),
  events: (limit = 30) => request<AppEvent[] | null>(`/api/events?limit=${limit}`),

  // ─── Admin ───
  backupUrl: () => '/api/backup',
  importClient: (body: {
    name: string; publicKey: string;
    privateKey?: string; preSharedKey?: string; address?: string; notes?: string;
  }) => request<Client>('/api/wireguard/client/import', {
    method: 'POST', body: JSON.stringify(body),
  }),
  restore: async (file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    const res = await fetch('/api/restore', { method: 'POST', credentials: 'same-origin', body: fd })
    if (!res.ok) {
      let msg = res.statusText
      try { const d = await res.json(); if (d?.error) msg = d.error } catch { /* ignore */ }
      throw new ApiError(res.status, msg)
    }
    return res.json() as Promise<{ success: boolean; restored: string[] }>
  },
}

function enc(s: string) { return encodeURIComponent(s) }
