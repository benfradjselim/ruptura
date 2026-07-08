import { writable, derived } from 'svelte/store'
import { getToken, setToken } from './api.js'

export const token = writable(getToken())
export const user = writable(null)
export const currentPage = writable('dashboard')
export const alerts = writable([])
export const kpis = writable({})
export const wsConnected = writable(false)

token.subscribe((t) => setToken(t))

export const isLoggedIn = derived(token, ($t) => !!$t)

// ── Theme store (dark / light) ─────────────────────────────────────────────
// Persisted in localStorage. Applied to <html data-theme="dark|light">.
const savedTheme = typeof localStorage !== 'undefined'
  ? (localStorage.getItem('ruptura_theme') || 'dark')
  : 'dark'

export const theme = writable(savedTheme)

theme.subscribe((t) => {
  if (typeof localStorage !== 'undefined') localStorage.setItem('ruptura_theme', t)
  if (typeof document !== 'undefined') document.documentElement.setAttribute('data-theme', t)
})

// Apply on boot
if (typeof document !== 'undefined') {
  document.documentElement.setAttribute('data-theme', savedTheme)
}

// ── Infra stores ──────────────────────────────────────────────────────────
export const infraGroups     = writable([])
export const infraPropagation = writable({})
