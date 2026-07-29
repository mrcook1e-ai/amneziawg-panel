import type {
  Client, SessionState,
  Overview, Series, ClientStats, AppEvent, ClientPatch,
  ProfileInfo, ProfileCreateBody, ProfilePatchBody, CreateClientArgs,
	 Subscriber, CabinetView, AddDeviceResult, BillingRole,
	 BillingCycle, BillingSummary, CabinetBillingSummary, BillingPreviewLine,
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
  session: () => request<SessionState>('/api/session'),
  login: (password: string) =>
    request<{ success: boolean }>('/api/session', { method: 'POST', body: JSON.stringify({ password }) }),
  logout: () => request<{ success: boolean }>('/api/session', { method: 'DELETE' }),

  // Profiles
  listProfiles: () => request<ProfileInfo[]>('/api/profiles/'),
  getProfile:   (id: string) => request<ProfileInfo>(`/api/profiles/${enc(id)}`),
  createProfile:(body: ProfileCreateBody) =>
    request<ProfileInfo>('/api/profiles/', { method: 'POST', body: JSON.stringify(body) }),
  patchProfile: (id: string, body: ProfilePatchBody) =>
    request<ProfileInfo>(`/api/profiles/${enc(id)}`, { method: 'PATCH', body: JSON.stringify(body) }),
  deleteProfile:(id: string) =>
    request<{ success: boolean }>(`/api/profiles/${enc(id)}`, { method: 'DELETE' }),
  restartProfile: (id: string) =>
    request<{ success: boolean }>(`/api/profiles/${enc(id)}/restart`, { method: 'POST' }),

  // Server-wide actions (apply across all profiles)
  resetClients: () => request<{ success: boolean }>('/api/wireguard/server/reset-clients', { method: 'POST' }),
  factoryReset: () => request<{ success: boolean }>('/api/wireguard/server/factory-reset', { method: 'POST' }),

  // Clients
  listClients:  () => request<Client[]>('/api/wireguard/client/'),
  createClient: (args: CreateClientArgs) =>
    request<{ success: boolean }>('/api/wireguard/client/', { method: 'POST', body: JSON.stringify(args) }),
  moveClient: (id: string, profileId: string) =>
    request<{ success: boolean }>(`/api/wireguard/client/${enc(id)}/profile`, { method: 'PATCH', body: JSON.stringify({ profileId }) }),
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
  qrUrl:        (id: string) => `/api/wireguard/client/${enc(id)}/qrcode.svg`,
  configDownloadUrl: (id: string) => `/api/wireguard/client/${enc(id)}/configuration`,

  overview:    () => request<Overview>('/api/stats/overview'),
  series:      (range = '24h') => request<Series>(`/api/stats/series?range=${encodeURIComponent(range)}`),
  clientStats: (id: string) => request<ClientStats>(`/api/wireguard/client/${enc(id)}/stats`),
  clientEvents:(id: string, limit = 30) =>
    request<AppEvent[] | null>(`/api/wireguard/client/${enc(id)}/events?limit=${limit}`),
  events: (limit = 30) => request<AppEvent[] | null>(`/api/events?limit=${limit}`),

  // Subscribers (admin)
  listSubscribers:  () => request<Subscriber[]>('/api/subscribers/'),
  getSubscriber:    (id: string) => request<Subscriber>(`/api/subscribers/${enc(id)}`),
	 createSubscriber: (body: { name: string; notes?: string; billingRole?: BillingRole }) =>
    request<Subscriber>('/api/subscribers/', { method: 'POST', body: JSON.stringify(body) }),
	 patchSubscriber: (id: string, body: { name?: string; notes?: string; billingRole?: BillingRole }) =>
    request<Subscriber>(`/api/subscribers/${enc(id)}`, { method: 'PATCH', body: JSON.stringify(body) }),
  regenerateSubscriberToken: (id: string) =>
    request<Subscriber>(`/api/subscribers/${enc(id)}/regenerate-token`, { method: 'POST' }),
  deleteSubscriber: (id: string) =>
    request<{ success: boolean }>(`/api/subscribers/${enc(id)}`, { method: 'DELETE' }),

  subscriberStats: (id: string) => request<ClientStats>(`/api/subscribers/${enc(id)}/stats`),

	 // Shared hosting expenses (admin)
	 billingCycles: () => request<BillingCycle[]>('/api/billing/cycles'),
	 billingCycle: (id: number) => request<BillingCycle>(`/api/billing/cycles/${id}`),
	 billingCyclePreview: (id: number) => request<BillingPreviewLine[]>(`/api/billing/cycles/${id}/preview`),
	 createBillingCycle: (body: Omit<BillingCycle, 'id' | 'status' | 'payerCount' | 'createdAt' | 'publishedAt' | 'invoices'>) =>
		 request<BillingCycle>('/api/billing/cycles', { method: 'POST', body: JSON.stringify(body) }),
	 publishBillingCycle: (id: number) =>
		 request<{ status: string }>(`/api/billing/cycles/${id}/publish`, { method: 'POST' }),
	 closeBillingCycle: (id: number) =>
		 request<{ status: string }>(`/api/billing/cycles/${id}/close`, { method: 'POST' }),
	 deleteBillingCycle: (id: number) =>
		 request<{ status: string }>(`/api/billing/cycles/${id}`, { method: 'DELETE' }),
	 markInvoicePaid: (id: number) =>
		 request<{ status: string }>(`/api/billing/invoices/${id}/pay`, { method: 'POST' }),
	 cancelInvoice: (id: number) =>
		 request<{ status: string }>(`/api/billing/invoices/${id}/cancel`, { method: 'POST' }),
	 billingSummary: () => request<BillingSummary>('/api/billing/summary'),

  // Cabinet (public — token in URL is the credential)
  cabinetGet: (token: string) =>
    request<CabinetView>(`/api/cabinet/${enc(token)}`),
	 cabinetBilling: (token: string) =>
		 request<CabinetBillingSummary>(`/api/cabinet/${enc(token)}/billing`),
	 cabinetCheckout: (token: string, invoiceId: number, email: string) =>
		 request<{ confirmationUrl: string }>(`/api/cabinet/${enc(token)}/billing/checkout`, {
			 method: 'POST', body: JSON.stringify({ invoiceId, email }),
		 }),
  cabinetAddDevice: (token: string, body: { preset?: string; snippet?: string; deviceName: string }) =>
    request<AddDeviceResult>(`/api/cabinet/${enc(token)}/devices`, {
      method: 'POST', body: JSON.stringify(body),
    }),
  cabinetDeleteDevice: (token: string, devId: string) =>
    request<{ success: boolean }>(`/api/cabinet/${enc(token)}/devices/${enc(devId)}`, {
      method: 'DELETE',
    }),
  cabinetDeviceConfUrl: (token: string, devId: string) =>
    `/api/cabinet/${enc(token)}/devices/${enc(devId)}/configuration`,
  cabinetDeviceQrUrl: (token: string, devId: string) =>
    `/api/cabinet/${enc(token)}/devices/${enc(devId)}/qrcode.svg`,
  // Optional allowedIPs — cabinet split-tunnel override. Server validates
  // CIDRs and silently drops invalid entries; empty / missing → server default.
  cabinetDeviceAmneziaVpnUrl: (token: string, devId: string, allowedIPs?: string) =>
    `/api/cabinet/${enc(token)}/devices/${enc(devId)}/amnezia.vpn${
      allowedIPs ? `?allowed_ips=${encodeURIComponent(allowedIPs)}` : ''
    }`,
  cabinetDeviceAmneziaQrUrl: (token: string, devId: string, allowedIPs?: string) =>
    `/api/cabinet/${enc(token)}/devices/${enc(devId)}/amnezia-qrcode.svg${
      allowedIPs ? `?allowed_ips=${encodeURIComponent(allowedIPs)}` : ''
    }`,
  cabinetDeviceAmneziaQrChunks: (token: string, devId: string, allowedIPs?: string) =>
    request<{ chunks: string[] }>(
      `/api/cabinet/${enc(token)}/devices/${enc(devId)}/amnezia-qr-chunks${
        allowedIPs ? `?allowed_ips=${encodeURIComponent(allowedIPs)}` : ''
      }`,
    ),

  // Admin: Amnezia-native format URLs
  amneziaVpnUrl: (id: string) => `/api/wireguard/client/${enc(id)}/amnezia.vpn`,
  amneziaQrUrl:  (id: string) => `/api/wireguard/client/${enc(id)}/amnezia-qrcode.svg`,
  amneziaQrChunks: (id: string) =>
    request<{ chunks: string[] }>(`/api/wireguard/client/${enc(id)}/amnezia-qr-chunks`),

  backupUrl: () => '/api/backup',
  importClient: (body: {
    name: string; publicKey: string; profileId?: string;
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
