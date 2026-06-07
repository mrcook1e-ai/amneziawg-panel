<script setup lang="ts">
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toasts'
import Field from '@/components/molecules/Field.vue'
import Input from '@/components/atoms/Input.vue'
import Button from '@/components/atoms/Button.vue'
import Icon from '@/components/atoms/Icon.vue'
import { useTitle } from '@/composables/useTitle'

useTitle(() => 'Вход · Amnezia Panel')

const auth = useAuthStore()
const toasts = useToastStore()
const router = useRouter()
const route = useRoute()

const password = ref('')
const reveal = ref(false)
const busy = ref(false)
const err = ref('')

// Whitelist only same-origin relative paths to block open-redirect via
// ?to=//evil.com or ?to=https://evil.com. Must start with a single "/"
// and not "//", and must not contain a scheme.
function safeRedirect(raw: unknown): string {
  if (typeof raw !== 'string' || !raw) return '/'
  if (!raw.startsWith('/')) return '/'
  if (raw.startsWith('//')) return '/'
  if (/^\/+[a-z][a-z0-9+.-]*:/i.test(raw)) return '/'
  return raw
}

async function submit() {
  if (!password.value) { err.value = 'Введите пароль'; return }
  busy.value = true; err.value = ''
  const ok = await auth.login(password.value)
  busy.value = false
  if (ok) {
    toasts.success('Вход выполнен')
    router.replace(safeRedirect(route.query.to))
  } else {
    err.value = 'Неверный пароль'
  }
}

const today = new Date().toLocaleDateString('ru-RU', {
  weekday: 'long', day: 'numeric', month: 'long',
}).toUpperCase()
</script>

<template>
  <main class="min-h-full grid place-items-center px-6 py-12">
    <div class="w-full max-w-md space-y-8">
      <header class="space-y-3 animate-rise">
        <div class="flex items-center gap-3">
          <img
            src="/logo.png"
            alt="Amnezia"
            class="h-10 w-10 rounded-xl object-contain invert dark:invert-0"
            draggable="false"
          />
          <div class="leading-tight">
            <div class="text-[15.5px] text-ink-900 font-semibold tracking-tight">Amnezia</div>
            <div class="eyebrow -mt-px">Panel</div>
          </div>
        </div>

        <div class="eyebrow tnum">{{ today }}</div>
        <h1 class="num-display text-ink-900 text-[44px]">
          Вход
        </h1>
        <p class="text-[13px] text-ink-500 leading-relaxed max-w-sm">
          Введите пароль для доступа к панели.
        </p>
      </header>

      <form @submit.prevent="submit" class="card p-6 sm:p-7 space-y-5 animate-rise delay-2">
        <Field label="Пароль" :error="err">
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
              class="absolute inset-y-0 right-0 px-3 text-ink-500 hover:text-ink-900 transition-colors"
              @click="reveal = !reveal"
              :aria-label="reveal ? 'Скрыть пароль' : 'Показать пароль'"
            >
              <Icon :name="reveal ? 'eye-off' : 'eye'" :size="16" />
            </button>
          </div>
        </Field>

        <Button type="submit" variant="primary" block :loading="busy">Войти</Button>
      </form>

      <p class="text-center eyebrow">
        Сессия в защищённой cookie
      </p>
    </div>
  </main>
</template>
