export interface Client {
  id: string
  subscriberId: string
  subscriberName?: string
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
  // null/undefined = inherit Itime from profile, otherwise this exact value
  // (including 0 for "disable CPS for this client", typically Windows).
  itimeOverride?: number | null
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
  // Junk train
  jc: number
  jmin: number
  jmax: number
  // Packet padding (AWG 2.0 — S3/S4 mandatory)
  s1: number
  s2: number
  s3: number
  s4: number
  // Header ranges, format "min-max"
  h1: string
  h2: string
  h3: string
  h4: string
  // Optional CPS strings
  i1?: string
  i2?: string
  i3?: string
  i4?: string
  i5?: string
  j1?: string
  j2?: string
  j3?: string
  // CPS chain interval in seconds, 0 = disabled
  itime: number
  clientCount: number
  hasMimicry: boolean
}

// BYOC profile creation — the admin pastes an [Interface] snippet
// (typically from AmneziaWG-Architect); the server parses it.
export interface ProfileCreateBody {
  id?: string
  name: string
  description?: string
  snippet: string
}

export interface ProfilePatchBody {
  name?: string
  description?: string
  // When present, replaces the obfuscation block atomically.
  snippet?: string
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
  rx7d: number
  tx7d: number
  rx30d: number
  tx30d: number
  rxTotal: number
  txTotal: number
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
  | 'device.created'
  | 'device.deleted'
  | 'profile.created'
  | 'profile.deleted'
  | 'profile.patched'
  | 'profile.restart'
  | 'subscriber.created'
  | 'subscriber.patched'
  | 'subscriber.deleted'
  | 'subscriber.regen_token'
  | 'server.reset_clients'
  | 'server.factory_reset'

// Subscriber: a named account owned by one person. Holds the access token
// that authenticates them at their cabinet (/cabinet/<token>).
export interface Subscriber {
  id: string
  name: string
  accessToken: string
  url: string
  notes?: string
  createdAt: string
  deviceCount: number
  devices?: Client[]
}

// What the public cabinet sees — never includes the access token itself.
export interface CabinetDevice {
  id: string
  name: string
  address: string
  enabled: boolean
  createdAt: string
  latestHandshakeAt?: string | null
}

export interface CabinetView {
  name: string
  devices: CabinetDevice[]
}

export interface AddDeviceResult {
  deviceId: string
  name: string
  address: string
  conf: string
  qrPng64: string
}

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
  // To set, send a number. To clear (inherit profile's Itime), set
  // clearItimeOverride = true.
  itimeOverride?: number
  clearItimeOverride?: boolean
}

export interface CreateClientArgs {
  name: string
  profileId?: string
  notes?: string
}
