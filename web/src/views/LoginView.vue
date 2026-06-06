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
    <div class="w-full max-w-sm space-y-8">
      <!-- Brand: identical to the island TopBar so login feels like part of the panel,
           not a separate page. -->
      <div class="flex items-center gap-3 px-1">
        <div class="h-11 w-11 rounded-2xl bg-ink-900 text-ink-50 grid place-items-center shadow-card">
          <Icon name="shield" :size="20" />
        </div>
        <div class="leading-tight">
          <div class="text-[17px] font-semibold text-ink-900 tracking-tight">AmneziaWG</div>
          <div class="text-[12.5px] text-ink-500">Control panel</div>
        </div>
      </div>

      <div class="card p-6 sm:p-7">
        <h1 class="text-[19px] font-semibold text-ink-900 tracking-tight">Welcome back</h1>
        <p class="mt-1 text-[13px] text-ink-500">Enter the panel password to continue.</p>

        <form @submit.prevent="submit" class="mt-6 space-y-5">
          <Field label="Password" :error="err">
            <div class="relative">
              <Input
                v-model="password"
                :type="reveal ? 'text' : 'password'"
                autocomplete="current-password"
                autofocus
                :invalid="!!err"
              />
              <button
                type="button"
                class="absolute inset-y-0 right-0 px-3 text-ink-500 hover:text-ink-900 transition"
                @click="reveal = !reveal"
                :aria-label="reveal ? 'Hide password' : 'Show password'"
              >
                <Icon :name="reveal ? 'eye-off' : 'eye'" :size="16" />
              </button>
            </div>
          </Field>

          <Button type="submit" variant="primary" block :loading="busy">Sign in</Button>
        </form>
      </div>

      <p class="text-center text-[11.5px] text-ink-500 px-4">
        Session cookie is HTTP-only. Don't share this panel URL.
      </p>
    </div>
  </main>
</template>
