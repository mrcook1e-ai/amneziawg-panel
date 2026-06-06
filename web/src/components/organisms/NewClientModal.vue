<script setup lang="ts">
import { ref, watch } from 'vue'
import Modal from '@/components/molecules/Modal.vue'
import Field from '@/components/molecules/Field.vue'
import Input from '@/components/atoms/Input.vue'
import Button from '@/components/atoms/Button.vue'

const props = defineProps<{ open: boolean; busy?: boolean }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'submit', name: string): void }>()

const name = ref('')
const err = ref('')

watch(() => props.open, (v) => { if (v) { name.value = ''; err.value = '' } })

function submit() {
  const n = name.value.trim()
  if (!n) { err.value = 'Введите имя'; return }
  emit('submit', n)
}
</script>

<template>
  <Modal :open="open" title="Новый клиент" @close="emit('close')">
    <Field label="Имя" :error="err" hint="Отображается в панели и попадает в имя файла конфига.">
      <Input v-model="name" autofocus placeholder="например, iPhone, Ноутбук" @keydown.enter="submit" />
    </Field>
    <template #footer>
      <Button variant="ghost" size="sm" @click="emit('close')">Отмена</Button>
      <Button variant="primary" size="sm" :loading="busy" @click="submit">Создать</Button>
    </template>
  </Modal>
</template>
