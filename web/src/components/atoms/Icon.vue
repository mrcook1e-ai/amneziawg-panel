<script setup lang="ts">
// Tiny inline icon set. Add cases as needed; SVG path data lives here so we
// don't pull a runtime icon library and every icon stays one DOM node.

type Name =
  | 'plus' | 'trash' | 'copy' | 'qrcode' | 'download' | 'edit'
  | 'check' | 'x' | 'logout' | 'shield' | 'eye' | 'eye-off' | 'chevron-down'

const props = withDefaults(defineProps<{ name: Name; size?: number }>(), { size: 16 })

const paths: Record<Name, string> = {
  'plus':         'M12 5v14M5 12h14',
  'trash':        'M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6M10 11v6M14 11v6',
  'copy':         'M9 9V5a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2h-4M5 9h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-8a2 2 0 0 1 2-2Z',
  'qrcode':       'M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h3v3h-3zM18 18h3v3h-3zM14 19h2v2h-2z',
  'download':     'M12 3v12M7 10l5 5 5-5M5 21h14',
  'edit':         'M16 3l5 5-11 11H5v-5L16 3zM14 5l5 5',
  'check':        'M5 12l5 5L20 7',
  'x':            'M6 6l12 12M18 6L6 18',
  'logout':       'M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4M10 17l-5-5 5-5M5 12h12',
  'shield':       'M12 3l8 3v6c0 5-4 8-8 9-4-1-8-4-8-9V6l8-3z',
  'eye':          'M2 12s4-7 10-7 10 7 10 7-4 7-10 7S2 12 2 12zM12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z',
  'eye-off':      'M3 3l18 18M10.6 6.1A10 10 0 0 1 12 6c6 0 10 6 10 6a17 17 0 0 1-3.2 3.9M6.2 6.2A17 17 0 0 0 2 12s4 7 10 7c1 0 1.9-.1 2.8-.4M9.9 14.1A3 3 0 0 1 12 9',
  'chevron-down': 'M6 9l6 6 6-6',
}
</script>

<template>
  <svg
    :width="size" :height="size" viewBox="0 0 24 24"
    fill="none" stroke="currentColor" stroke-width="1.75"
    stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"
  >
    <path :d="paths[props.name]" />
  </svg>
</template>
