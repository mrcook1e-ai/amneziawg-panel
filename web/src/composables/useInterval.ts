import { onMounted, onBeforeUnmount } from 'vue'

export function useInterval(fn: () => void, ms: number, opts: { immediate?: boolean; pauseHidden?: boolean } = {}) {
  let id: number | undefined
  const start = () => { if (id == null) id = window.setInterval(fn, ms) }
  const stop = () => { if (id != null) { clearInterval(id); id = undefined } }
  const onVis = () => { document.hidden ? stop() : start() }

  onMounted(() => {
    if (opts.immediate) fn()
    if (opts.pauseHidden) document.addEventListener('visibilitychange', onVis)
    start()
  })
  onBeforeUnmount(() => {
    document.removeEventListener('visibilitychange', onVis)
    stop()
  })
  return { stop, start }
}
