// SVG chart primitives — shared by all chart widgets
import { humanise } from './format.js'

// ─── Coordinate helpers ───────────────────────────────────────────────────────

function scaleX(t, tMin, tRange, width, padX) {
  return padX + ((t - tMin) / tRange) * (width - 2 * padX)
}
function scaleY(v, vMin, vRange, height, padY) {
  return height - padY - ((v - vMin) / vRange) * (height - 2 * padY)
}

function extent(points) {
  const ts = points.map(p => p.t)
  const vs = points.map(p => p.v)
  const tMin = Math.min(...ts), tMax = Math.max(...ts)
  const vMin = Math.min(...vs), vMax = Math.max(...vs)
  return {
    tMin, tMax, tRange: tMax - tMin || 1,
    vMin, vMax, vRange: vMax - vMin || 1,
  }
}

/** Build an SVG polyline points string from [{t, v}] data */
export function buildPath(points, width, height, padX = 10, padY = 8) {
  if (!points || points.length < 2) return ''
  const { tMin, tRange, vMin, vRange } = extent(points)
  return points.map(p =>
    `${scaleX(p.t, tMin, tRange, width, padX).toFixed(1)},${scaleY(p.v, vMin, vRange, height, padY).toFixed(1)}`
  ).join(' ')
}

/** Build SVG polygon points string for area fill (closes back along bottom) */
export function buildArea(points, width, height, padX = 10, padY = 8) {
  if (!points || points.length < 2) return ''
  const { tMin, tRange, vMin, vRange } = extent(points)
  const bottom = height - padY
  const top = points.map(p =>
    `${scaleX(p.t, tMin, tRange, width, padX).toFixed(1)},${scaleY(p.v, vMin, vRange, height, padY).toFixed(1)}`
  )
  const firstX = scaleX(points[0].t, tMin, tRange, width, padX).toFixed(1)
  const lastX  = scaleX(points[points.length - 1].t, tMin, tRange, width, padX).toFixed(1)
  return [...top, `${lastX},${bottom}`, `${firstX},${bottom}`].join(' ')
}

/** Build X-axis tick positions + labels (4 ticks) */
export function buildTimeTicks(points, width, padX = 10) {
  if (!points || points.length < 2) return []
  const tMin = points[0].t
  const tMax = points[points.length - 1].t
  const tRange = tMax - tMin || 1
  return [0, 0.33, 0.66, 1].map(frac => ({
    x: padX + frac * (width - 2 * padX),
    label: new Date(tMin + frac * tRange).toTimeString().slice(0, 5),
    frac,
  }))
}

/**
 * Build X-axis ticks for a forecast chart.
 * Tick X positions are calculated in the full allPts coordinate space,
 * but labels span only the forecastPts time window (future range).
 */
export function buildForecastTicks(allPts, forecastPts, width, padX = 10) {
  if (!forecastPts || forecastPts.length < 2) return buildTimeTicks(allPts, width, padX)
  const tMin   = Math.min(...allPts.map(p => p.t))
  const tMax   = Math.max(...allPts.map(p => p.t))
  const tRange = tMax - tMin || 1
  const fMin   = forecastPts[0].t
  const fMax   = forecastPts[forecastPts.length - 1].t
  return [0, 0.33, 0.66, 1].map(frac => {
    const t = fMin + frac * (fMax - fMin)
    return {
      x: padX + ((t - tMin) / tRange) * (width - 2 * padX),
      label: new Date(t).toTimeString().slice(0, 5),
      frac,
    }
  })
}

/**
 * Build Y-axis labels {min, mid, max} formatted for the given metric.
 * If metricName is provided, uses humanise() for correct units.
 */
export function buildYLabels(points, metricName = '') {
  if (!points || points.length === 0) return { min: '', mid: '', max: '' }
  const vs  = points.map(p => p.v)
  const min = Math.min(...vs)
  const max = Math.max(...vs)
  const mid = (min + max) / 2

  if (metricName) {
    const fmt = v => {
      const h = humanise(metricName, v)
      return h.value + (h.unit ? h.unit : '')
    }
    return { min: fmt(min), mid: fmt(mid), max: fmt(max) }
  }

  // Fallback: generic numeric formatting
  const fmt = v => {
    const a = Math.abs(v)
    if (a >= 1e6)  return (v / 1e6).toFixed(1) + 'M'
    if (a >= 1e3)  return (v / 1e3).toFixed(1) + 'K'
    if (a < 0.001 && a > 0) return v.toFixed(4)
    if (a < 1)  return v.toFixed(3)
    return v.toFixed(a < 10 ? 2 : a < 100 ? 1 : 0)
  }
  return { min: fmt(min), mid: fmt(mid), max: fmt(max) }
}

/**
 * Build horizontal grid lines.
 * Returns array of {y, value} — pixel Y position + raw data value for labelling.
 * count=4 produces lines at 25%, 50%, 75% of the data range.
 */
export function buildHGridLines(points, height, padY = 8, count = 4) {
  if (!points || points.length < 2) return []
  const vs = points.map(p => p.v)
  const vMin = Math.min(...vs)
  const vMax = Math.max(...vs)
  const vRange = vMax - vMin || 1
  const lines = []
  for (let i = 1; i < count; i++) {
    const frac = i / count
    const v = vMin + frac * vRange
    const y = height - padY - frac * (height - 2 * padY)
    lines.push({ y: y.toFixed(1), v })
  }
  return lines
}

/**
 * Build vertical grid lines aligned to X-axis ticks.
 * Returns array of x pixel positions.
 */
export function buildVGridLines(points, width, padX = 10) {
  return buildTimeTicks(points, width, padX).map(t => t.x)
}

/** Compute the Y pixel for a given value in a dataset */
export function valToY(v, points, height, padY = 8) {
  const vs   = points.map(p => p.v)
  const vMin = Math.min(...vs)
  const vMax = Math.max(...vs)
  const vRange = vMax - vMin || 1
  return height - padY - ((v - vMin) / vRange) * (height - 2 * padY)
}

// ─── Gauge arc ────────────────────────────────────────────────────────────────
const GAP   = 0.25
const START = Math.PI * (0.5 + GAP / 2)
const END   = Math.PI * (0.5 - GAP / 2) + 2 * Math.PI

export function arcPath(pct, r, cx, cy) {
  const clamp = Math.max(0, Math.min(100, pct))
  const angle = START + (clamp / 100) * (END - START)
  const x1 = cx + r * Math.cos(START)
  const y1 = cy + r * Math.sin(START)
  const x2 = cx + r * Math.cos(angle)
  const y2 = cy + r * Math.sin(angle)
  const large = angle - START > Math.PI ? 1 : 0
  return `M ${x1.toFixed(2)} ${y1.toFixed(2)} A ${r} ${r} 0 ${large} 1 ${x2.toFixed(2)} ${y2.toFixed(2)}`
}

export function gaugeColor(pct) {
  if (pct >= 80) return '#ef4444'
  if (pct >= 60) return '#fbbf24'
  return '#38bdf8'
}
