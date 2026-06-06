export interface Client {
  id: string
  profileId: string
  name: string
  address: string
  publicKey: string
  enabled: boolean
  createdAt: string
  updatedAt: string
  latestHandshakeAt: string | null
  transferRx: number
  transferTx: number
  persistentKeepalive: string

  notes?: string
  expiresAt?: string | null
  dnsOverride?: string
  allowedIPsOverride?: string
  mtuOverride?: number
  totalRx?: number
  totalTx?: number
  lastHandshakeAt?: string | null
}

export interface SessionState {
  requiresPassword: boolean
  authenticated: boolean
}

export interface ProfileInfo {
  id: string
  name: string
  description?: string
  iface: string
  port: number
  publicKey: string
  address: string
  endpoint: string
  jc: number
  jmin: number
  jmax: number
  s1: number
  s2: number
  h1: string
  h2: string
  h3: string
  h4: string
  i1?: string
  i2?: string
  i3?: string
  i4?: string
  i5?: string
  clientCount: number
  hasMimicry: boolean
}

export interface ProfileCreateBody {
  id?: string
  name: string
  description?: string
  i1?: string
  i2?: string
  i3?: string
  i4?: string
  i5?: string
}

export interface ProfilePatchBody {
  name?: string
  description?: string
  i1?: string
  i2?: string
  i3?: string
  i4?: string
  i5?: string
}

export type ToastKind = 'info' | 'success' | 'warning' | 'danger'

export interface Toast {
  id: number
  kind: ToastKind
  message: string
}

export interface TopRow {
  clientId: string
  rx: number
  tx: number
}

export interface Overview {
  windowSeconds: number
  rxLast: number
  txLast: number
  rxToday: number
  txToday: number
  top: TopRow[] | null
  asof: string
}

export interface SeriesPoint {
  ts: number
  rx: number
  tx: number
}

export interface Series {
  bucketSeconds: number
  points: SeriesPoint[]
}

export interface ClientStats {
  windowSeconds: number
  rxLast: number
  txLast: number
  rx24h: number
  tx24h: number
  rx7d: number
  tx7d: number
  onlineRatio7d: number
  series: SeriesPoint[]
}

export type EventKind =
  | 'client.created'
  | 'client.deleted'
  | 'client.enabled'
  | 'client.disabled'
  | 'client.renamed'
  | 'client.expired'
  | 'client.patched'
  | 'client.moved'
  | 'profile.created'
  | 'profile.deleted'
  | 'profile.patched'
  | 'profile.restart'
  | 'profile.regen_magic'
  | 'server.reset_clients'
  | 'server.factory_reset'

export interface AppEvent {
  id: number
  ts: string
  kind: EventKind
  clientId?: string
  payload?: Record<string, unknown> | null
}

export interface ClientPatch {
  notes?: string
  expiresAt?: string | null
  clearExpiresAt?: boolean
  dnsOverride?: string
  allowedIPsOverride?: string
  mtuOverride?: number
}

export interface CreateClientArgs {
  name: string
  profileId?: string
  notes?: string
}
