<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import Modal from '@/components/molecules/Modal.vue'
import Button from '@/components/atoms/Button.vue'
import CodeBlock from '@/components/molecules/CodeBlock.vue'
import Spinner from '@/components/atoms/Spinner.vue'
import { api } from '@/lib/api'

const props = defineProps<{ open: boolean; clientId: string | null; clientName?: string }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const code = ref('')
const loading = ref(false)

async function load() {
  if (!props.open || !props.clientId) { code.value = ''; return }
  loading.value = true
  try { code.value = await api.clientConfig(props.clientId) }
  finally { loading.value = false }
}

watch(() => [props.open, props.clientId], load, { immediate: false })

const dlUrl = computed(() => props.clientId ? api.configDownloadUrl(props.clientId) : '')

function download() {
  const a = document.createElement('a')
  a.href = dlUrl.value
  document.body.appendChild(a); a.click(); a.remove()
}
</script>

<template>
  <Modal :open="open" :title="clientName ? `Конфиг · ${clientName}` : 'Конфигурация'" size="lg" @close="emit('close')">
    <div class="space-y-3">
      <div class="text-[11px] text-ink-500">файл .conf для AmneziaWG / WireGuard клиента</div>
      <div v-if="loading" class="py-10 grid place-items-center"><Spinner :size="20" /></div>
      <CodeBlock v-else :code="code" />
    </div>
    <template #footer>
      <Button variant="ghost" size="sm" @click="emit('close')">Закрыть</Button>
      <Button variant="primary" size="sm" :disabled="!code" @click="download">Скачать .conf</Button>
    </template>
  </Modal>
</template>
