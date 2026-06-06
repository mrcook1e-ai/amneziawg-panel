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
  ? 'Откройте AmneziaVPN на телефоне и наведите камеру. Если QR не сканируется — скопируйте ссылку vpn:// и откройте её в браузере телефона.'
  : 'Сканируйте в приложении AmneziaWG или любом клиенте WireGuard.')

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
    toasts.success('Ссылка скопирована')
    setTimeout(() => (copied.value = false), 1500)
  }
}
</script>

<template>
  <!--
    Crank up the modal to "lg" so the QR has room to breathe. The QR image
    itself is rendered at 1024×1024 by the server — here we display it at
    ~420px which keeps it pin-sharp on retina, and at full-width on mobile.
  -->
  <Modal :open="open" :title="clientName ? `QR · ${clientName}` : 'QR-код'" size="lg" @close="emit('close')">
    <div class="flex flex-col items-center gap-5 py-2">
      <Segmented
        v-model="format"
        :options="[{ value: 'vpn', label: 'AmneziaVPN' }, { value: 'conf', label: 'WireGuard' }]"
      />

      <!--
        White inner card — QR encoders work best on pure white regardless of
        page theme. We pin a 1:1 aspect ratio and let the image fill it.
      -->
      <div class="w-full max-w-[440px]">
        <div class="bg-white rounded-2xl p-5 shadow-card border border-ink-900/5 aspect-square grid place-items-center">
          <img
            v-if="clientId"
            :key="qrSrc"
            :src="qrSrc"
            alt="QR-код конфигурации"
            class="block w-full h-full object-contain"
            draggable="false"
          />
        </div>
        <p class="mt-3 text-[12px] text-ink-500 text-center leading-relaxed">{{ hint }}</p>
      </div>
    </div>
    <template #footer>
      <Button variant="ghost" size="sm" @click="emit('close')">Закрыть</Button>
      <Button variant="secondary" size="sm" @click="copyLink">
        <Icon :name="copied ? 'check' : 'copy'" :size="14" />
        {{ copied ? 'Скопировано' : 'Копировать ссылку' }}
      </Button>
      <Button variant="primary" size="sm" @click="download">
        <Icon name="download" :size="14" />
        Скачать {{ format === 'vpn' ? '.vpn' : '.conf' }}
      </Button>
    </template>
  </Modal>
</template>
