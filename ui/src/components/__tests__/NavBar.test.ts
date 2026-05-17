import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render } from '@testing-library/svelte'
import NavBar from '../NavBar.svelte'

// poll() calls both endpoints on mount — mock both to avoid real HTTP
vi.mock('../../lib/api', () => ({
  fetchHealth: vi.fn().mockResolvedValue({
    version: '7.0.0', status: 'ok', uptime_seconds: 100,
    storage: { status: 'ok' }, ingest: { metrics: 0, logs: 0, traces: 0 },
  }),
  fetchEngineStatus: vi.fn().mockResolvedValue({
    analyzer: { tick_interval_ms: 0, last_tick_ago_ms: 0, active_workloads: 3, calibrating_workloads: 1, pending_workloads: 0 },
    ingest: { metrics_per_sec: 42, logs_per_sec: 5, traces_per_sec: 10 },
    actions: { pending_tier1: 0, pending_tier2: 0, executed_last_hour: 0 },
    version: '7.0.0', edition: 'community', uptime_seconds: 1200,
  }),
}))

beforeEach(() => vi.clearAllMocks())

describe('NavBar', () => {
  it('renders all four nav links', () => {
    const { getByText } = render(NavBar, { props: { route: 'fleet' } })
    expect(getByText('Fleet')).toBeInTheDocument()
    expect(getByText('Topology')).toBeInTheDocument()
    expect(getByText('Engine')).toBeInTheDocument()
    expect(getByText('Nodes')).toBeInTheDocument()
  })

  it('marks the active route link', () => {
    const { container } = render(NavBar, { props: { route: 'engine' } })
    const active = container.querySelector('a.active')
    expect(active).toBeInTheDocument()
    expect(active?.textContent).toContain('Engine')
  })

  it('defaults to Fleet when route is empty string', () => {
    const { container } = render(NavBar, { props: { route: '' } })
    const active = container.querySelector('a.active')
    expect(active?.textContent).toContain('Fleet')
  })

  it('renders brand name', () => {
    const { getByText } = render(NavBar, { props: { route: 'fleet' } })
    expect(getByText('RUPTURA')).toBeInTheDocument()
  })
})
