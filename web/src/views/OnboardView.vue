<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '@/lib/api'
import type { OnboardRedeemResult } from '@/types'

const route = useRoute()
const token = computed(() => String(route.params.token || ''))

type Phase = 'loading' | 'invalid' | 'used' | 'ready' | 'submitting' | 'done' | 'error'
const phase = ref<Phase>('loading')
const err = ref('')

const clientName = ref('')
const snippet = ref('')
const result = ref<OnboardRedeemResult | null>(null)

onMounted(async () => {
  try {
    const s = await api.onboardStatus(token.value)
    if (s.used) { phase.value = 'used'; return }
    if (!s.valid) { phase.value = 'invalid'; return }
    phase.value = 'ready'
  } catch {
    phase.value = 'invalid'
  }
})

async function submit() {
  err.value = ''
  if (!snippet.value.trim()) { err.value = 'Вставьте блок [Interface] из генератора'; return }
  phase.value = 'submitting'
  try {
    result.value = await api.onboardRedeem(token.value, {
      snippet: snippet.value.trim(),
      clientName: clientName.value.trim() || 'client',
    })
    phase.value = 'done'
  } catch (e: any) {
    err.value = e?.message || 'Ошибка'
    phase.value = 'error'
  }
}

function downloadConf() {
  if (!result.value) return
  const blob = new Blob([result.value.conf], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${(clientName.value.trim() || 'client').replace(/[^a-zA-Z0-9-_]+/g, '-')}.conf`
  document.body.appendChild(a); a.click(); a.remove()
  URL.revokeObjectURL(url)
}

async function copyConf() {
  if (!result.value) return
  try { await navigator.clipboard.writeText(result.value.conf) } catch { /* ignore */ }
}
</script>

<template>
  <div class="min-h-screen bg-ink-50 text-ink-900 flex items-start sm:items-center justify-center py-8 px-4">
    <div class="w-full max-w-2xl">

      <header class="text-center mb-8">
        <div class="eyebrow">AmneziaWG</div>
        <h1 class="num-display text-[36px] sm:text-[44px] mt-1">Настройка подключения</h1>
      </header>

      <!-- Loading -->
      <div v-if="phase === 'loading'" class="rounded-2xl bg-white shadow-card border border-ink-900/5 p-8 text-center text-ink-500">
        Проверяем приглашение…
      </div>

      <!-- Invalid / Used -->
      <div v-else-if="phase === 'invalid' || phase === 'used'" class="rounded-2xl bg-white shadow-card border border-ink-900/5 p-8 space-y-3">
        <div class="text-[15px] font-semibold text-danger">
          {{ phase === 'used' ? 'Это приглашение уже использовано' : 'Приглашение недействительно' }}
        </div>
        <p class="text-[13px] text-ink-500 leading-relaxed">
          {{ phase === 'used'
            ? 'Конфиг был выдан один раз — повторное получение невозможно. Если вы не получили его или потеряли, попросите администратора выпустить новое приглашение.'
            : 'Ссылка не существует, отозвана или истёк срок действия. Свяжитесь с администратором.' }}
        </p>
      </div>

      <!-- Form -->
      <div v-else-if="phase === 'ready' || phase === 'submitting' || phase === 'error'"
           class="rounded-2xl bg-white shadow-card border border-ink-900/5 p-6 sm:p-8 space-y-5">
        <div class="space-y-2">
          <p class="text-[13px] text-ink-700 leading-relaxed">
            Сгенерируйте параметры обфускации в
            <a class="underline" target="_blank" rel="noopener" href="https://vadim-khristenko.github.io/AmneziaWG-Architect/">AmneziaWG-Architect</a>
            (выберите версию AWG 2.0), скопируйте блок <code class="mono">[Interface]</code> и вставьте сюда.
            Ключи и адреса сервер подставит сам.
          </p>
          <p class="text-[12px] text-ink-500">
            ⚠ Конфиг выдаётся один раз — сразу сохраните его на устройство.
          </p>
        </div>

        <div class="space-y-1.5">
          <label class="text-[12px] text-ink-700 font-medium">Имя устройства (необязательно)</label>
          <input
            v-model="clientName"
            class="w-full p-2.5 rounded-lg bg-ink-100/60 border border-ink-900/10 text-[13px] focus-ring"
            placeholder="iPhone 15 / MacBook / etc."
          />
        </div>

        <div class="space-y-1.5">
          <label class="text-[12px] text-ink-700 font-medium">Блок [Interface] из генератора</label>
          <textarea
            v-model="snippet"
            rows="14"
            class="w-full p-3 rounded-lg bg-ink-100/60 border border-ink-900/10 text-[11.5px] mono leading-snug focus-ring"
            :placeholder="`[Interface]\nJc = 4\nJmin = 362\nJmax = 943\nS1 = 43\nS2 = 65\nS3 = 35\nS4 = 28\nH1 = 320858491-320865164\nH2 = 1445464973-1445512660\nH3 = 3235131120-3235164350\nH4 = 3875042355-3875063814\nItime = 60\nI1 = <b 0x...><r 28><t><rc 12>`"
          />
        </div>

        <p v-if="err" class="text-[12.5px] text-danger">{{ err }}</p>

        <button
          class="w-full py-3 rounded-lg bg-ink-900 text-white text-[14px] font-medium disabled:opacity-50 hover:bg-ink-800 transition"
          :disabled="phase === 'submitting'"
          @click="submit"
        >
          {{ phase === 'submitting' ? 'Создаём подключение…' : 'Получить конфиг' }}
        </button>
      </div>

      <!-- Done -->
      <div v-else-if="phase === 'done' && result" class="rounded-2xl bg-white shadow-card border border-ink-900/5 p-6 sm:p-8 space-y-6">
        <div class="space-y-1">
          <div class="text-success text-[13px] font-medium">✓ Подключение создано</div>
          <p class="text-[12.5px] text-ink-500 leading-relaxed">
            Отсканируйте QR-код в приложении AmneziaWG / WireGuard, либо скачайте <span class="mono">.conf</span> и импортируйте вручную.
            Эта страница больше не покажет конфиг — сохраните сейчас.
          </p>
        </div>

        <div class="grid sm:grid-cols-[auto_1fr] gap-5 items-start">
          <div class="bg-white border border-ink-900/10 rounded-xl p-3 mx-auto sm:mx-0">
            <img :src="`data:image/png;base64,${result.qrPng64}`" alt="QR" class="block w-[220px] h-[220px]" />
          </div>
          <div class="space-y-3 min-w-0">
            <button
              class="w-full py-2.5 rounded-lg bg-ink-900 text-white text-[13px] font-medium hover:bg-ink-800 transition"
              @click="downloadConf"
            >
              ⬇ Скачать .conf
            </button>
            <button
              class="w-full py-2.5 rounded-lg bg-ink-100 text-ink-900 text-[13px] font-medium hover:bg-ink-200 transition"
              @click="copyConf"
            >
              📋 Скопировать конфиг
            </button>
            <details class="text-[11.5px]">
              <summary class="cursor-pointer text-ink-500 select-none">Показать .conf</summary>
              <pre class="mt-2 p-3 rounded-lg bg-ink-100/60 border border-ink-900/10 mono text-[10.5px] leading-snug overflow-x-auto whitespace-pre">{{ result.conf }}</pre>
            </details>
          </div>
        </div>
      </div>

    </div>
  </div>
</template>
