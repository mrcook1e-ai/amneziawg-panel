<script setup lang="ts">
import Modal from './Modal.vue'
import Button from '@/components/atoms/Button.vue'

withDefaults(defineProps<{
  open: boolean
  title: string
  message?: string
  confirmText?: string
  cancelText?: string
  tone?: 'neutral' | 'danger'
  loading?: boolean
}>(), { confirmText: 'Подтвердить', cancelText: 'Отмена', tone: 'neutral' })

const emit = defineEmits<{ (e: 'confirm'): void; (e: 'cancel'): void }>()
</script>

<template>
  <Modal :open="open" :title="title" size="sm" @close="emit('cancel')">
    <p v-if="message" class="text-[13px] text-ink-600">{{ message }}</p>
    <template #footer>
      <Button variant="ghost" size="sm" @click="emit('cancel')">{{ cancelText }}</Button>
      <Button :variant="tone === 'danger' ? 'danger' : 'primary'" size="sm" :loading="loading" @click="emit('confirm')">
        {{ confirmText }}
      </Button>
    </template>
  </Modal>
</template>
