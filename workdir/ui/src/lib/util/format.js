// Shared formatting utilities — reused by all widget types

export const PCT_METRICS   = new Set(['cpu_percent','memory_percent','disk_percent',
                                       'container_cpu_percent','container_mem_percent'])
export const RATE_METRICS  = new Set(['error_rate','timeout_rate'])
export const BPS_METRICS   = new Set(['net_rx_bps','net_tx_bps'])
export const BYTES_METRICS = new Set(['container_net_rx_bytes','container_net_tx_bytes'])
export const MB_METRICS    = new Set(['container_mem_used_mb'])
export const KPI_NAMES     = new Set(['stress','fatigue','mood','pressure','humidity',
                                       'contagion','resilience','entropy','velocity','health_score'])

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

export function fmtBits(bps) {
  if (bps >= 1e9)  return { value: (bps/1e9).toFixed(2), unit: 'Gbps' }
  if (bps >= 1e6)  return { value: (bps/1e6).toFixed(1), unit: 'Mbps' }
  if (bps >= 1e3)  return { value: (bps/1e3).toFixed(1), unit: 'Kbps' }
  return { value: bps.toFixed(0), unit: 'bps' }
}

export function fmtBytes(b) {
  if (b >= 1e9)  return { value: (b/1e9).toFixed(2), unit: 'GB' }
  if (b >= 1e6)  return { value: (b/1e6).toFixed(1), unit: 'MB' }
  if (b >= 1e3)  return { value: (b/1e3).toFixed(1), unit: 'KB' }
  return { value: b.toFixed(0), unit: 'B' }
}

/** Returns {value: string, unit: string} for any metric name + raw number */
export function humanise(metricName, raw) {
  if (raw == null || isNaN(raw)) return { value: '—', unit: '' }
  if (metricName === 'uptime_seconds')           return { value: fmtUptime(raw), unit: '' }
  if (BPS_METRICS.has(metricName))               return fmtBits(raw)
  if (BYTES_METRICS.has(metricName))             return fmtBytes(raw)
  if (MB_METRICS.has(metricName))                return { value: raw.toFixed(0), unit: 'MB' }
  if (PCT_METRICS.has(metricName) || RATE_METRICS.has(metricName))
                                                 return { value: (raw * 100).toFixed(1), unit: '%' }
  if (metricName === 'request_rate')             return { value: raw.toFixed(1), unit: 'req/s' }
  if (KPI_NAMES.has(metricName))                 return { value: (raw * 100).toFixed(1), unit: '%' }
  if (metricName === 'load1' || metricName === 'load5' || metricName === 'load15')
                                                 return { value: raw.toFixed(2), unit: '' }
  return { value: raw.toFixed(2), unit: '' }
}

/** Returns {value: string, unit: string} for explicit KPI 0-1 values */
export function kpiHumanise(kpiName, raw) {
  if (raw == null || isNaN(raw)) return { value: '—', unit: '%' }
  return { value: (raw * 100).toFixed(1), unit: '%' }
}

/** Format a timestamp (ms or Date) as HH:MM */
export function fmtTime(t) {
  const d = t instanceof Date ? t : new Date(t)
  return d.toTimeString().slice(0,5)
}

/** Format a timestamp as relative "2m ago" style */
export function fmtRelative(t) {
  const d = t instanceof Date ? t : new Date(t)
  const sec = Math.floor((Date.now() - d.getTime()) / 1000)
  if (sec < 60)   return `${sec}s ago`
  if (sec < 3600) return `${Math.floor(sec/60)}m ago`
  if (sec < 86400)return `${Math.floor(sec/3600)}h ago`
  return `${Math.floor(sec/86400)}d ago`
}

export const SEVERITY_COLOR = {
  info:      '#38bdf8',
  warning:   '#fbbf24',
  critical:  '#ef4444',
  emergency: '#a855f7',
}
