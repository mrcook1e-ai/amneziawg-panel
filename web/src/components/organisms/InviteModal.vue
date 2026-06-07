<script setup lang="ts">
import { ref, watch } from 'vue'
import Modal from '@/components/molecules/Modal.vue'
import Field from '@/components/molecules/Field.vue'
import Input from '@/components/atoms/Input.vue'
import Button from '@/components/atoms/Button.vue'
import Segmented from '@/components/atoms/Segmented.vue'
import Icon from '@/components/atoms/Icon.vue'
import { useToastStore } from '@/stores/toasts'
import type { OnboardToken } from '@/types'

const props = defineProps<{
  open: boolean
  busy?: boolean
  // When set by parent after a successful API call, switches to "created"
  // phase with URL displayed. Parent clears it when modal closes.
  created?: OnboardToken | null
}>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', body: { name: string; expiresIn: number }): void
}>()

const toasts = useToastStore()

const name = ref('')
const ttl = ref<'1h' | '1d' | '7d' | 'never'>('1d')
const err = ref('')
const copied = ref(false)

const ttlOptions = [
  { value: '1h',    label: '1 час'  },
  { value: '1d',    label: '1 день' },
  { value: '7d',    label: '7 дней' },
  { value: 'never', label: 'Без срока' },
] as const

const ttlMap: Record<typeof ttl.value, number> = {
  '1h': 3600,
  '1d': 86400,
  '7d': 604800,
  'never': 0,
}

watch(() => props.open, (v) => {
  if (!v) return
  name.value = ''
  ttl.value = '1d'
  err.value = ''
  copied.value = false
})

function submit() {
  const n = name.value.trim()
  if (!n) { err.value = 'Укажите имя — кому выдаёте'; return }
  emit('submit', { name: n, expiresIn: ttlMap[ttl.value] })
}

async function copyURL() {
  if (!props.created) return
  try {
    await navigator.clipboard.writeText(props.created.url)
    copied.value = true
    toasts.success('Ссылка скопирована')
    setTimeout(() => (copied.value = false), 1500)
  } catch { toasts.error('Не удалось скопировать') }
}
</script>

<template>
  <Modal
    :open="open"
    size="md"
    :title="created ? 'Готово — передайте ссылку клиенту' : 'Новый клиент'"
    @close="emit('close')"
  >
    <!-- Phase 1: form -->
    <div v-if="!created" class="space-y-4">
      <p class="text-[12.5px] text-ink-500 leading-relaxed">
        Передадите клиенту одноразовую ссылку. Он откроет её в браузере, вставит параметры обфускации
        (берутся из <a class="underline" target="_blank" rel="noopener" href="https://vadim-khristenko.github.io/AmneziaWG-Architect/">Architect</a>)
        и получит свой <span class="mono">.conf</span> + QR. Сервер автоматически поднимет отдельный awgN-интерфейс.
      </p>

      <Field label="Имя" hint="Только для вашей пометки в списке.">
        <Input v-model="name" autofocus placeholder="Вася / iPhone жены / …" @keydown.enter="submit" />
      </Field>

      <Field label="Срок действия ссылки">
        <Segmented v-model="ttl" :options="[...ttlOptions]" />
      </Field>

      <p v-if="err" class="text-[12px] text-danger">{{ err }}</p>
    </div>

    <!-- Phase 2: created — show URL with copy button -->
    <div v-else class="space-y-4">
      <p class="text-[12.5px] text-ink-700 leading-relaxed">
        Ссылка для <span class="font-medium">{{ created.name }}</span> создана.
        Скопируйте и отправьте через любой мессенджер.
        Ссылка одноразовая — после того как клиент получит конфиг, она перестанет работать.
      </p>

      <div class="p-3 rounded-lg bg-ink-100/60 border border-ink-900/10">
        <div class="mono text-[11.5px] text-ink-900 break-all leading-snug select-all">{{ created.url }}</div>
      </div>

      <Button variant="primary" size="md" class="w-full" @click="copyURL">
        <Icon :name="copied ? 'check' : 'copy'" :size="15" />
        {{ copied ? 'Скопировано' : 'Скопировать ссылку' }}
      </Button>

      <p v-if="created.expiresAt" class="text-[11.5px] text-ink-500">
        Истекает {{ new Date(created.expiresAt).toLocaleString() }}.
      </p>
      <p v-else class="text-[11.5px] text-ink-500">
        Без срока действия — отзовите вручную в списке клиентов, если передумаете.
      </p>
    </div>

    <template #footer>
      <template v-if="!created">
        <Button variant="ghost" size="sm" @click="emit('close')">Отмена</Button>
        <Button variant="primary" size="sm" :loading="busy" @click="submit">Создать ссылку</Button>
      </template>
      <template v-else>
        <Button variant="primary" size="sm" @click="emit('close')">Готово</Button>
      </template>
    </template>
  </Modal>
</template>
