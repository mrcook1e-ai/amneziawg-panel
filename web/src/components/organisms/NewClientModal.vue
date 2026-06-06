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
  if (!n) { err.value = 'Name is required'; return }
  emit('submit', n)
}
</script>

<template>
  <Modal :open="open" title="New client" @close="emit('close')">
    <Field label="Name" :error="err" hint="Shown in the panel and in the downloaded config file name.">
      <Input v-model="name" autofocus placeholder="e.g. iPhone, Laptop" @keydown.enter="submit" />
    </Field>
    <template #footer>
      <Button variant="ghost" size="sm" @click="emit('close')">Cancel</Button>
      <Button variant="primary" size="sm" :loading="busy" @click="submit">Create</Button>
    </template>
  </Modal>
</template>
