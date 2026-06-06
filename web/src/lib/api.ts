import type { Client, ServerInfo, SessionState } from '@/types'

export class ApiError extends Error {
  constructor(public status: number, message: string) { super(message) }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    credentials: 'same-origin',
    headers: { 'Accept': 'application/json', ...(init.body ? { 'Content-Type': 'application/json' } : {}), ...init.headers },
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
  session: () => request<SessionState>('/api/session'),
  login: (password: string) => request<{ success: boolean }>('/api/session', { method: 'POST', body: JSON.stringify({ password }) }),
  logout: () => request<{ success: boolean }>('/api/session', { method: 'DELETE' }),

  serverInfo:        () => request<ServerInfo>('/api/wireguard/server/'),
  regenerateMagic:   () => request<ServerInfo>('/api/wireguard/server/regenerate-magic', { method: 'POST' }),
  restartInterface:  () => request<{ success: boolean }>('/api/wireguard/server/restart', { method: 'POST' }),
  resetClients:      () => request<{ success: boolean }>('/api/wireguard/server/reset-clients', { method: 'POST' }),

  listClients: () => request<Client[]>('/api/wireguard/client/'),
  createClient: (name: string) => request<{ success: boolean }>('/api/wireguard/client/', { method: 'POST', body: JSON.stringify({ name }) }),
  deleteClient: (id: string) => request<{ success: boolean }>(`/api/wireguard/client/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  enableClient: (id: string) => request<{ success: boolean }>(`/api/wireguard/client/${encodeURIComponent(id)}/enable`, { method: 'POST' }),
  disableClient: (id: string) => request<{ success: boolean }>(`/api/wireguard/client/${encodeURIComponent(id)}/disable`, { method: 'POST' }),
  renameClient: (id: string, name: string) => request<{ success: boolean }>(`/api/wireguard/client/${encodeURIComponent(id)}/name`, { method: 'PUT', body: JSON.stringify({ name }) }),
  setAddress: (id: string, address: string) => request<{ success: boolean }>(`/api/wireguard/client/${encodeURIComponent(id)}/address`, { method: 'PUT', body: JSON.stringify({ address }) }),
  clientConfig: (id: string) => request<string>(`/api/wireguard/client/${encodeURIComponent(id)}/configuration`),
  clientVPN:    (id: string) => request<string>(`/api/wireguard/client/${encodeURIComponent(id)}/amnezia.vpn`),
  qrUrl:        (id: string) => `/api/wireguard/client/${encodeURIComponent(id)}/qrcode.svg`,
  vpnQrUrl:     (id: string) => `/api/wireguard/client/${encodeURIComponent(id)}/amnezia-qrcode.svg`,
  configDownloadUrl: (id: string) => `/api/wireguard/client/${encodeURIComponent(id)}/configuration`,
  vpnDownloadUrl:    (id: string) => `/api/wireguard/client/${encodeURIComponent(id)}/amnezia.vpn`,
}
