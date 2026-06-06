<script setup lang="ts">
import { onMounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import ToastHost from '@/components/molecules/ToastHost.vue'

const auth = useAuthStore()
onMounted(() => auth.refresh())
</script>

<template>
  <router-view />
  <ToastHost />
</template>
