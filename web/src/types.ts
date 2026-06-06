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
