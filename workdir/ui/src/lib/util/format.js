// Shared formatting utilities — reused by all widget types

export const PCT_METRICS   = new Set([
  'cpu_percent','memory_percent','disk_percent',
  'container_cpu_percent','container_mem_percent',
])
export const RATE_METRICS  = new Set(['error_rate','timeout_rate'])
export const BPS_METRICS   = new Set(['net_rx_bps','net_tx_bps'])
export const BYTES_METRICS = new Set([
  'container_net_rx_bytes','container_net_tx_bytes',
  'disk_read_bytes','disk_write_bytes',
])
export const BPS_DISK      = new Set(['disk_read_bps','disk_write_bps'])
export const MB_METRICS    = new Set(['container_mem_used_mb'])
export const LOAD_METRICS  = new Set(['load_avg_1','load_avg_5','load_avg_15','load1','load5','load15'])
export const KPI_NAMES     = new Set([
  'stress','fatigue','mood','pressure','humidity',
  'contagion','resilience','entropy','velocity','health_score',
])

export function fmtUptime(s) {
  if (s < 0) s = 0
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = Math.floor(s % 60)
  if (d > 0)  return `${d}d ${h}h`
  if (h > 0)  return `${h}h ${m}m`
  return `${m}m ${sec}s`
}

export function fmtBytesPerSec(Bps) {
  if (Bps >= 1e9)  return { value: (Bps / 1e9).toFixed(2), unit: 'GB/s' }
  if (Bps >= 1e6)  return { value: (Bps / 1e6).toFixed(1), unit: 'MB/s' }
  if (Bps >= 1e3)  return { value: (Bps / 1e3).toFixed(1), unit: 'KB/s' }
  return { value: Bps.toFixed(0), unit: 'B/s' }
}

export function fmtBytes(b) {
  if (b >= 1e9)  return { value: (b / 1e9).toFixed(2), unit: 'GB' }
  if (b >= 1e6)  return { value: (b / 1e6).toFixed(1), unit: 'MB' }
  if (b >= 1e3)  return { value: (b / 1e3).toFixed(1), unit: 'KB' }
  return { value: b.toFixed(0), unit: 'B' }
}

// Generic large-number formatter: 1 234 567 → "1.2M"
function fmtNum(v) {
  const a = Math.abs(v)
  if (a >= 1e9)  return { value: (v / 1e9).toFixed(2), unit: 'B' }
  if (a >= 1e6)  return { value: (v / 1e6).toFixed(1), unit: 'M' }
  if (a >= 1e3)  return { value: (v / 1e3).toFixed(1), unit: 'K' }
  if (a < 0.001 && a > 0) return { value: (v * 1e6).toFixed(1), unit: 'µ' }
  if (a < 1)     return { value: v.toFixed(3), unit: '' }
  return { value: v.toFixed(a < 10 ? 2 : a < 100 ? 1 : 0), unit: '' }
}

/**
 * Returns {value: string, unit: string} for any metric name + raw stored value.
 *
 * Stored scales:
 *   PCT_METRICS      → 0–100  (raw percent, e.g. cpu_percent = 65.3)
 *   RATE_METRICS     → 0–1    (fraction, e.g. error_rate = 0.02)
 *   BPS_METRICS      → B/s    (raw bytes/sec)
 *   BPS_DISK         → B/s
 *   BYTES_METRICS    → bytes
 *   MB_METRICS       → megabytes
 *   LOAD_METRICS     → dimensionless (0–N)
 *   KPI_NAMES        → 0–1    except health_score which is 0–100
 *   processes        → integer count
 *   uptime_seconds   → seconds
 *   request_rate     → req/s
 */
export function humanise(metricName, raw) {
  if (raw == null || isNaN(raw)) return { value: '—', unit: '' }

  if (metricName === 'uptime_seconds')
    return { value: fmtUptime(raw), unit: '' }

  if (metricName === 'processes')
    return { value: String(Math.round(raw)), unit: '' }

  if (metricName === 'request_rate')
    return { value: raw.toFixed(1), unit: 'req/s' }

  // PCT_METRICS: raw is already 0–100
  if (PCT_METRICS.has(metricName))
    return { value: raw.toFixed(1), unit: '%' }

  // RATE_METRICS: raw is 0–1 fraction
  if (RATE_METRICS.has(metricName))
    return { value: (raw * 100).toFixed(2), unit: '%' }

  // Network/disk throughput
  if (BPS_METRICS.has(metricName) || BPS_DISK.has(metricName))
    return fmtBytesPerSec(raw)

  if (BYTES_METRICS.has(metricName))
    return fmtBytes(raw)

  if (MB_METRICS.has(metricName))
    return { value: raw.toFixed(0), unit: 'MB' }

  if (LOAD_METRICS.has(metricName))
    return { value: raw.toFixed(2), unit: '' }

  // KPI: health_score is 0–100; all others are 0–1
  if (metricName === 'health_score')
    return { value: raw.toFixed(1), unit: '/100' }
  if (KPI_NAMES.has(metricName))
    return { value: (raw * 100).toFixed(1), unit: '%' }

  // Unknown metric: auto-format number
  return fmtNum(raw)
}

/** Returns {value: string, unit: string} for explicit KPI 0–1 values (snapshot context) */
export function kpiHumanise(kpiName, raw) {
  if (raw == null || isNaN(raw)) return { value: '—', unit: kpiName === 'health_score' ? '/100' : '%' }
  if (kpiName === 'health_score') return { value: raw.toFixed(1), unit: '/100' }
  return { value: (raw * 100).toFixed(1), unit: '%' }
}

/** Format a timestamp (ms or Date) as HH:MM */
export function fmtTime(t) {
  const d = t instanceof Date ? t : new Date(t)
  return d.toTimeString().slice(0, 5)
}

/** Format a timestamp as relative "2m ago" style */
export function fmtRelative(t) {
  const d = t instanceof Date ? t : new Date(t)
  const sec = Math.floor((Date.now() - d.getTime()) / 1000)
  if (sec < 60)    return `${sec}s ago`
  if (sec < 3600)  return `${Math.floor(sec / 60)}m ago`
  if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`
  return `${Math.floor(sec / 86400)}d ago`
}

export const SEVERITY_COLOR = {
  info:      '#38bdf8',
  warning:   '#fbbf24',
  critical:  '#ef4444',
  emergency: '#a855f7',
}
