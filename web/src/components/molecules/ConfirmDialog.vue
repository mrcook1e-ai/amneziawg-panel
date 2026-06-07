<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import Modal from './Modal.vue'
import Button from '@/components/atoms/Button.vue'
import Input from '@/components/atoms/Input.vue'

/*
  ConfirmDialog с двумя режимами:
    • обычное подтверждение — две кнопки.
    • requireText — пользователь должен набрать заданную фразу, иначе кнопка
      «подтвердить» заблокирована. Используется для действительно опасных
      операций (factory reset, удаление по имени).
*/
const props = withDefaults(defineProps<{
  open: boolean
  title: string
  message?: string
  confirmText?: string
  cancelText?: string
  tone?: 'neutral' | 'danger'
  loading?: boolean
  /** Если задано — пользователь должен ввести эту строку слово в слово. */
  requireText?: string
}>(), { confirmText: 'Подтвердить', cancelText: 'Отмена', tone: 'neutral' })

const emit = defineEmits<{ (e: 'confirm'): void; (e: 'cancel'): void }>()

const typed = ref('')
watch(() => props.open, (v) => { if (v) typed.value = '' })

const blocked = computed(() => !!props.requireText && typed.value.trim() !== props.requireText)

function onConfirm() {
  if (blocked.value) return
  emit('confirm')
}
</script>

<template>
  <Modal :open="open" :title="title" size="sm" @close="emit('cancel')">
    <p v-if="message" class="text-[13px] text-ink-600 leading-relaxed">{{ message }}</p>

    <div v-if="requireText" class="mt-4 space-y-2">
      <p class="text-[12px] text-ink-500">
        Введите
        <span class="mono text-ink-900 bg-ink-100 px-1.5 py-0.5 rounded">{{ requireText }}</span>
        чтобы подтвердить.
      </p>
      <Input
        v-model="typed"
        :placeholder="requireText"
        :aria-label="`Введите ${requireText} для подтверждения`"
        autofocus
        @keydown.enter="onConfirm"
      />
    </div>

    <template #footer>
      <Button variant="ghost" size="sm" @click="emit('cancel')">{{ cancelText }}</Button>
      <Button
        :variant="tone === 'danger' ? 'danger' : 'primary'"
        size="sm"
        :loading="loading"
        :disabled="blocked"
        @click="onConfirm"
      >
        {{ confirmText }}
      </Button>
    </template>
  </Modal>
</template>
