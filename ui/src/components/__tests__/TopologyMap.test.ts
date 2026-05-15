import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render } from '@testing-library/svelte'
import TopologyMap from '../TopologyMap.svelte'
import { fetchTopology } from '../../lib/api'

const EMPTY_GRAPH = { nodes: [], edges: [] }

const GRAPH_WITH_DATA = {
  nodes: [
    { id: 'prod/Deployment/api', health_score: 82, fused_r: 0.3, state: 'healthy' as const },
    { id: 'prod/Deployment/db',  health_score: 31, fused_r: 2.8, state: 'degraded' as const },
  ],
  edges: [
    { source: 'prod/Deployment/api', target: 'prod/Deployment/db',
      call_rate: 450, error_rate: 0.04, p99_latency_ms: 22 },
  ],
}

vi.mock('../../lib/api', () => ({
  fetchTopology: vi.fn(),
}))

// cytoscape uses canvas APIs not present in jsdom — stub it
vi.mock('cytoscape', () => ({
  default: vi.fn(() => ({
    on: vi.fn(),
    destroy: vi.fn(),
  })),
}))

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(fetchTopology).mockResolvedValue(EMPTY_GRAPH)
})

describe('TopologyMap', () => {
  it('shows empty-state message when no nodes returned', async () => {
    const { findByText } = render(TopologyMap)
    expect(await findByText(/No service connections discovered yet/i)).toBeInTheDocument()
  })

  it('shows node and edge counts when data is present', async () => {
    vi.mocked(fetchTopology).mockResolvedValueOnce(GRAPH_WITH_DATA)
    const { findByText } = render(TopologyMap)
    expect(await findByText(/2 nodes/i)).toBeInTheDocument()
    expect(await findByText(/1 edge/i)).toBeInTheDocument()
  })

  it('renders refresh button', async () => {
    const { findByTitle } = render(TopologyMap)
    expect(await findByTitle('Refresh now')).toBeInTheDocument()
  })

  it('calls fetchTopology on mount', async () => {
    render(TopologyMap)
    await vi.waitFor(() => {
      expect(fetchTopology).toHaveBeenCalledTimes(1)
    })
  })

  it('shows error message when API fails', async () => {
    vi.mocked(fetchTopology).mockRejectedValueOnce(new Error('network error'))
    const { findByText } = render(TopologyMap)
    expect(await findByText(/network error/i)).toBeInTheDocument()
  })

  it('shows retry button after error', async () => {
    vi.mocked(fetchTopology).mockRejectedValueOnce(new Error('fail'))
    const { findByRole } = render(TopologyMap)
    expect(await findByRole('button', { name: /retry/i })).toBeInTheDocument()
  })

  it('renders legend items', async () => {
    const { findByText } = render(TopologyMap)
    await findByText(/No service connections discovered yet/i) // wait for load to complete
    expect(document.body).toHaveTextContent('healthy')
    expect(document.body).toHaveTextContent('degraded')
    expect(document.body).toHaveTextContent('critical')
    expect(document.body).toHaveTextContent('no telemetry')
  })
})
