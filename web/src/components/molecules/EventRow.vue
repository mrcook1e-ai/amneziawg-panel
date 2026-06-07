<script setup lang="ts">
import { computed } from 'vue'
import type { AppEvent } from '@/types'
import { eventTime } from '@/lib/format'
import { useClientsStore } from '@/stores/clients'

const props = defineProps<{ event: AppEvent }>()
const clients = useClientsStore()

type Tone = 'neutral' | 'success' | 'warning' | 'danger'

const meta = computed<{ verb: string; subject: string; note: string; tone: Tone }>(() => {
  const e = props.event
  const c = e.clientId ? clients.byId(e.clientId) : null
  const payload = (e.payload ?? {}) as Record<string, any>
  const name = payload.name || c?.name || ''
  switch (e.kind) {
    case 'client.created':  return { verb: 'Создан',     subject: name, note: payload.address || '', tone: 'success' }
    case 'client.enabled':  return { verb: 'Включён',    subject: name, note: '', tone: 'success' }
    case 'client.disabled': return { verb: 'Выключен',   subject: name, note: '', tone: 'neutral' }
    case 'client.deleted':  return { verb: 'Удалён',     subject: name, note: '', tone: 'danger'  }
    case 'client.renamed':  return { verb: 'Переименован', subject: payload.from || '', note: `→ ${payload.to || ''}`, tone: 'neutral' }
    case 'client.expired':  return { verb: 'Срок истёк', subject: name, note: 'автоотключение', tone: 'warning' }
    case 'client.patched':  return { verb: 'Изменён',    subject: name, note: 'настройки', tone: 'neutral' }
    case 'client.moved':         return { verb: 'Перемещён', subject: name, note: `${payload.from || ''} → ${payload.to || ''}`, tone: 'neutral' }
    case 'profile.created':      return { verb: 'Профиль', subject: payload.name || '', note: `:${payload.port ?? ''}`, tone: 'success' }
    case 'profile.deleted':      return { verb: 'Профиль', subject: 'удалён', note: '', tone: 'danger' }
    case 'profile.patched':      return { verb: 'Профиль', subject: payload.name || 'изменён', note: '', tone: 'neutral' }
    case 'profile.restart':      return { verb: 'Профиль', subject: 'перезапущен', note: '', tone: 'warning' }
    case 'token.created':        return { verb: 'Инвайт', subject: payload.name || 'создан', note: '', tone: 'neutral' }
    case 'token.revoked':        return { verb: 'Инвайт', subject: 'отозван', note: '', tone: 'warning' }
    case 'token.redeemed':       return { verb: 'Инвайт', subject: 'использован', note: payload.name || '', tone: 'success' }
    case 'server.reset_clients': return { verb: 'Сервер', subject: 'удалены все клиенты',  note: `${payload.removed ?? 0}`, tone: 'danger' }
    case 'server.factory_reset': return { verb: 'Сервер', subject: 'заводской сброс',  note: '', tone: 'danger' }
    default: return { verb: e.kind, subject: name, note: '', tone: 'neutral' }
  }
})

const dotCls = computed(() => ({
  neutral: 'bg-ink-300',
  success: 'bg-success',
  warning: 'bg-warning',
  danger:  'bg-danger',
}[meta.value.tone]))
</script>

<template>
  <div class="group flex items-center gap-3 py-3 px-4">
    <span class="inline-block h-1.5 w-1.5 rounded-full shrink-0" :class="dotCls" />
    <span class="eyebrow shrink-0 w-[96px] tnum">{{ meta.verb }}</span>
    <span class="text-[14px] text-ink-900 truncate flex-1 min-w-0 font-medium">
      {{ meta.subject || '—' }}
    </span>
    <span v-if="meta.note" class="text-[12px] text-ink-500 truncate hidden sm:inline">{{ meta.note }}</span>
    <span class="mono text-[11px] text-ink-500 tnum shrink-0">{{ eventTime(event.ts) }}</span>
  </div>
</template>
