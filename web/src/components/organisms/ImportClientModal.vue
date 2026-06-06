<script setup lang="ts">
/*
  Импорт существующего клиента по уже выданному публичному ключу. Сценарий:
  у пользователя сохранён конфиг на устройстве, но в панели его нет — например,
  после переустановки сервера. Привязываем peer обратно по pubkey, IP-адрес
  можно указать вручную (если он зафиксирован в конфиге) или дать выделить
  автоматически.
*/
import { ref, watch, computed, onMounted } from 'vue'
import Modal from '@/components/molecules/Modal.vue'
import Field from '@/components/molecules/Field.vue'
import Input from '@/components/atoms/Input.vue'
import Button from '@/components/atoms/Button.vue'
import { useProfilesStore } from '@/stores/profiles'

const props = defineProps<{ open: boolean; busy?: boolean }>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', body: { name: string; profileId: string; publicKey: string; privateKey?: string; preSharedKey?: string; address?: string; notes?: string }): void
}>()

const profiles = useProfilesStore()
const name = ref('')
const profileId = ref('')
const publicKey = ref('')
const privateKey = ref('')
const preSharedKey = ref('')
const address = ref('')
const notes = ref('')
const err = ref('')

onMounted(() => { if (!profiles.items.length) profiles.fetch(true) })

watch(() => props.open, (v) => {
  if (v) {
    name.value = ''; publicKey.value = ''; privateKey.value = ''
    preSharedKey.value = ''; address.value = ''; notes.value = ''
    profileId.value = profiles.defaultId
    err.value = ''
  }
})

const profileOptions = computed(() => profiles.items.map(p => ({
  value: p.id, label: `${p.name} · :${p.port}`,
})))

function submit() {
  const n = name.value.trim()
  const pk = publicKey.value.trim()
  if (!n) { err.value = 'Введите имя'; return }
  if (!pk) { err.value = 'Публичный ключ обязателен'; return }
  if (!profileId.value) { err.value = 'Выберите профиль'; return }
  emit('submit', {
    name: n, profileId: profileId.value, publicKey: pk,
    privateKey: privateKey.value.trim() || undefined,
    preSharedKey: preSharedKey.value.trim() || undefined,
    address: address.value.trim() || undefined,
    notes: notes.value.trim() || undefined,
  })
}
</script>

<template>
  <Modal :open="open" size="lg" title="Импорт клиента" @close="emit('close')">
    <div class="space-y-4">
      <p class="text-[12.5px] text-ink-500 leading-relaxed">
        Если у клиента уже есть конфиг, можно привязать его обратно по публичному ключу.
        Приватный ключ нужен только для повторного скачивания конфига с панели — без него
        peer работает, но новый .conf панель не покажет.
      </p>

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <Field label="Имя">
          <Input v-model="name" placeholder="например, iPhone-возврат" autofocus />
        </Field>
        <Field label="IP в подсети" hint="Оставьте пусто — выделится автоматически">
          <Input v-model="address" placeholder="10.99.0.42" />
        </Field>
      </div>

      <Field label="Профиль подключения">
        <select v-model="profileId" class="w-full h-10 px-3 rounded-lg bg-ink-100/60 border border-ink-900/10 text-[13.5px] text-ink-900 focus-ring">
          <option v-for="o in profileOptions" :key="o.value" :value="o.value">{{ o.label }}</option>
        </select>
      </Field>

      <Field label="Публичный ключ">
        <Input v-model="publicKey" placeholder="base64..." />
      </Field>

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <Field label="Приватный ключ" hint="Опционально — для перевыдачи конфига">
          <Input v-model="privateKey" placeholder="base64..." />
        </Field>
        <Field label="Pre-shared key" hint="Если используется">
          <Input v-model="preSharedKey" placeholder="base64..." />
        </Field>
      </div>

      <Field label="Описание">
        <Input v-model="notes" placeholder="Комментарий для себя" />
      </Field>

      <p v-if="err" class="text-[12px] text-danger">{{ err }}</p>
    </div>

    <template #footer>
      <Button variant="ghost" size="sm" @click="emit('close')">Отмена</Button>
      <Button variant="primary" size="sm" :loading="busy" @click="submit">Импортировать</Button>
    </template>
  </Modal>
</template>
