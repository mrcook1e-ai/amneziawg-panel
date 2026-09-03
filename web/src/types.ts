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
  totalRx?: number
  totalTx?: number
  lastHandshakeAt?: string | null
	 billingSuspended?: boolean
}

export type BillingRole = 'owner' | 'trusted' | 'payer'

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
  // Packet padding. S3/S4 are 0 on AWG 1.0 profiles, which predate them.
  s1: number
  s2: number
  s3: number
  s4: number
  // Magic headers: "n" (fixed, AWG 1.0 / 3.1) or "min-max" (range, AWG 2.0)
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

  // AWG 3.x. All optional — absent means "not set", i.e. AWG 2.0 behaviour.
  // headerProtectionKey is secret key material: mask it in the UI.
  headerProtectionKey?: string
  contentPaddingAddition?: string
  rekeyAfterTime?: string
  rekeyTimeout?: string
  rejectAfterTime?: string
  keepaliveTimeout?: string
  maxHandshakeAttempts?: string
  randomTrailers?: boolean
  disableCookies?: boolean
  // Peer-section keepalive as a range ("25-35"). Empty = server-wide default.
  persistentKeepalive?: string

  clientCount: number
  hasMimicry: boolean
  // Protocol generation detected from the profile's own markers, the same way
  // the official AmneziaVPN client classifies an imported config.
  generation: AWGGeneration
}

export type AWGGeneration = '1.0' | '1.5' | '2.0' | '3.1'

/** Cabinet device presets — one per AmneziaWG protocol generation. */
export type PresetKey = 'awg1' | 'awg2' | 'awg31' 

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
	 billingRole: BillingRole
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

export type InvoiceStatus = 'pending' | 'paid' | 'canceled'
export type BillingCycleStatus = 'draft' | 'published' | 'closed'
export type CabinetBillingStatus = 'exempt' | 'pending' | 'grace' | 'overdue' | 'paid'
export type BillingSplitMode = 'equal' | 'traffic'

export interface BillingInvoice {
	 id: number
	 cycleId: number
	 subscriberId: string
	 subscriberName: string
	 amount: number
	 status: InvoiceStatus
	 paidAt?: number | null
}

export interface BillingCycle {
	 id: number
	 title: string
	 periodStart: number
	 periodEnd: number
	 paymentDueAt: number
	 graceEndsAt: number
	 totalAmount: number
	 status: BillingCycleStatus
	 splitMode: BillingSplitMode
	 payerCount: number
	 createdAt: number
	 publishedAt?: number | null
	 invoices?: BillingInvoice[]
}

export interface BillingPreviewLine {
	 subscriberId: string
	 subscriberName: string
	 bytes: number
	 amount: number
}

export interface BillingSummary {
	 totalReceived: number
	 totalPending: number
}

export interface CabinetBillingSummary {
	 billingRole: BillingRole
	 derivedStatus: CabinetBillingStatus
	 checkoutEnabled: boolean
	 paymentContact?: string
	 latestInvoice?: BillingInvoice
	 latestCycle?: BillingCycle
	 history?: BillingHistoryItem[]
}

export interface BillingHistoryItem {
	 cycleTitle: string
	 amount: number
	 status: InvoiceStatus
	 periodEnd: number
	 paidAt?: number | null
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
}

export interface CreateClientArgs {
  name: string
  profileId?: string
  notes?: string
}
