import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, fireEvent } from '@testing-library/svelte'
import SuppressionModal from '../SuppressionModal.svelte'
import { fetchSuppressions, createSuppression, deleteSuppression } from '../../lib/api'

vi.mock('../../lib/api', () => ({
  fetchSuppressions: vi.fn().mockResolvedValue([]),
  createSuppression: vi.fn().mockResolvedValue({
    id: 'new-1', workload: 'ns/Deployment/svc', start: '', end: '', reason: '',
  }),
  deleteSuppression: vi.fn().mockResolvedValue(undefined),
}))

beforeEach(() => vi.clearAllMocks())

describe('SuppressionModal', () => {
  it('renders the modal title', () => {
    const { getByText } = render(SuppressionModal, { props: { defaultWorkload: '' } })
    expect(getByText(/Maintenance Windows/i)).toBeInTheDocument()
  })

  it('pre-fills workload input from defaultWorkload prop', () => {
    const { container } = render(SuppressionModal, {
      props: { defaultWorkload: 'production/Deployment/checkout' },
    })
    const input = container.querySelector('input[type="text"]') as HTMLInputElement
    expect(input?.value).toBe('production/Deployment/checkout')
  })

  it('shows empty state message when no windows', async () => {
    const { findByText } = render(SuppressionModal, { props: { defaultWorkload: '' } })
    expect(await findByText(/No suppression windows/i)).toBeInTheDocument()
  })

  it('shows validation error when workload is empty on submit', async () => {
    const { getByRole, findByText } = render(SuppressionModal, {
      props: { defaultWorkload: '' },
    })
    await fireEvent.click(getByRole('button', { name: /Create window/i }))
    expect(await findByText(/Workload is required/i)).toBeInTheDocument()
  })

  it('has a close button', () => {
    const { getByRole } = render(SuppressionModal, { props: { defaultWorkload: '' } })
    expect(getByRole('button', { name: /Close/i })).toBeInTheDocument()
  })

  it('renders both sections: active windows and add form', () => {
    const { getByText } = render(SuppressionModal, { props: { defaultWorkload: '' } })
    expect(getByText(/Active & Scheduled/i)).toBeInTheDocument()
    expect(getByText(/Add Window/i)).toBeInTheDocument()
  })

  it('shows existing windows returned by the API', async () => {
    vi.mocked(fetchSuppressions).mockResolvedValueOnce([
      {
        id: 'sup-1',
        workload: 'prod/Deployment/api',
        start: new Date(Date.now() - 3_600_000).toISOString(),
        end: new Date(Date.now() + 3_600_000).toISOString(),
        reason: 'deploy window',
      },
    ])
    const { findByText } = render(SuppressionModal, { props: { defaultWorkload: '' } })
    expect(await findByText('prod/Deployment/api')).toBeInTheDocument()
  })
})
