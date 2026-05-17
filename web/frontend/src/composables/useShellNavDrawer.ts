import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { shellNavDrawerOpen } from './shellNavState'

export function useShellNavDrawer() {
  const open = ref(false)
  const route = useRoute()

  function close() {
    open.value = false
  }

  function openDrawer() {
    open.value = true
  }

  watch(open, (v) => {
    shellNavDrawerOpen.value = v
  })

  watch(
    () => route.path,
    () => close(),
  )

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && open.value) close()
  }

  watch(open, (v) => {
    if (typeof document === 'undefined') return
    document.body.style.overflow = v ? 'hidden' : ''
  })

  onMounted(() => {
    document.addEventListener('keydown', onKeydown)
  })

  onBeforeUnmount(() => {
    document.removeEventListener('keydown', onKeydown)
    shellNavDrawerOpen.value = false
    if (typeof document !== 'undefined') document.body.style.overflow = ''
  })

  return { open, close, openDrawer }
}
