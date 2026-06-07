<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import Modal from '@/components/molecules/Modal.vue'
import Field from '@/components/molecules/Field.vue'
import Input from '@/components/atoms/Input.vue'
import Button from '@/components/atoms/Button.vue'
import type { ProfileInfo } from '@/types'

const props = defineProps<{
  open: boolean
  mode: 'create' | 'edit'
  profile?: ProfileInfo | null
  busy?: boolean
}>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', body: { id?: string; name: string; description?: string; snippet?: string }): void
}>()

const id = ref('')
const name = ref('')
const description = ref('')
const snippet = ref('')
const err = ref('')

// In edit mode the snippet is optional — leaving it empty means "keep current
// obfuscation params". Reconstructing the current snippet for display would
// require server-side rendering; we just show the existing summary instead.
const currentSummary = computed(() => {
  const p = props.profile
  if (!p) return ''
  return [
    `Jc=${p.jc}  Jmin=${p.jmin}  Jmax=${p.jmax}`,
    `S1=${p.s1}  S2=${p.s2}  S3=${p.s3}  S4=${p.s4}`,
    `H1=${p.h1}`,
    `H2=${p.h2}`,
    `H3=${p.h3}`,
    `H4=${p.h4}`,
    `Itime=${p.itime}`,
  ].join('\n')
})

watch(() => props.open, (v) => {
  if (!v) return
  err.value = ''
  snippet.value = ''
  if (props.mode === 'edit' && props.profile) {
    id.value = props.profile.id
    name.value = props.profile.name
    description.value = props.profile.description || ''
  } else {
    id.value = ''
    name.value = ''
    description.value = ''
  }
})

function submit() {
  const n = name.value.trim()
  if (!n) { err.value = 'Введите имя'; return }
  if (props.mode === 'create' && id.value.trim() && !/^[a-z0-9-]{2,32}$/.test(id.value.trim())) {
    err.value = 'ID: 2–32 символа, только a–z, 0–9, дефис'
    return
  }
  const trimmed = snippet.value.trim()
  if (props.mode === 'create' && !trimmed) {
    err.value = 'Вставьте [Interface]-блок из генератора AmneziaWG-Architect'
    return
  }
  emit('submit', {
    id: props.mode === 'create' ? id.value.trim() || undefined : undefined,
    name: n,
    description: description.value.trim() || undefined,
    snippet: trimmed || undefined,
  })
}
</script>

<template>
  <Modal :open="open" size="lg" :title="mode === 'create' ? 'Новый профиль подключения' : 'Редактирование профиля'" @close="emit('close')">
    <div class="space-y-4">
      <p class="text-[12.5px] text-ink-500 leading-relaxed">
        Профиль — отдельный AmneziaWG 2.0 интерфейс на своём UDP-порту. Параметры обфускации (Jc/J/S/H/I/Itime)
        вставляются единым блоком из внешнего генератора —
        <a class="underline" target="_blank" rel="noopener" href="https://vadim-khristenko.github.io/AmneziaWG-Architect/">AmneziaWG-Architect</a>.
        Сервер сам сгенерирует ключи и адреса.
      </p>

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <Field label="Имя" hint="Видно админу в списке профилей.">
          <Input v-model="name" autofocus placeholder="Mimicry QUIC" />
        </Field>
        <Field v-if="mode === 'create'" label="ID (необязательно)" hint="2–32 символа, a–z, 0–9, дефис. Если пусто — сгенерируется.">
          <Input v-model="id" placeholder="mimicry-quic" />
        </Field>
        <Field v-else label="ID" hint="ID нельзя поменять.">
          <Input :model-value="id" disabled />
        </Field>
      </div>

      <Field label="Описание (необязательно)">
        <Input v-model="description" placeholder="Для устройств за DPI" />
      </Field>

      <Field
        :label="mode === 'create' ? 'Snippet [Interface]' : 'Новый snippet [Interface] (необязательно)'"
        :hint="mode === 'create'
          ? 'Скопируйте блок [Interface] из AmneziaWG-Architect и вставьте сюда. Поля PrivateKey/Address/ListenPort и комментарии будут проигнорированы — их подставит сервер.'
          : 'Если оставить пусто — параметры не изменятся. Вставка нового блока полностью заменит обфускацию профиля.'"
      >
        <textarea
          v-model="snippet"
          rows="12"
          class="w-full p-3.5 rounded-2xl bg-ink-100 text-[11.5px] mono leading-snug outline-none transition-colors duration-150 focus:bg-amber-50 dark:focus:bg-amber-400/10"
          :placeholder="`[Interface]\nJc = 4\nJmin = 362\nJmax = 943\nS1 = 43\nS2 = 65\nS3 = 35\nS4 = 28\nH1 = 320858491-320865164\nH2 = 1445464973-1445512660\nH3 = 3235131120-3235164350\nH4 = 3875042355-3875063814\nItime = 60\nI1 = <b 0x...><r 28><t><rc 12>`"
        />
      </Field>

      <div v-if="mode === 'edit' && currentSummary" class="p-3 rounded-lg bg-ink-100/40 space-y-1">
        <div class="text-[11px] uppercase tracking-[0.12em] text-ink-500">Текущая обфускация</div>
        <pre class="mono text-[11px] text-ink-700 whitespace-pre-wrap leading-relaxed">{{ currentSummary }}</pre>
      </div>

      <p v-if="err" class="text-[12px] text-danger">{{ err }}</p>
    </div>

    <template #footer>
      <Button variant="ghost" size="sm" @click="emit('close')">Отмена</Button>
      <Button variant="primary" size="sm" :loading="busy" @click="submit">
        {{ mode === 'create' ? 'Создать' : 'Сохранить' }}
      </Button>
    </template>
  </Modal>
</template>
