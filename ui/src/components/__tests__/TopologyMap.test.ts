import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render } from '@testing-library/svelte'
import TopologyMap from '../TopologyMap.svelte'
import { fetchTopology } from '../../lib/api'
import type { TopologyNode, TopologyEdge } from '../../lib/api'

function makeNode(overrides: Partial<TopologyNode> & { id: string }): TopologyNode {
  return {
    label: overrides.id.split('/').at(-1) ?? overrides.id,
    namespace: 'prod',
    kind: 'Deployment',
    health_score: 80,
    fused_r: 0.2,
    state: 'healthy',
    stress: 0.1,
    fatigue: 0.1,
    contagion: 0.05,
    mood: 0.8,
    velocity: 0.5,
    entropy: 0.1,
    ...overrides,
  }
}

function makeEdge(overrides: Partial<TopologyEdge> & { source: string; target: string }): TopologyEdge {
  return {
    call_rate: 100,
    error_rate: 0.02,
    p99_latency_ms: 12,
    edge_type: 'trace',
    strength: 1.0,
    ...overrides,
  }
}

const EMPTY_GRAPH = { nodes: [], edges: [] }

const GRAPH_WITH_DATA = {
  nodes: [
    makeNode({ id: 'prod/Deployment/api', health_score: 82, fused_r: 0.3, state: 'healthy' }),
    makeNode({ id: 'prod/Deployment/db',  health_score: 31, fused_r: 2.8, state: 'critical' }),
  ],
  edges: [
    makeEdge({ source: 'prod/Deployment/api', target: 'prod/Deployment/db', call_rate: 450, error_rate: 0.04, p99_latency_ms: 22 }),
  ],
}

vi.mock('../../lib/api', () => ({
  fetchTopology: vi.fn(),
}))


beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(fetchTopology).mockResolvedValue(EMPTY_GRAPH)
})

describe('TopologyMap', () => {
  it('shows empty edge indicator when no edges returned', async () => {
    const { findByText } = render(TopologyMap)
    expect(await findByText(/No edges/i)).toBeInTheDocument()
  })

  it('shows workload count when data is present', async () => {
    vi.mocked(fetchTopology).mockResolvedValueOnce(GRAPH_WITH_DATA)
    const { findByText } = render(TopologyMap)
    expect(await findByText(/2 workloads/i)).toBeInTheDocument()
  })

  it('renders refresh button', async () => {
    const { findByTitle } = render(TopologyMap)
    expect(await findByTitle('Refresh')).toBeInTheDocument()
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

  it('shows trace-confirmed edge indicator when trace edges present', async () => {
    vi.mocked(fetchTopology).mockResolvedValueOnce(GRAPH_WITH_DATA)
    const { findByText } = render(TopologyMap)
    expect(await findByText(/Trace-confirmed edges/i)).toBeInTheDocument()
  })

  it('renders legend items', async () => {
    const { findByText } = render(TopologyMap)
    await findByText(/No edges/i)
    expect(document.body).toHaveTextContent('Healthy')
    expect(document.body).toHaveTextContent('Critical')
  })
})
