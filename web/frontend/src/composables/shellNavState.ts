import { computed, ref } from 'vue'

/** 控制台/管理端移动导航抽屉是否打开（供全局 FAB 等避让）。 */
export const shellNavDrawerOpen = ref(false)

/** 营销首页移动导航抽屉是否打开。 */
export const siteNavDrawerOpen = ref(false)

export const globalFabHidden = computed(() => shellNavDrawerOpen.value || siteNavDrawerOpen.value)
