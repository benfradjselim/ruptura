import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render } from '@testing-library/svelte'
import Engine from '../../routes/Engine.svelte'
import { fetchEngineStatus, fetchEngineStorage } from '../../lib/api'

const STATUS_OK = {
  analyzer: { tick_interval_ms: 15000, last_tick_ago_ms: 1200, active_workloads: 10, calibrating_workloads: 3, pending_workloads: 1 },
  ingest:   { metrics_per_sec: 120.5, logs_per_sec: 8.3, traces_per_sec: 42.0 },
  actions:  { pending_tier1: 0, pending_tier2: 2, executed_last_hour: 5 },
  version: '7.0.0',
  edition: 'community',
  uptime_seconds: 3720,
}

const STORAGE_OK = {
  badger: { disk_bytes: 245_760_000, vlog_size_bytes: 18_000_000, num_tables: 6, keys: 18420 },
}

vi.mock('../../lib/api', () => ({
  fetchEngineStatus: vi.fn(),
  fetchEngineStorage: vi.fn(),
}))

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(fetchEngineStatus).mockResolvedValue(STATUS_OK)
  vi.mocked(fetchEngineStorage).mockResolvedValue(STORAGE_OK)
})

describe('Engine', () => {
  it('shows version and edition after load', async () => {
    const { findByText } = render(Engine)
    expect(await findByText('7.0.0')).toBeInTheDocument()
    expect(await findByText('community')).toBeInTheDocument()
  })

  it('shows active workload count', async () => {
    const { findByText } = render(Engine)
    expect(await findByText('10')).toBeInTheDocument()
  })

  it('shows ingest rate bars section', async () => {
    const { findByText } = render(Engine)
    expect(await findByText('Metrics')).toBeInTheDocument()
    expect(await findByText('Logs')).toBeInTheDocument()
    expect(await findByText('Traces')).toBeInTheDocument()
  })

  it('shows formatted ingest values', async () => {
    const { findByText } = render(Engine)
    expect(await findByText('120.5/s')).toBeInTheDocument()
  })

  it('shows storage section with formatted disk size', async () => {
    const { findByText } = render(Engine)
    expect(await findByText('234.4 MB')).toBeInTheDocument()
  })

  it('shows pending tier-2 action count', async () => {
    const { findAllByText } = render(Engine)
    const twos = await findAllByText('2')
    expect(twos.length).toBeGreaterThanOrEqual(1)
  })

  it('shows uptime formatted', async () => {
    const { findByText } = render(Engine)
    expect(await findByText('1h 2m')).toBeInTheDocument()
  })

  it('shows error banner when status fetch fails', async () => {
    vi.mocked(fetchEngineStatus).mockRejectedValueOnce(new Error('unreachable'))
    vi.mocked(fetchEngineStorage).mockRejectedValueOnce(new Error('unreachable'))
    const { findByText } = render(Engine)
    expect(await findByText(/unreachable/i)).toBeInTheDocument()
  })

  it('shows retry button on error', async () => {
    vi.mocked(fetchEngineStatus).mockRejectedValueOnce(new Error('fail'))
    vi.mocked(fetchEngineStorage).mockRejectedValueOnce(new Error('fail'))
    const { findByRole } = render(Engine)
    expect(await findByRole('button', { name: /retry/i })).toBeInTheDocument()
  })

  it('renders refresh button', async () => {
    const { findByTitle } = render(Engine)
    expect(await findByTitle('Refresh')).toBeInTheDocument()
  })

  it('shows footer with version', async () => {
    const { findByText } = render(Engine)
    await findByText('7.0.0')
    expect(document.body).toHaveTextContent('ruptura-ui v1.0.0')
  })
})
