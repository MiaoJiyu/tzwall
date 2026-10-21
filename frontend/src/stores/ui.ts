import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useUiStore = defineStore('ui', () => {
  const theme = ref<string>(localStorage.getItem('tzwall_theme') || 'auto')
  const isDark = ref<boolean>(false)
  const watermark = ref<{ username: string; ip: string }>({ username: '游客', ip: '' })

  function apply() {
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
    isDark.value = theme.value === 'dark' || (theme.value === 'auto' && prefersDark)
    document.documentElement.classList.toggle('dark', isDark.value)
  }
  function set(t: string) {
    theme.value = t
    localStorage.setItem('tzwall_theme', t)
    apply()
  }
  function toggle() {
    set(isDark.value ? 'light' : 'dark')
  }
  return { theme, isDark, watermark, apply, set, toggle }
})
