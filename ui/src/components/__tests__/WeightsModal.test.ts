import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, fireEvent } from '@testing-library/svelte'
import WeightsModal from '../WeightsModal.svelte'
import { fetchWeights, saveWeights } from '../../lib/api'

vi.mock('../../lib/api', () => ({
  fetchWeights: vi.fn().mockResolvedValue([]),
  saveWeights: vi.fn().mockResolvedValue({ applied: 1 }),
}))

beforeEach(() => vi.clearAllMocks())

describe('WeightsModal', () => {
  it('renders the modal title', () => {
    const { getByRole } = render(WeightsModal)
    expect(getByRole('dialog', { name: /signal weight/i })).toBeInTheDocument()
  })

  it('renders all six signal column headers as 3-char abbreviations', () => {
    const { getAllByRole } = render(WeightsModal)
    const headers = getAllByRole('columnheader').map(h => h.textContent?.trim())
    for (const abbr of ['str', 'fat', 'moo', 'pre', 'hum', 'con']) {
      expect(headers).toContain(abbr)
    }
  })

  it('shows a default row when API returns empty', async () => {
    const { findByLabelText } = render(WeightsModal)
    // onMount → load() → fetchWeights() → rows = [newRow()] → selector input appears
    const selector = await findByLabelText('Selector for row 1')
    expect(selector).toBeInTheDocument()
  })

  it('shows existing weight rows from the API', async () => {
    vi.mocked(fetchWeights).mockResolvedValueOnce([
      { selector: 'payments/*', stress: 0.35, fatigue: 0.15, mood: 0.20, pressure: 0.20, humidity: 0.05, contagion: 0.05 },
    ])
    const { findByDisplayValue } = render(WeightsModal)
    expect(await findByDisplayValue('payments/*')).toBeInTheDocument()
  })

  it('has a Save button', () => {
    const { getByRole } = render(WeightsModal)
    expect(getByRole('button', { name: /Save/i })).toBeInTheDocument()
  })

  it('has an Add rule button', () => {
    const { getByText } = render(WeightsModal)
    expect(getByText('+ Add rule')).toBeInTheDocument()
  })

  it('adds a new row when Add rule button is clicked', async () => {
    const { getByText, findAllByLabelText } = render(WeightsModal)
    // Wait for default row to appear
    await findAllByLabelText(/Selector for row/)
    await fireEvent.click(getByText('+ Add rule'))
    const selectors = await findAllByLabelText(/Selector for row/)
    expect(selectors.length).toBeGreaterThanOrEqual(2)
  })

  it('has a close button', () => {
    const { getByRole } = render(WeightsModal)
    expect(getByRole('button', { name: /Close/i })).toBeInTheDocument()
  })
})
