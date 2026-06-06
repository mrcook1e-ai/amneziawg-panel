<script setup lang="ts">
import { ref } from 'vue'
import IconButton from '@/components/atoms/IconButton.vue'
import Icon from '@/components/atoms/Icon.vue'
import { copy } from '@/lib/clipboard'

const props = defineProps<{ value: string; title?: string }>()
const copied = ref(false)

async function onClick() {
  if (await copy(props.value)) {
    copied.value = true
    setTimeout(() => (copied.value = false), 1200)
  }
}
</script>

<template>
  <IconButton size="sm" :title="title || 'Copy'" @click="onClick">
    <Icon :name="copied ? 'check' : 'copy'" :size="14" />
  </IconButton>
</template>
