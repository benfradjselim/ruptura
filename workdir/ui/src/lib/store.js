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

// KPI severity helpers
export function kpiColor(name, value) {
  const thresholds = {
    stress: [0.3, 0.6, 0.8],
    fatigue: [0.3, 0.6, 0.8],
    mood: [0.8, 0.5, 0.3],    // reversed: high mood is good
    pressure: [0.3, 0.6, 0.8],
    humidity: [0.3, 0.6, 0.8],
    contagion: [0.3, 0.6, 0.8],
  }
  const t = thresholds[name] || [0.3, 0.6, 0.8]
  if (name === 'mood') {
    if (value >= t[0]) return '#22c55e'
    if (value >= t[1]) return '#eab308'
    if (value >= t[2]) return '#f97316'
    return '#ef4444'
  }
  if (value <= t[0]) return '#22c55e'
  if (value <= t[1]) return '#eab308'
  if (value <= t[2]) return '#f97316'
  return '#ef4444'
}

export function kpiLabel(name, value) {
  const labels = {
    stress: ['calm', 'nervous', 'stressed', 'panic'],
    fatigue: ['rested', 'tired', 'exhausted', 'burnout'],
    mood: ['depressed', 'sad', 'neutral', 'content', 'happy'],
    pressure: ['stable', 'rising', 'storm_approaching', 'improving'],
    humidity: ['dry', 'humid', 'very_humid', 'storm'],
    contagion: ['low', 'moderate', 'epidemic', 'pandemic'],
  }
  const l = labels[name] || ['low', 'medium', 'high', 'critical']
  const idx = Math.min(Math.floor(value * l.length), l.length - 1)
  return l[idx]
}
