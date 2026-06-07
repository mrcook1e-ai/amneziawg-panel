<script setup lang="ts">
/*
  Обёртка над lucide-vue-next. Иконки — официальный Vue-пакет Lucide (те же
  иконки что на react-icons/lu). Vite tree-shake'ит неиспользованные.

  Контракт <Icon name="..."> сохранён — call-site не меняется.
*/

import {
  Plus,
  Trash2,
  Copy,
  QrCode,
  Download,
  Upload,
  Pencil,
  Check,
  X,
  LogOut,
  Shield,
  Eye,
  EyeOff,
  ChevronDown,
  ChevronRight,
  ChevronLeft,
  Settings,
  RefreshCw,
  Power,
  Info,
  Sun,
  Moon,
  Sparkles,
  Search,
} from 'lucide-vue-next'

type Name =
  | 'plus' | 'trash' | 'copy' | 'qrcode' | 'download' | 'upload' | 'edit'
  | 'check' | 'x' | 'logout' | 'shield' | 'eye' | 'eye-off'
  | 'chevron-down' | 'chevron-right' | 'chevron-left'
  | 'settings' | 'refresh' | 'power' | 'info'
  | 'sun' | 'moon' | 'sparkles' | 'search'

const props = withDefaults(defineProps<{
  name: Name
  size?: number
  stroke?: number
}>(), { size: 16, stroke: 1.75 })

const map: Record<Name, any> = {
  'plus':          Plus,
  'trash':         Trash2,
  'copy':          Copy,
  'qrcode':        QrCode,
  'download':      Download,
  'upload':        Upload,
  'edit':          Pencil,
  'check':         Check,
  'x':             X,
  'logout':        LogOut,
  'shield':        Shield,
  'eye':           Eye,
  'eye-off':       EyeOff,
  'chevron-down':  ChevronDown,
  'chevron-right': ChevronRight,
  'chevron-left':  ChevronLeft,
  'settings':      Settings,
  'refresh':       RefreshCw,
  'power':         Power,
  'info':          Info,
  'sun':           Sun,
  'moon':          Moon,
  'sparkles':      Sparkles,
  'search':        Search,
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
