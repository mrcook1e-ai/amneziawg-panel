<script setup lang="ts">
import IconButton from '@/components/atoms/IconButton.vue'
import Icon from '@/components/atoms/Icon.vue'
import { useAuthStore } from '@/stores/auth'
import { useRouter, useRoute } from 'vue-router'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()

async function logout() {
  await auth.logout()
  router.push({ name: 'login' })
}

function toClients()  { router.push({ name: 'clients' }) }
function toSettings() { router.push({ name: 'settings' }) }
</script>

<template>
  <header class="sticky top-3 z-30 px-4">
    <div class="max-w-5xl mx-auto">
      <div class="glass rounded-2xl h-14 px-3 sm:px-4 flex items-center justify-between">
        <button class="flex items-center gap-2.5 -mx-1 px-1 rounded-lg focus-ring" @click="toClients">
          <div class="h-8 w-8 rounded-xl bg-ink-900 text-white grid place-items-center">
            <Icon name="shield" :size="15" />
          </div>
          <div class="leading-tight text-left">
            <div class="text-[13.5px] font-semibold text-ink-900 tracking-tight">AmneziaWG</div>
            <div class="text-[11px] text-ink-500">Control panel</div>
          </div>
        </button>

        <div class="flex items-center gap-1">
          <slot name="actions" />
          <IconButton
            :title="route.name === 'settings' ? 'Clients' : 'Settings'"
            @click="route.name === 'settings' ? toClients() : toSettings()"
          >
            <Icon :name="route.name === 'settings' ? 'x' : 'settings'" :size="16" />
          </IconButton>
          <IconButton v-if="auth.requiresPassword && auth.authenticated" title="Logout" @click="logout">
            <Icon name="logout" :size="16" />
          </IconButton>
        </div>
      </div>
    </div>
  </header>
</template>
