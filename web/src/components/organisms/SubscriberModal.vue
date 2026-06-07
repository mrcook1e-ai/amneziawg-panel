<script setup lang="ts">
/*
  Создание нового клиента (подписчика). Двухфазная форма:
    1) Имя + заметка → POST /api/subscribers → сервер выдаёт accessToken
    2) Показываем URL кабинета с большой кнопкой «Скопировать»

  Кабинет — постоянный, не одноразовый: ссылка валидна пока её не отозвали.
*/
import { ref, watch } from 'vue'
import Modal from '@/components/molecules/Modal.vue'
import Field from '@/components/molecules/Field.vue'
import Input from '@/components/atoms/Input.vue'
import Button from '@/components/atoms/Button.vue'
import Icon from '@/components/atoms/Icon.vue'
import { useToastStore } from '@/stores/toasts'
import type { Subscriber } from '@/types'

const props = defineProps<{
  open: boolean
  busy?: boolean
  created?: Subscriber | null
}>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', body: { name: string; notes?: string }): void
}>()

const toasts = useToastStore()
const name = ref('')
const notes = ref('')
const err = ref('')
const copied = ref(false)

watch(() => props.open, (v) => {
  if (!v) return
  name.value = ''
  notes.value = ''
  err.value = ''
  copied.value = false
})

function submit() {
  const n = name.value.trim()
  if (!n) { err.value = 'Укажите имя клиента'; return }
  emit('submit', { name: n, notes: notes.value.trim() || undefined })
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
    <div v-if="!created" class="space-y-4">
      <p class="text-[12.5px] text-ink-500 leading-relaxed">
        Создаётся аккаунт с постоянной ссылкой на личный кабинет. Клиент откроет ссылку, увидит свои устройства,
        сможет добавить новое (вставив параметры обфускации из
        <a class="underline" target="_blank" rel="noopener" href="https://vadim-khristenko.github.io/AmneziaWG-Architect/">Architect</a>),
        скачать .conf/QR, удалить ненужное.
      </p>

      <Field label="Имя клиента" hint="Что-то узнаваемое: «Вася», «отдел маркетинга», «Anton's family».">
        <Input v-model="name" autofocus placeholder="Вася" @keydown.enter="submit" />
      </Field>

      <Field label="Заметка (необязательно)" hint="Видна только админу. Что-то про оплату, контакт, срок и т.п.">
        <Input v-model="notes" placeholder="@vasya · до конца квартала" />
      </Field>

      <p v-if="err" class="text-[12px] text-danger">{{ err }}</p>
    </div>

    <div v-else class="space-y-4">
      <p class="text-[12.5px] text-ink-700 leading-relaxed">
        Аккаунт <span class="font-medium">«{{ created.name }}»</span> создан. Передайте ссылку клиенту —
        в этом кабинете он сам управляет своими устройствами. Ссылка постоянная; чтобы отозвать —
        обновите токен в карточке клиента.
      </p>

      <div class="p-3.5 rounded-xl bg-ink-100">
        <div class="mono text-[11.5px] text-ink-900 break-all leading-snug select-all">{{ created.url }}</div>
      </div>

      <Button variant="primary" size="md" class="w-full" @click="copyURL">
        <Icon :name="copied ? 'check' : 'copy'" :size="15" />
        {{ copied ? 'Скопировано' : 'Скопировать ссылку' }}
      </Button>

      <p class="text-[11px] text-ink-500">
        Ссылка не имеет срока действия. Любой, у кого она есть, попадёт в кабинет —
        передавайте через защищённый канал.
      </p>
    </div>

    <template #footer>
      <template v-if="!created">
        <Button variant="ghost" size="sm" @click="emit('close')">Отмена</Button>
        <Button variant="primary" size="sm" :loading="busy" @click="submit">Создать</Button>
      </template>
      <template v-else>
        <Button variant="primary" size="sm" @click="emit('close')">Готово</Button>
      </template>
    </template>
  </Modal>
</template>
