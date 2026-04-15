// SVG chart primitives — shared by all chart widgets

/** Build an SVG polyline points string from [{t, v}] data */
export function buildPath(points, width, height, padX = 10, padY = 8) {
  if (!points || points.length < 2) return ''
  const ts  = points.map(p => p.t)
  const vs  = points.map(p => p.v)
  const tMin = Math.min(...ts), tMax = Math.max(...ts)
  const vMin = Math.min(...vs), vMax = Math.max(...vs)
  const tRange = tMax - tMin || 1
  const vRange = vMax - vMin || 1
  const pts = points.map(p => {
    const x = padX + ((p.t - tMin) / tRange) * (width - 2 * padX)
    const y = height - padY - ((p.v - vMin) / vRange) * (height - 2 * padY)
    return `${x.toFixed(1)},${y.toFixed(1)}`
  })
  return pts.join(' ')
}

/** Build SVG polygon points string for area fill (closes back along bottom) */
export function buildArea(points, width, height, padX = 10, padY = 8) {
  if (!points || points.length < 2) return ''
  const ts  = points.map(p => p.t)
  const vs  = points.map(p => p.v)
  const tMin = Math.min(...ts), tMax = Math.max(...ts)
  const vMin = Math.min(...vs), vMax = Math.max(...vs)
  const tRange = tMax - tMin || 1
  const vRange = vMax - vMin || 1
  const bottom = height - padY

  const top = points.map(p => {
    const x = padX + ((p.t - tMin) / tRange) * (width - 2 * padX)
    const y = height - padY - ((p.v - vMin) / vRange) * (height - 2 * padY)
    return `${x.toFixed(1)},${y.toFixed(1)}`
  })
  const firstX = (padX + ((points[0].t - tMin) / tRange) * (width - 2 * padX)).toFixed(1)
  const lastX  = (padX + ((points[points.length-1].t - tMin) / tRange) * (width - 2 * padX)).toFixed(1)
  return [...top, `${lastX},${bottom}`, `${firstX},${bottom}`].join(' ')
}

/** Build 4 evenly-spaced X-axis tick {x, label} from a points array */
export function buildTimeTicks(points, width, padX = 10) {
  if (!points || points.length < 2) return []
  const tMin = points[0].t
  const tMax = points[points.length - 1].t
  const tRange = tMax - tMin || 1
  return [0, 0.33, 0.66, 1].map(frac => {
    const t = tMin + frac * tRange
    const x = padX + frac * (width - 2 * padX)
    const d = new Date(t)
    const label = d.toTimeString().slice(0, 5)
    return { x, label }
  })
}

/** Build Y-axis {min, mid, max} labels */
export function buildYLabels(points) {
  if (!points || points.length === 0) return { min: '', mid: '', max: '' }
  const vs  = points.map(p => p.v)
  const min = Math.min(...vs)
  const max = Math.max(...vs)
  const fmt = v => Math.abs(v) >= 1000 ? (v/1000).toFixed(1)+'k'
                 : Math.abs(v) < 1    ? v.toFixed(3)
                 : v.toFixed(1)
  return { min: fmt(min), mid: fmt((min+max)/2), max: fmt(max) }
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
const GAP   = 0.25  // fraction of circle cut from the bottom
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
