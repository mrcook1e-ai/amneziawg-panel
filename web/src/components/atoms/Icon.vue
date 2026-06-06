<script setup lang="ts">
/*
  Обёртка над @tabler/icons-vue. Иконки приходят как отдельные Vue-компоненты,
  Vite tree-shake'ит неиспользованные — в бандл попадает только то, что мы
  упомянули ниже в карте `map`.

  Зачем обёртка, а не прямой импорт по месту:
    1. Единая точка для размера / stroke-width / окраски — не таскаем
       `:size="16" :stroke="1.5"` в каждый шаблон.
    2. Уже существующие <Icon name="..."> по всему коду продолжают работать,
       без правок 60+ файлов.
    3. Можно подменить любую иконку на свой SVG не трогая call-site (раньше
       так и делали — этот контракт оставляем).
*/

import {
  IconPlus,
  IconTrash,
  IconCopy,
  IconQrcode,
  IconDownload,
  IconUpload,
  IconPencil,
  IconCheck,
  IconX,
  IconLogout,
  IconShield,
  IconEye,
  IconEyeOff,
  IconChevronDown,
  IconChevronRight,
  IconChevronLeft,
  IconSettings,
  IconRefresh,
  IconPower,
  IconInfoCircle,
  IconSun,
  IconMoon,
  IconSparkles,
  IconSearch,
} from '@tabler/icons-vue'

type Name =
  | 'plus' | 'trash' | 'copy' | 'qrcode' | 'download' | 'upload' | 'edit'
  | 'check' | 'x' | 'logout' | 'shield' | 'eye' | 'eye-off'
  | 'chevron-down' | 'chevron-right' | 'chevron-left'
  | 'settings' | 'refresh' | 'power' | 'info'
  | 'sun' | 'moon' | 'sparkles' | 'search'

const props = withDefaults(defineProps<{
  name: Name
  size?: number
  /** Толщина обводки Tabler. 1.5 — спокойная, 2 — стандартная. */
  stroke?: number
}>(), { size: 16, stroke: 1.75 })

// Маппинг внутренних имён → компонент Tabler. Если потребуется заменить
// конкретную иконку (например, custom QR) — просто меняем правую часть.
const map: Record<Name, any> = {
  'plus':         IconPlus,
  'trash':        IconTrash,
  'copy':         IconCopy,
  'qrcode':       IconQrcode,
  'download':     IconDownload,
  'upload':       IconUpload,
  'edit':         IconPencil,
  'check':        IconCheck,
  'x':            IconX,
  'logout':       IconLogout,
  'shield':       IconShield,
  'eye':          IconEye,
  'eye-off':      IconEyeOff,
  'chevron-down':  IconChevronDown,
  'chevron-right': IconChevronRight,
  'chevron-left':  IconChevronLeft,
  'settings':     IconSettings,
  'refresh':      IconRefresh,
  'power':        IconPower,
  'info':         IconInfoCircle,
  'sun':          IconSun,
  'moon':         IconMoon,
  'sparkles':     IconSparkles,
  'search':       IconSearch,
}
</script>

<template>
  <component
    :is="map[props.name]"
    :size="props.size"
    :stroke-width="props.stroke"
    aria-hidden="true"
  />
</template>
