<script setup lang="ts">
import { ref, watch } from 'vue'
import Modal from '@/components/molecules/Modal.vue'
import Field from '@/components/molecules/Field.vue'
import Input from '@/components/atoms/Input.vue'
import Button from '@/components/atoms/Button.vue'
import Segmented from '@/components/atoms/Segmented.vue'

const props = defineProps<{ open: boolean; busy?: boolean }>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', body: { name: string; expiresIn: number }): void
}>()

const name = ref('')
const ttl = ref<'1h' | '1d' | '7d' | 'never'>('1d')
const err = ref('')

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
})

function submit() {
  const n = name.value.trim()
  if (!n) { err.value = 'Укажите имя — кому выдаёте'; return }
  emit('submit', { name: n, expiresIn: ttlMap[ttl.value] })
}
</script>

<template>
  <Modal :open="open" size="md" title="Новый инвайт" @close="emit('close')">
    <div class="space-y-4">
      <p class="text-[12.5px] text-ink-500 leading-relaxed">
        Одноразовая ссылка. Передайте её клиенту — он откроет в браузере, вставит параметры обфускации
        (берутся из <a class="underline" target="_blank" rel="noopener" href="https://vadim-khristenko.github.io/AmneziaWG-Architect/">Architect</a>)
        и получит свой <span class="mono">.conf</span> + QR. Сервер автоматически поднимет отдельный awgN-интерфейс на новом порту.
      </p>

      <Field label="Кому" hint="Только для вашей пометки в списке инвайтов.">
        <Input v-model="name" autofocus placeholder="Вася / iPhone жены / …" />
      </Field>

      <Field label="Срок действия">
        <Segmented v-model="ttl" :options="[...ttlOptions]" />
      </Field>

      <p v-if="err" class="text-[12px] text-danger">{{ err }}</p>
    </div>

    <template #footer>
      <Button variant="ghost" size="sm" @click="emit('close')">Отмена</Button>
      <Button variant="primary" size="sm" :loading="busy" @click="submit">Создать инвайт</Button>
    </template>
  </Modal>
</template>
