// What the Rupture Horizon says about the engine itself: warm-up, confidence,
// blind spots and the data it reads.
import { formatEta } from './failures.js'

// Detected conditions come back within 5 minutes of a start, forecasts after
// about 30 minutes of readings.
export const DETECTION_WARMUP_SECONDS = 5 * 60
export const FORECAST_WARMUP_SECONDS = 30 * 60

const clock = t => new Date(t).toTimeString().slice(0, 5)

/**
 * @param {number | undefined} uptimeSeconds
 * @param {number} now milliseconds since the epoch
 * @returns {{ startedMinutes: number, detecting: boolean, forecastsAt: string } | null}
 */
export function warmup(uptimeSeconds, now) {
  if (uptimeSeconds === undefined || uptimeSeconds === null || uptimeSeconds >= FORECAST_WARMUP_SECONDS) return null
  return {
    startedMinutes: Math.floor(uptimeSeconds / 60),
    detecting: uptimeSeconds >= DETECTION_WARMUP_SECONDS,
    forecastsAt: clock(now + (FORECAST_WARMUP_SECONDS - uptimeSeconds) * 1000),
  }
}

// Only these modes measure their confidence; the others always report 1.
const MEASURED_CONFIDENCE = {
  saturation: 'trend agreement',
  recurrent_failure: 'rhythm regularity',
}

/** @param {{ failureMode: string, kind: string, confidence?: number }} failure */
export function confidenceLabel(failure) {
  const name = MEASURED_CONFIDENCE[failure.failureMode]
  if (failure.kind !== 'predicted' || !name || !(failure.confidence > 0)) return ''
  return `${name} ${Math.round(failure.confidence * 100)}%`
}

/** @param {{ etaEarlySeconds?: number, etaLateSeconds?: number }} failure */
export function etaRange(failure) {
  if (!failure.etaEarlySeconds) return ''
  if (!failure.etaLateSeconds) return `from ${formatEta(failure.etaEarlySeconds)}`
  return `${formatEta(failure.etaEarlySeconds)} to ${formatEta(failure.etaLateSeconds)}`
}

/**
 * Groups pods without a memory limit by namespace, largest group first.
 * @param {string[]} pods as namespace/pod
 * @returns {{ namespace: string, pods: string[] }[]}
 */
export function blindSpotsByNamespace(pods = []) {
  const groups = new Map()
  for (const pod of pods) {
    const [namespace, ...rest] = pod.split('/')
    groups.set(namespace, [...(groups.get(namespace) || []), rest.join('/')])
  }
  return [...groups.entries()]
    .map(([namespace, names]) => ({ namespace, pods: [...names].sort() }))
    .sort((a, b) => b.pods.length - a.pods.length || a.namespace.localeCompare(b.namespace))
}

const INPUT_LABELS = {
  'memory.usage': 'Memory usage',
  'memory.limit': 'Memory limit',
  'memory.request': 'Memory request',
  'cpu.usage': 'CPU usage',
  'cpu.limit': 'CPU limit',
  'cpu.request': 'CPU request',
  'cpu.throttled': 'CPU throttling',
  'container.restarts': 'Restarts',
  'container.waiting': 'Waiting reasons',
  'pod.ready': 'Pod readiness',
  'network.receive': 'Network in',
  'network.transmit': 'Network out',
}

/**
 * @param {{ role: string, targets: number, available: boolean }[]} inputs
 * @returns {{ role: string, label: string, targets: number, available: boolean }[]}
 */
export function readableInputs(inputs = []) {
  return inputs
    .map(input => ({ ...input, label: INPUT_LABELS[input.role] || input.role }))
    .sort((a, b) => Number(b.available) - Number(a.available) || b.targets - a.targets)
}

// A trend below this share of the limit per hour reads as flat.
const FLAT_TREND_PER_HOUR = 0.01

/**
 * One row of the pods closest to their memory limit.
 * @param {{ workload: string, pod: string, usage: number, trendPerHour: number }} p
 * @returns {{ pod: string, namespace: string, share: string, trend: string, rising: boolean, reachesIn: string }}
 */
export function closestRow(p) {
  const [namespace, pod] = p.pod.includes('/') ? p.pod.split('/') : ['', p.pod]
  const trend = p.trendPerHour || 0
  const rising = trend >= FLAT_TREND_PER_HOUR
  let trendText = 'flat'
  if (rising) trendText = `+${Math.round(trend * 100)}%/h`
  else if (trend <= -FLAT_TREND_PER_HOUR) trendText = `${Math.round(trend * 100)}%/h`
  const left = 1 - p.usage
  const reachesIn = rising && left > 0 ? `limit in ${formatEta(Math.round((left / trend) * 3600))} at this pace` : ''
  return { pod, namespace, share: `${Math.round(p.usage * 100)}%`, trend: trendText, rising, reachesIn }
}
