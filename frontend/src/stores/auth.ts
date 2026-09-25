import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { login as requestLogin } from '../api/auth'
import { tokenKey, userKey } from '../api/client'
import type { LoginRequest, SessionUser, UserRole } from '../types/auth'

function storedUser(): SessionUser | null {
  try { return JSON.parse(localStorage.getItem(userKey) ?? 'null') as SessionUser | null }
  catch { return null }
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem(tokenKey) ?? '')
  const user = ref<SessionUser | null>(storedUser())
  const authenticated = computed(() => Boolean(token.value && user.value))
  async function login(input: LoginRequest) {
    const session = await requestLogin(input)
    token.value = session.token
    user.value = session.user
    localStorage.setItem(tokenKey, token.value)
    localStorage.setItem(userKey, JSON.stringify(user.value))
  }
  function logout() {
    token.value = ''
    user.value = null
    localStorage.removeItem(tokenKey)
    localStorage.removeItem(userKey)
  }
  function hasRole(...roles: UserRole[]) { return Boolean(user.value && roles.includes(user.value.role)) }
  return { token, user, authenticated, login, logout, hasRole }
})
