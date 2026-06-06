<script setup lang="ts">
/*
  Простой скелетон — серый блок с пульсом. Управляется через width/height/class.
  Дочерние компоненты могут заворачивать его в собственные карточки/строки.
*/
withDefaults(defineProps<{
  width?: string | number
  height?: string | number
  rounded?: 'sm' | 'md' | 'lg' | 'full'
}>(), { rounded: 'md' })

const radius = {
  sm: '4px',
  md: '6px',
  lg: '12px',
  full: '9999px',
} as const
</script>

<template>
  <span
    class="skeleton block"
    :style="{
      width:  typeof width  === 'number' ? width  + 'px' : (width  || '100%'),
      height: typeof height === 'number' ? height + 'px' : (height || '12px'),
      borderRadius: radius[rounded],
    }"
  />
</template>
