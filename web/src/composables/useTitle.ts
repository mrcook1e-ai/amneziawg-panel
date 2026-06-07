import { watchEffect, onBeforeUnmount } from 'vue'

/*
  useTitle — реактивное управление <title> вкладки. Принимает getter, чтобы
  работать с ref'ами/computed без явной разрядки. На размонтировании
  компонента возвращает предыдущее значение, чтобы при возврате на другой
  экран не оставался чужой title.
*/
export function useTitle(getter: () => string) {
  const prev = typeof document !== 'undefined' ? document.title : ''
  watchEffect(() => {
    const t = getter()
    if (typeof document !== 'undefined' && t) document.title = t
  })
  onBeforeUnmount(() => {
    if (typeof document !== 'undefined') document.title = prev
  })
}
