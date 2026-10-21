import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string>(localStorage.getItem('token') || '')
  const user = ref<any>(JSON.parse(localStorage.getItem('tzwall_user') || 'null'))
  const isLoggedIn = computed(() => !!token.value)
  const role = computed(() => user.value?.role || 'user')
  const isAdmin = computed(() => ['admin', 'super_admin'].includes(role.value))

  function set(tokenVal: string, userVal: any) {
    token.value = tokenVal
    user.value = userVal
    localStorage.setItem('token', tokenVal)
    localStorage.setItem('tzwall_user', JSON.stringify(userVal))
  }
  function setUser(u: any) {
    user.value = u
    localStorage.setItem('tzwall_user', JSON.stringify(u))
  }
  function logout() {
    token.value = ''
    user.value = null
    localStorage.removeItem('token')
    localStorage.removeItem('tzwall_user')
  }
  return { token, user, isLoggedIn, role, isAdmin, set, setUser, logout }
})
