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

export type ToastKind = 'info' | 'success' | 'warning' | 'danger'

export interface Toast {
  id: number
  kind: ToastKind
  message: string
}
