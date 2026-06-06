<script setup lang="ts">
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toasts'
import Field from '@/components/molecules/Field.vue'
import Input from '@/components/atoms/Input.vue'
import Button from '@/components/atoms/Button.vue'
import Icon from '@/components/atoms/Icon.vue'

const auth = useAuthStore()
const toasts = useToastStore()
const router = useRouter()
const route = useRoute()

const password = ref('')
const reveal = ref(false)
const busy = ref(false)
const err = ref('')

async function submit() {
  if (!password.value) { err.value = 'Password is required'; return }
  busy.value = true; err.value = ''
  const ok = await auth.login(password.value)
  busy.value = false
  if (ok) {
    toasts.success('Signed in')
    router.replace((route.query.to as string) || '/')
  } else {
    err.value = 'Incorrect password'
  }
}
</script>

<template>
  <main class="min-h-full grid place-items-center px-5 py-10">
    <div class="w-full max-w-sm card p-6">
      <div class="flex items-center gap-2.5 mb-5">
        <div class="h-9 w-9 rounded-lg bg-ink-900 text-white grid place-items-center">
          <Icon name="shield" :size="17" />
        </div>
        <div class="leading-tight">
          <div class="text-[15px] font-semibold text-ink-900">AmneziaWG Panel</div>
          <div class="text-[12px] text-ink-500">Dev · sign in to continue</div>
        </div>
      </div>

      <form @submit.prevent="submit" class="space-y-4">
        <Field label="Password" :error="err">
          <div class="relative">
            <Input
              v-model="password"
              :type="reveal ? 'text' : 'password'"
              autocomplete="current-password"
              autofocus
              :invalid="!!err"
            />
            <button type="button" class="absolute inset-y-0 right-0 px-3 text-ink-500 hover:text-ink-800" @click="reveal = !reveal" :aria-label="reveal ? 'Hide password' : 'Show password'">
              <Icon :name="reveal ? 'eye-off' : 'eye'" :size="16" />
            </button>
          </div>
        </Field>

        <Button type="submit" variant="primary" block :loading="busy">Sign in</Button>
      </form>
    </div>
  </main>
</template>
