import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/svelte'
import WorkloadCard from '../WorkloadCard.svelte'
import type { FleetHost } from '../../lib/api'

function host(overrides: Partial<FleetHost> = {}): FleetHost {
  return {
    host: 'production/Deployment/payment-api',
    state: 'healthy',
    health_score: 82,
    stress: 20,
    fatigue: 15,
    contagion: 5,
    active_alerts: 0,
    last_seen: new Date().toISOString(),
    fused_rupture_index: 0.4,
    ...overrides,
  }
}

describe('WorkloadCard', () => {
  it('shows PENDING badge for pending_telemetry state', () => {
    const { getByText } = render(WorkloadCard, {
      props: { host: host({ state: 'pending_telemetry' }) },
    })
    expect(getByText('PENDING')).toBeInTheDocument()
    expect(getByText('Waiting for first OTLP telemetry…')).toBeInTheDocument()
  })

  it('shows health score for active workload', () => {
    const { getByText } = render(WorkloadCard, {
      props: { host: host({ health_score: 74 }) },
    })
    expect(getByText('74')).toBeInTheDocument()
  })

  it('shows short display name from namespace/kind/name path', () => {
    const { getByText } = render(WorkloadCard, {
      props: { host: host({ host: 'production/Deployment/payment-api' }) },
    })
    expect(getByText('payment-api')).toBeInTheDocument()
    expect(getByText('production · Deployment')).toBeInTheDocument()
  })

  it('shows rupture warning when fused_r > 1.5 and health_score > 60', () => {
    const { getByText } = render(WorkloadCard, {
      props: {
        host: host({
          health_score: 72,
          fused_rupture_index: 2.1,
        }),
      },
    })
    expect(getByText(/Early rupture/i)).toBeInTheDocument()
    expect(getByText(/FusedR 2\.1/)).toBeInTheDocument()
  })

  it('does NOT show rupture warning when fused_r <= 1.5', () => {
    const { queryByText } = render(WorkloadCard, {
      props: { host: host({ health_score: 72, fused_rupture_index: 1.5 }) },
    })
    expect(queryByText(/Early rupture signal/i)).not.toBeInTheDocument()
  })

  it('does NOT show rupture warning when health_score <= 60', () => {
    const { queryByText } = render(WorkloadCard, {
      props: { host: host({ health_score: 55, fused_rupture_index: 3.0 }) },
    })
    expect(queryByText(/Early rupture signal/i)).not.toBeInTheDocument()
  })

  it('shows low-confidence label when confidence_window < 60', () => {
    const { getByText } = render(WorkloadCard, {
      props: {
        host: host({
          health_score: 75,
          fused_rupture_index: 2.0,
          health_forecast: {
            trend: 'degrading',
            in_15min: 65,
            in_30min: 55,
            critical_eta_minutes: 45,
            confidence_window: 30,
          },
        }),
      },
    })
    expect(getByText(/low conf\./i)).toBeInTheDocument()
  })

  it('does NOT show low-confidence when confidence_window >= 60', () => {
    const { queryByText } = render(WorkloadCard, {
      props: {
        host: host({
          health_score: 75,
          fused_rupture_index: 2.0,
          health_forecast: {
            trend: 'degrading',
            in_15min: 65,
            in_30min: 55,
            critical_eta_minutes: 45,
            confidence_window: 80,
          },
        }),
      },
    })
    expect(queryByText(/low conf\./i)).not.toBeInTheDocument()
  })

  it('shows ETA forecast body when critical_eta_minutes > 0', () => {
    const { getByText } = render(WorkloadCard, {
      props: {
        host: host({
          health_score: 75,
          fused_rupture_index: 2.0,
          health_forecast: {
            trend: 'degrading',
            in_15min: 65,
            in_30min: 55,
            critical_eta_minutes: 38,
            confidence_window: 80,
          },
        }),
      },
    })
    expect(getByText(/in 15m/i)).toBeInTheDocument()
  })

  it('does not show score or bars for pending workload', () => {
    const { queryByText } = render(WorkloadCard, {
      props: { host: host({ state: 'pending_telemetry', health_score: 0 }) },
    })
    expect(queryByText('Stress')).not.toBeInTheDocument()
  })

  it('applies selected styling via class prop', () => {
    const { container } = render(WorkloadCard, {
      props: { host: host(), selected: true },
    })
    expect(container.querySelector('.selected')).toBeInTheDocument()
  })
})
