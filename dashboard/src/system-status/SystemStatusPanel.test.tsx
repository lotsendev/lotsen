import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { SystemStatusPanel } from './SystemStatusPanel'
import * as api from '../lib/api'

vi.mock('../lib/api', () => ({
  getSystemStatus: vi.fn(),
}))

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

describe('SystemStatusPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders loading state while request is pending', () => {
    vi.mocked(api.getSystemStatus).mockImplementation(() => new Promise(() => {}))

    renderWithQuery(<SystemStatusPanel />)

    expect(screen.getByText('Loading system status…')).toBeInTheDocument()
  })

  it('renders API signal and timestamp for healthy status', async () => {
    const lastUpdated = '2026-02-22T11:30:00.000Z'
    vi.mocked(api.getSystemStatus).mockResolvedValue({
      api: {
        state: 'healthy',
        lastUpdated,
      },
    })

    renderWithQuery(<SystemStatusPanel />)

    await waitFor(() => expect(screen.getByText('healthy')).toBeInTheDocument())
    expect(screen.getByText('API signal:')).toBeInTheDocument()
    expect(screen.getByText('Last updated:')).toBeInTheDocument()
    expect(screen.getByText(new Date(lastUpdated).toLocaleString())).toBeInTheDocument()
  })

  it('renders fetch failure UI when endpoint errors', async () => {
    vi.mocked(api.getSystemStatus).mockRejectedValue(new Error('boom'))

    renderWithQuery(<SystemStatusPanel />)

    await waitFor(() =>
      expect(screen.getByText('Unable to fetch system status right now.')).toBeInTheDocument()
    )
  })
})
