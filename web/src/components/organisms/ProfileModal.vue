<script setup lang="ts">
import { ref, watch } from 'vue'
import Modal from '@/components/molecules/Modal.vue'
import Field from '@/components/molecules/Field.vue'
import Input from '@/components/atoms/Input.vue'
import Button from '@/components/atoms/Button.vue'
import type { ProfileInfo } from '@/types'

const props = defineProps<{
  open: boolean
  mode: 'create' | 'edit'
  profile?: ProfileInfo | null
  busy?: boolean
}>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', body: {
    id?: string; name: string; description?: string;
    i1?: string; i2?: string; i3?: string; i4?: string; i5?: string;
  }): void
}>()

const id = ref('')
const name = ref('')
const description = ref('')
const useMimicry = ref(false)
const i1 = ref(''); const i2 = ref(''); const i3 = ref(''); const i4 = ref(''); const i5 = ref('')
const err = ref('')

watch(() => props.open, (v) => {
  if (!v) return
  err.value = ''
  if (props.mode === 'edit' && props.profile) {
    id.value = props.profile.id
    name.value = props.profile.name
    description.value = props.profile.description || ''
    i1.value = props.profile.i1 || ''
    i2.value = props.profile.i2 || ''
    i3.value = props.profile.i3 || ''
    i4.value = props.profile.i4 || ''
    i5.value = props.profile.i5 || ''
    useMimicry.value = props.profile.hasMimicry
  } else {
    id.value = ''
    name.value = ''
    description.value = ''
    i1.value = ''; i2.value = ''; i3.value = ''; i4.value = ''; i5.value = ''
    useMimicry.value = false
  }
})

function submit() {
  const n = name.value.trim()
  if (!n) { err.value = 'Введите имя'; return }
  if (props.mode === 'create' && id.value.trim() && !/^[a-z0-9-]{2,32}$/.test(id.value.trim())) {
    err.value = 'ID: 2–32 символа, только a–z, 0–9, дефис'
    return
  }
  emit('submit', {
    id: props.mode === 'create' ? id.value.trim() || undefined : undefined,
    name: n,
    description: description.value.trim() || undefined,
    i1: useMimicry.value ? i1.value.trim() : '',
    i2: useMimicry.value ? i2.value.trim() : '',
    i3: useMimicry.value ? i3.value.trim() : '',
    i4: useMimicry.value ? i4.value.trim() : '',
    i5: useMimicry.value ? i5.value.trim() : '',
  })
}
</script>

<template>
  <Modal :open="open" size="lg" :title="mode === 'create' ? 'Новый профиль подключения' : 'Редактирование профиля'" @close="emit('close')">
    <div class="space-y-4">
      <p class="text-[12.5px] text-ink-500 leading-relaxed">
        Профиль — это отдельный AmneziaWG-интерфейс на своём UDP-порту со своим ключом сервера и набором обфускации.
        Клиент подключается ровно к одному профилю.
      </p>

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <Field label="Имя" hint="Видно админу в списке профилей.">
          <Input v-model="name" autofocus placeholder="Mimicry QUIC" />
        </Field>
        <Field v-if="mode === 'create'" label="ID (необязательно)" hint="2–32 символа, a–z, 0–9, дефис. Если пусто — сгенерируется.">
          <Input v-model="id" placeholder="mimicry-quic" />
        </Field>
        <Field v-else label="ID" hint="ID нельзя поменять.">
          <Input :model-value="id" disabled />
        </Field>
      </div>

      <Field label="Описание (необязательно)">
        <Input v-model="description" placeholder="Для устройств за DPI" />
      </Field>

      <div class="flex items-center justify-between gap-3 p-3 rounded-lg bg-ink-100/40">
        <div>
          <div class="text-[13px] text-ink-900 font-medium">Мимикрия AWG 1.5 (I1–I5)</div>
          <div class="text-[11.5px] text-ink-500 mt-0.5">
            Опционально. CPS-строки получаются во внешнем генераторе
            <a class="underline" target="_blank" rel="noopener" href="https://github.com/amnezia-vpn/amneziawg-architect">AmneziaWG-Architect</a>.
            Без I-полей профиль работает в режиме AWG 1.0.
          </div>
        </div>
        <label class="shrink-0 inline-flex items-center gap-2 text-[12.5px] text-ink-700">
          <input type="checkbox" v-model="useMimicry" class="h-4 w-4 accent-current" />
          Включить
        </label>
      </div>

      <div v-if="useMimicry" class="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <Field label="I1">
          <textarea v-model="i1" rows="2" class="w-full p-2 rounded-lg bg-ink-100/60 border border-ink-900/10 text-[12px] mono focus-ring" placeholder="CPS-строка" />
        </Field>
        <Field label="I2">
          <textarea v-model="i2" rows="2" class="w-full p-2 rounded-lg bg-ink-100/60 border border-ink-900/10 text-[12px] mono focus-ring" placeholder="CPS-строка" />
        </Field>
        <Field label="I3">
          <textarea v-model="i3" rows="2" class="w-full p-2 rounded-lg bg-ink-100/60 border border-ink-900/10 text-[12px] mono focus-ring" placeholder="CPS-строка" />
        </Field>
        <Field label="I4">
          <textarea v-model="i4" rows="2" class="w-full p-2 rounded-lg bg-ink-100/60 border border-ink-900/10 text-[12px] mono focus-ring" placeholder="CPS-строка" />
        </Field>
        <Field label="I5">
          <textarea v-model="i5" rows="2" class="w-full p-2 rounded-lg bg-ink-100/60 border border-ink-900/10 text-[12px] mono focus-ring" placeholder="CPS-строка" />
        </Field>
      </div>

      <p v-if="err" class="text-[12px] text-danger">{{ err }}</p>
    </div>

    <template #footer>
      <Button variant="ghost" size="sm" @click="emit('close')">Отмена</Button>
      <Button variant="primary" size="sm" :loading="busy" @click="submit">
        {{ mode === 'create' ? 'Создать' : 'Сохранить' }}
      </Button>
    </template>
  </Modal>
</template>
