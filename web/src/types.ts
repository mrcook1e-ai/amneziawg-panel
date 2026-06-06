export interface Client {
  id: string
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

  // Extended fields (Phase 1 backend).
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

export interface ServerInfo {
  publicKey: string
  address: string
  interface: string
  endpoint: string
  subnet: string
  port: number
  egressIface: string
  dns: string
  mtu: number
  allowedIPs: string
  persistentKeepalive: number
  jc: number
  jmin: number
  jmax: number
  s1: number
  s2: number
  h1: string
  h2: string
  h3: string
  h4: string
  clientCount: number
}

export type ToastKind = 'info' | 'success' | 'warning' | 'danger'

export interface Toast {
  id: number
  kind: ToastKind
  message: string
}

// ─── Stats (Phase 2 backend) ───────────────────────────────────────────────

export interface TopRow {
  clientId: string
  rx: number
  tx: number
}

export interface Overview {
  windowSeconds: number   // 300
  rxLast: number          // bytes in last 5 min
  txLast: number
  rxToday: number
  txToday: number
  top: TopRow[] | null
  asof: string
}

export interface SeriesPoint {
  ts: number   // unix seconds
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

// ─── Events ────────────────────────────────────────────────────────────────

export type EventKind =
  | 'client.created'
  | 'client.deleted'
  | 'client.enabled'
  | 'client.disabled'
  | 'client.renamed'
  | 'client.expired'
  | 'client.patched'
  | 'server.restart'
  | 'server.regen_magic'
  | 'server.reset_clients'

export interface AppEvent {
  id: number
  ts: string
  kind: EventKind
  clientId?: string
  payload?: Record<string, unknown> | null
}

// ─── Client patch ──────────────────────────────────────────────────────────

export interface ClientPatch {
  notes?: string
  expiresAt?: string | null
  clearExpiresAt?: boolean
  dnsOverride?: string
  allowedIPsOverride?: string
  mtuOverride?: number
}
