<script setup lang="ts">
import { ref, computed } from 'vue'
import { useToastStore } from '@/stores/toasts'
import Button from '@/components/atoms/Button.vue'
import Icon from '@/components/atoms/Icon.vue'

/*
  SplitTunnelPicker — наполняет AllowedIPs списком подсетей выбранных сервисов
  из iplist.opencck.org. Это публичный реестр, который ведёт сообщество
  (https://github.com/rekryt/iplist). Идея split-tunnel: в туннель уходит
  только трафик к выбранным заблокированным сервисам, остальное — мимо.

  v-model:value — текущая строка AllowedIPs (та же, что в инпуте над пикером).
  При "Применить" дёргаем JSON по выбранным сервисам, флэтим, проставляем.

  Без сохранения preset'ов между визитами — это намеренно: список IP меняется,
  правильно перегенерировать вручную, а не разморозить кэш.
*/

const props = defineProps<{ value: string }>()
const emit = defineEmits<{ (e: 'update:value', v: string): void }>()

const toasts = useToastStore()

// Курируемый список — то, что реально нужно русскоязычной аудитории.
// Полный реестр iplist'а ~300 сервисов; их перечисление в UI было бы шумом.
// Если кому-то нужен экзотический сервис — он напишет CIDR'ы руками.
interface Service { id: string; label: string }
const SERVICES: Service[] = [
  { id: 'telegram',  label: 'Telegram'        },
  { id: 'discord',   label: 'Discord'         },
  { id: 'youtube',   label: 'YouTube'         },
  { id: 'openai',    label: 'OpenAI · ChatGPT'},
  { id: 'instagram', label: 'Instagram'       },
  { id: 'twitter',   label: 'X · Twitter'     },
  { id: 'meta',      label: 'Meta · Facebook' },
  { id: 'whatsapp',  label: 'WhatsApp'        },
  { id: 'tiktok',    label: 'TikTok'          },
  { id: 'signal',    label: 'Signal'          },
  { id: 'spotify',   label: 'Spotify'         },
  { id: 'linkedin',  label: 'LinkedIn'        },
  { id: 'github',    label: 'GitHub'          },
  { id: 'protonmail',label: 'Proton'          },
]

const picked = ref<Set<string>>(new Set())
const busy = ref(false)
const summary = ref<{ services: number; cidrs: number } | null>(null)

function toggle(id: string) {
  const next = new Set(picked.value)
  if (next.has(id)) next.delete(id); else next.add(id)
  picked.value = next
}
function pickAll() { picked.value = new Set(SERVICES.map(s => s.id)) }
function clearAll() { picked.value = new Set() }

const canApply = computed(() => picked.value.size > 0 && !busy.value)

async function apply() {
  if (!canApply.value) return
  busy.value = true
  try {
    const site = [...picked.value].join(',')
    const url  = `https://iplist.opencck.org/?format=json&data=cidr4&site=${encodeURIComponent(site)}`
    const res  = await fetch(url, { headers: { Accept: 'application/json' } })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const data = await res.json() as Record<string, string[]>
    // Flatten, dedupe — некоторые подсети пересекаются между сервисами.
    const all = new Set<string>()
    for (const v of Object.values(data)) for (const cidr of v) all.add(cidr)
    if (!all.size) throw new Error('Пустой ответ от iplist')
    const value = [...all].sort().join(', ')
    emit('update:value', value)
    summary.value = { services: picked.value.size, cidrs: all.size }
    toasts.success(`Загружено: ${all.size} подсетей из ${picked.value.size} сервисов`)
  } catch (e: any) {
    toasts.error(e?.message?.includes('Failed to fetch')
      ? 'Не удалось связаться с iplist.opencck.org — проверьте интернет'
      : `Ошибка: ${e?.message || 'не удалось получить список'}`)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="rounded-2xl bg-ink-100/60 p-4 space-y-3.5">
    <!-- Header -->
    <div class="flex items-start gap-3">
      <div class="flex-1 min-w-0 space-y-1">
        <div class="eyebrow flex items-center gap-2">
          <Icon name="shield" :size="11" />
          Раздельное туннелирование
        </div>
        <p class="text-[11.5px] text-ink-500 leading-relaxed">
          В туннель уйдёт только трафик к выбранным сервисам — заменит значение Allowed IPs выше.
          Источник:
          <a
            href="https://github.com/rekryt/iplist"
            target="_blank" rel="noopener"
            class="text-ink-700 hover:text-ink-900 underline decoration-ink-300 underline-offset-2"
          >iplist</a>.
        </p>
      </div>
      <div class="flex items-center gap-1 shrink-0">
        <button
          type="button"
          class="eyebrow text-ink-500 hover:text-ink-900 transition-colors px-1.5"
          @click="picked.size === SERVICES.length ? clearAll() : pickAll()"
        >{{ picked.size === SERVICES.length ? 'снять всё' : 'выбрать всё' }}</button>
      </div>
    </div>

    <!-- Service chips -->
    <div class="flex flex-wrap gap-1.5">
      <button
        v-for="s in SERVICES" :key="s.id"
        type="button"
        :aria-pressed="picked.has(s.id)"
        @click="toggle(s.id)"
        :class="[
          'h-7 px-3 rounded-lg text-[12px] font-medium tracking-chrome transition-colors focus-ring',
          picked.has(s.id)
            ? 'bg-ink-900 text-ink-50'
            : 'bg-ink-100 text-ink-700 hover:bg-ink-200',
        ]"
      >{{ s.label }}</button>
    </div>

    <!-- Apply row -->
    <div class="flex items-center justify-between gap-3 pt-1">
      <div class="text-[11.5px] text-ink-500 mono tnum">
        <template v-if="summary">
          ✓ {{ summary.services }} сервис(а) · {{ summary.cidrs }} подсетей применены
        </template>
        <template v-else-if="picked.size">
          выбрано: {{ picked.size }} из {{ SERVICES.length }}
        </template>
        <template v-else>—</template>
      </div>
      <Button
        size="sm"
        variant="primary"
        :disabled="!canApply"
        :loading="busy"
        @click="apply"
      >
        <Icon name="download" :size="13" />
        {{ busy ? 'Загружаем…' : 'Применить' }}
      </Button>
    </div>
  </div>
</template>
