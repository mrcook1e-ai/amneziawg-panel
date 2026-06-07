<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import Modal from '@/components/molecules/Modal.vue'
import Button from '@/components/atoms/Button.vue'
import Icon from '@/components/atoms/Icon.vue'
import { api } from '@/lib/api'
import { copy } from '@/lib/clipboard'
import { useToastStore } from '@/stores/toasts'

const props = defineProps<{ open: boolean; clientId: string | null; clientName?: string }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const copied = ref(false)
const toasts = useToastStore()

watch(() => props.open, (v) => { if (v) copied.value = false })

const qrSrc = computed(() => props.clientId ? api.qrUrl(props.clientId) : '')
const dlUrl = computed(() => props.clientId ? api.configDownloadUrl(props.clientId) : '')

function download() {
  const a = document.createElement('a')
  a.href = dlUrl.value
  document.body.appendChild(a); a.click(); a.remove()
}

async function copyConf() {
  if (!props.clientId) return
  const text = await api.clientConfig(props.clientId)
  if (await copy(text)) {
    copied.value = true
    toasts.success('Конфиг скопирован')
    setTimeout(() => (copied.value = false), 1500)
  }
}
</script>

<template>
  <Modal :open="open" :title="clientName ? `QR · ${clientName}` : 'QR-код'" size="lg" @close="emit('close')">
    <div class="flex flex-col items-center gap-5 py-2">
      <!-- Pure-white card for max QR contrast regardless of page theme. -->
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
        <p class="mt-3 text-[12px] text-ink-500 text-center leading-relaxed">
          Сканируйте в приложении AmneziaWG или любом клиенте WireGuard, поддерживающем AWG 2.0.
        </p>
      </div>
    </div>
    <template #footer>
      <Button variant="ghost" size="sm" @click="emit('close')">Закрыть</Button>
      <Button variant="secondary" size="sm" @click="copyConf">
        <Icon :name="copied ? 'check' : 'copy'" :size="14" />
        {{ copied ? 'Скопировано' : 'Копировать .conf' }}
      </Button>
      <Button variant="primary" size="sm" @click="download">
        <Icon name="download" :size="14" /> Скачать .conf
      </Button>
    </template>
  </Modal>
</template>
