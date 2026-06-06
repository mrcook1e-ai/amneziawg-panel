<script setup lang="ts">
import Button from '@/components/atoms/Button.vue'
import IconButton from '@/components/atoms/IconButton.vue'
import Icon from '@/components/atoms/Icon.vue'
import { useAuthStore } from '@/stores/auth'
import { useRouter } from 'vue-router'

const auth = useAuthStore()
const router = useRouter()

async function logout() {
  await auth.logout()
  router.push({ name: 'login' })
}
</script>

<template>
  <header class="sticky top-0 z-30 bg-white/85 backdrop-blur border-b border-ink-200">
    <div class="max-w-6xl mx-auto px-5 h-14 flex items-center justify-between">
      <div class="flex items-center gap-2.5">
        <div class="h-7 w-7 rounded-lg bg-ink-900 text-white grid place-items-center">
          <Icon name="shield" :size="15" />
        </div>
        <div class="leading-tight">
          <div class="text-[14px] font-semibold text-ink-900">AmneziaWG</div>
          <div class="text-[11px] text-ink-500">Control panel</div>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <slot name="actions" />
        <IconButton v-if="auth.requiresPassword && auth.authenticated" title="Logout" @click="logout">
          <Icon name="logout" :size="16" />
        </IconButton>
      </div>
    </div>
  </header>
</template>
