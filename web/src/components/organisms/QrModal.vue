<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import Modal from '@/components/molecules/Modal.vue'
import Button from '@/components/atoms/Button.vue'
import Segmented from '@/components/atoms/Segmented.vue'
import Icon from '@/components/atoms/Icon.vue'
import { api } from '@/lib/api'
import { copy } from '@/lib/clipboard'
import { useToastStore } from '@/stores/toasts'

type Format = 'vpn' | 'conf'

const props = defineProps<{ open: boolean; clientId: string | null; clientName?: string }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const format = ref<Format>('vpn')
const copied = ref(false)
const toasts = useToastStore()

watch(() => props.open, (v) => { if (v) { format.value = 'vpn'; copied.value = false } })

const qrSrc = computed(() => {
  if (!props.clientId) return ''
  return format.value === 'vpn' ? api.vpnQrUrl(props.clientId) : api.qrUrl(props.clientId)
})

const dlUrl = computed(() => {
  if (!props.clientId) return ''
  return format.value === 'vpn' ? api.vpnDownloadUrl(props.clientId) : api.configDownloadUrl(props.clientId)
})

const hint = computed(() => format.value === 'vpn'
  ? 'AmneziaVPN: copy the vpn:// link and tap it on your phone — opens the app directly. QR works for short configs but a 900-char vpn:// link is dense.'
  : 'Scan with the AmneziaWG dedicated app, or any client that supports raw WireGuard config.')

function download() {
  const a = document.createElement('a')
  a.href = dlUrl.value
  document.body.appendChild(a); a.click(); a.remove()
}

async function copyLink() {
  if (!props.clientId) return
  const text = format.value === 'vpn'
    ? await api.clientVPN(props.clientId)
    : await api.clientConfig(props.clientId)
  if (await copy(text)) {
    copied.value = true
    toasts.success('Link copied — paste on phone to open in AmneziaVPN')
    setTimeout(() => (copied.value = false), 1500)
  }
}
</script>

<template>
  <Modal :open="open" :title="clientName ? `QR · ${clientName}` : 'QR code'" size="sm" @close="emit('close')">
    <div class="flex flex-col items-center gap-3">
      <Segmented
        v-model="format"
        :options="[{ value: 'vpn', label: 'AmneziaVPN' }, { value: 'conf', label: 'Plain WG' }]"
      />
      <div class="p-3 bg-white rounded-xl">
        <img v-if="clientId" :key="qrSrc" :src="qrSrc" alt="Configuration QR code" width="256" height="256" class="block" />
      </div>
      <p class="text-[12px] text-ink-500 text-center max-w-xs">{{ hint }}</p>
    </div>
    <template #footer>
      <Button variant="ghost" size="sm" @click="emit('close')">Close</Button>
      <Button variant="secondary" size="sm" @click="copyLink">
        <Icon :name="copied ? 'check' : 'copy'" :size="14" />
        {{ copied ? 'Copied' : 'Copy link' }}
      </Button>
      <Button variant="primary" size="sm" @click="download">
        Download {{ format === 'vpn' ? '.vpn' : '.conf' }}
      </Button>
    </template>
  </Modal>
</template>
