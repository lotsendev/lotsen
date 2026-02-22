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

  it('renders healthy API and orchestrator states', async () => {
    const lastUpdated = '2026-02-22T11:30:00.000Z'
    vi.mocked(api.getSystemStatus).mockResolvedValue({
      api: {
        state: 'healthy',
        lastUpdated,
      },
      orchestrator: {
        state: 'healthy',
        lastUpdated,
      },
      docker: {
        state: 'healthy',
        lastUpdated,
      },
    })

    renderWithQuery(<SystemStatusPanel />)

    await waitFor(() => expect(screen.getAllByText('healthy')).toHaveLength(3))
    expect(screen.getByText('API signal')).toBeInTheDocument()
    expect(screen.getByText('Orchestrator liveness')).toBeInTheDocument()
    expect(screen.getByText('Docker connectivity')).toBeInTheDocument()
    expect(screen.getAllByText('State:')).toHaveLength(3)
    expect(screen.getByText('Last updated:')).toBeInTheDocument()
    expect(screen.getByText('Last heartbeat:')).toBeInTheDocument()
    expect(screen.getByText('Last checked:')).toBeInTheDocument()
    expect(screen.getByText('Freshness:')).toBeInTheDocument()
    expect(screen.getByText('Signal:')).toBeInTheDocument()
    expect(screen.getByText('Docker is reachable from orchestrator')).toBeInTheDocument()
    expect(screen.getAllByText(new Date(lastUpdated).toLocaleString())).toHaveLength(3)
  })

  it('renders degraded orchestrator and docker states', async () => {
    const apiUpdated = '2026-02-22T11:30:00.000Z'
    const orchestratorUpdated = '2026-02-22T11:20:00.000Z'
    vi.mocked(api.getSystemStatus).mockResolvedValue({
      api: {
        state: 'healthy',
        lastUpdated: apiUpdated,
      },
      orchestrator: {
        state: 'degraded',
        lastUpdated: orchestratorUpdated,
      },
      docker: {
        state: 'degraded',
        lastUpdated: orchestratorUpdated,
      },
    })

    renderWithQuery(<SystemStatusPanel />)

    await waitFor(() => expect(screen.getAllByText('degraded')).toHaveLength(2))
    expect(screen.getByText('Docker check failed at last probe')).toBeInTheDocument()
    expect(screen.getAllByText(new Date(orchestratorUpdated).toLocaleString())).toHaveLength(2)
  })

  it('renders stale orchestrator state and freshness', async () => {
    const apiUpdated = '2026-02-22T12:00:00.000Z'
    const orchestratorUpdated = '2026-02-22T11:40:00.000Z'
    vi.mocked(api.getSystemStatus).mockResolvedValue({
      api: {
        state: 'healthy',
        lastUpdated: apiUpdated,
      },
      orchestrator: {
        state: 'stale',
        lastUpdated: orchestratorUpdated,
      },
      docker: {
        state: 'healthy',
        lastUpdated: apiUpdated,
      },
    })

    renderWithQuery(<SystemStatusPanel />)

    await waitFor(() => expect(screen.getByText('stale')).toBeInTheDocument())
    expect(screen.getByText(/ago|just now/)).toBeInTheDocument()

  })

  it('renders unavailable docker telemetry explicitly', async () => {
    const apiUpdated = '2026-02-22T12:00:00.000Z'
    vi.mocked(api.getSystemStatus).mockResolvedValue({
      api: {
        state: 'healthy',
        lastUpdated: apiUpdated,
      },
      orchestrator: {
        state: 'healthy',
        lastUpdated: apiUpdated,
      },
      docker: {
        state: 'unavailable',
      },
    })

    renderWithQuery(<SystemStatusPanel />)

    await waitFor(() => expect(screen.getByText('unavailable')).toBeInTheDocument())
    expect(screen.getByText('No Docker connectivity telemetry yet')).toBeInTheDocument()
    expect(screen.getByText('No signal yet')).toBeInTheDocument()
  })

  it('renders fetch failure UI when endpoint errors', async () => {
    vi.mocked(api.getSystemStatus).mockRejectedValue(new Error('boom'))

    renderWithQuery(<SystemStatusPanel />)

    await waitFor(() =>
      expect(screen.getByText('Unable to fetch system status right now.')).toBeInTheDocument()
    )
  })
})
