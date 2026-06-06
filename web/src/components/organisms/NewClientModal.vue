<script setup lang="ts">
import { ref, watch, computed, onMounted } from 'vue'
import Modal from '@/components/molecules/Modal.vue'
import Field from '@/components/molecules/Field.vue'
import Input from '@/components/atoms/Input.vue'
import Button from '@/components/atoms/Button.vue'
import { useProfilesStore } from '@/stores/profiles'
import type { CreateClientArgs } from '@/types'

const props = defineProps<{ open: boolean; busy?: boolean }>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', args: CreateClientArgs): void
}>()

const profiles = useProfilesStore()

const name = ref('')
const profileId = ref('')
const notes = ref('')
const err = ref('')

onMounted(() => { if (!profiles.items.length) profiles.fetch(true) })

watch(() => props.open, (v) => {
  if (v) {
    name.value = ''; notes.value = ''; err.value = ''
    profileId.value = profiles.defaultId
  }
})

const profileOptions = computed(() => profiles.items.map(p => ({
  value: p.id,
  label: `${p.name} · :${p.port}${p.hasMimicry ? ' · мимикрия' : ''}`,
})))

function submit() {
  const n = name.value.trim()
  if (!n) { err.value = 'Введите имя'; return }
  if (!profileId.value) { err.value = 'Выберите профиль'; return }
  profiles.rememberLastUsed(profileId.value)
  emit('submit', { name: n, profileId: profileId.value, notes: notes.value.trim() || undefined })
}
</script>

<template>
  <Modal :open="open" title="Новый клиент" @close="emit('close')">
    <div class="space-y-4">
      <Field label="Имя" hint="Отображается в панели и попадает в имя файла конфига.">
        <Input v-model="name" autofocus placeholder="например, iPhone, Ноутбук" @keydown.enter="submit" />
      </Field>

      <Field label="Профиль подключения" hint="UDP-порт, ключ сервера и обфускация задаются профилем.">
        <select
          v-model="profileId"
          class="w-full h-10 px-3 rounded-lg bg-ink-100/60 border border-ink-900/10 text-[13.5px] text-ink-900 focus-ring"
        >
          <option v-for="o in profileOptions" :key="o.value" :value="o.value">{{ o.label }}</option>
        </select>
      </Field>

      <Field label="Описание (необязательно)" hint="Заметка для админа: чьё устройство, срок и т.п.">
        <Input v-model="notes" placeholder="iPhone Андрея, до конца месяца" />
      </Field>

      <p v-if="err" class="text-[12px] text-danger">{{ err }}</p>
    </div>
    <template #footer>
      <Button variant="ghost" size="sm" @click="emit('close')">Отмена</Button>
      <Button variant="primary" size="sm" :loading="busy" @click="submit">Создать</Button>
    </template>
  </Modal>
</template>
