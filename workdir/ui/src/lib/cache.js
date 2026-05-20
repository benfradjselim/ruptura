/**
 * Stale-while-revalidate cache for Ruptura API responses.
 *
 * On the first call: fetches immediately (no stale data available).
 * On subsequent calls: returns cached data instantly, fetches fresh data in the background,
 * then notifies subscribers via the returned store when the update lands.
 *
 * Usage:
 *   import { swr } from './cache.js'
 *   const { data, refresh } = swr('fleet', () => api.fleet(), 15_000)
 *   // data is a Svelte readable store — subscribe or use $data in templates.
 */

import { writable } from 'svelte/store'

const store = new Map() // key → { value, ts, subscribers }

/**
 * Returns a reactive Svelte store backed by stale-while-revalidate caching.
 *
 * @param {string}   key     Cache key (unique per endpoint).
 * @param {function} fetcher Async function that returns the fresh data.
 * @param {number}   ttl     How long (ms) before a background revalidation is triggered.
 *                           Data is ALWAYS served from cache when available; ttl only
 *                           controls how often the background fetch runs.
 * @returns {{ data: import('svelte/store').Readable, refresh: function }}
 */
export function swr(key, fetcher, ttl = 15_000) {
  if (!store.has(key)) {
    store.set(key, { value: null, ts: 0, pending: false, svelte: writable(null) })
  }
  const entry = store.get(key)

  async function revalidate() {
    if (entry.pending) return
    entry.pending = true
    try {
      const fresh = await fetcher()
      entry.value = fresh
      entry.ts = Date.now()
      entry.svelte.set(fresh)
    } catch (_) {
      // Keep stale value on error; caller sees no disruption.
    } finally {
      entry.pending = false
    }
  }

  const age = Date.now() - entry.ts
  if (entry.value === null) {
    // Cold start — must wait for first load.
    revalidate()
  } else if (age > ttl) {
    // Stale — serve cached data immediately, revalidate in background.
    revalidate()
  }

  return {
    /** Svelte readable store containing the latest data (null during cold start). */
    data: { subscribe: entry.svelte.subscribe },
    /** Force an immediate revalidation (e.g. on pull-to-refresh). */
    refresh: revalidate,
  }
}

/**
 * Invalidate a cache entry so the next swr() call triggers a fresh fetch.
 * Use after mutations (create/update/delete).
 */
export function invalidate(key) {
  if (store.has(key)) {
    store.get(key).ts = 0
  }
}

/** Clear all cached entries (e.g. on logout). */
export function clearAll() {
  store.clear()
}
