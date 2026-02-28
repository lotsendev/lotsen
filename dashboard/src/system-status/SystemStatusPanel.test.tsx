import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { SystemStatusPanel } from './SystemStatusPanel'
import * as api from '../lib/api'

vi.mock('../lib/api', () => ({
  getSystemStatus: vi.fn(),
  getLoadBalancerAccessLogs: vi.fn(),
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
    vi.mocked(api.getLoadBalancerAccessLogs).mockResolvedValue({
      items: [],
      hasMore: false,
    })
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
        checks: {
          processRunning: true,
          dashboardReachable: true,
          storeAccessible: true,
        },
      },
      orchestrator: {
        state: 'healthy',
        lastUpdated,
        checks: {
          processRunning: true,
          dockerReachable: true,
          storeAccessible: true,
        },
      },
      loadBalancer: {
        state: 'healthy',
        lastUpdated,
        checks: {
          processRunning: true,
          healthcheckResponding: true,
        },
      },
      docker: {
        state: 'healthy',
        lastUpdated,
        checks: {
          daemonHealthy: true,
        },
      },
      host: {
        cpu: {
          state: 'healthy',
          usagePercent: 31.2,
          lastUpdated,
        },
        ram: {
          state: 'healthy',
          usagePercent: 45.8,
          lastUpdated,
        },
      },
    })

    renderWithQuery(<SystemStatusPanel />)

    await waitFor(() => expect(screen.getAllByText('healthy')).toHaveLength(4))
    expect(screen.getByText('API signal')).toBeInTheDocument()
    expect(screen.getByText('Orchestrator')).toBeInTheDocument()
    expect(screen.getByText('Docker connectivity')).toBeInTheDocument()
    expect(screen.getByText('Load balancer')).toBeInTheDocument()
    expect(screen.getByText('Services')).toBeInTheDocument()
    expect(screen.getByText('Host metrics')).toBeInTheDocument()
    expect(screen.queryByText('Access logs')).not.toBeInTheDocument()
    expect(screen.getByText('CPU usage')).toBeInTheDocument()
    expect(screen.getByText('RAM usage')).toBeInTheDocument()
    expect(screen.getByText(/Last heartbeat:/)).toBeInTheDocument()
    expect(screen.getAllByText(/Last checked:/)).toHaveLength(2)
    expect(screen.getByText(/Freshness:/)).toBeInTheDocument()
    expect(screen.getByText('Docker is reachable from orchestrator')).toBeInTheDocument()
    expect(screen.getAllByText('healthy pressure')).toHaveLength(2)
    expect(screen.getByTestId('api-status-icon')).toBeInTheDocument()
    expect(screen.getByTestId('orchestrator-status-icon')).toBeInTheDocument()
    expect(screen.getByTestId('docker-status-icon')).toBeInTheDocument()
    expect(screen.getByTestId('load-balancer-status-icon')).toBeInTheDocument()
    expect(screen.getByText('Reading: 31.2%')).toBeInTheDocument()
    expect(screen.getByText('Reading: 45.8%')).toBeInTheDocument()
    expect(screen.getAllByText('Checks')).toHaveLength(4)
    expect(screen.getByTestId('api-check-0-pass')).toBeInTheDocument()
    expect(screen.getByTestId('docker-check-0-pass')).toBeInTheDocument()
    expect(screen.getByTestId('load-balancer-check-1-pass')).toBeInTheDocument()
    expect(screen.getAllByText(new RegExp(new Date(lastUpdated).toLocaleString()), { selector: 'p' })).toHaveLength(6)
  })

  it('renders degraded orchestrator and docker states', async () => {
    const apiUpdated = '2026-02-22T11:30:00.000Z'
    const orchestratorUpdated = '2026-02-22T11:20:00.000Z'
    vi.mocked(api.getSystemStatus).mockResolvedValue({
      api: {
        state: 'healthy',
        lastUpdated: apiUpdated,
        checks: {
          processRunning: true,
          dashboardReachable: true,
          storeAccessible: true,
        },
      },
      orchestrator: {
        state: 'degraded',
        lastUpdated: orchestratorUpdated,
        checks: {
          processRunning: true,
          dockerReachable: false,
          storeAccessible: false,
        },
      },
      loadBalancer: {
        state: 'degraded',
        lastUpdated: orchestratorUpdated,
        checks: {
          processRunning: true,
          healthcheckResponding: false,
        },
      },
      docker: {
        state: 'degraded',
        lastUpdated: orchestratorUpdated,
        checks: {
          daemonHealthy: false,
        },
      },
      host: {
        cpu: {
          state: 'healthy',
          usagePercent: 87.2,
          lastUpdated: orchestratorUpdated,
        },
        ram: {
          state: 'healthy',
          usagePercent: 82.1,
          lastUpdated: orchestratorUpdated,
        },
      },
    })

    renderWithQuery(<SystemStatusPanel />)

    await waitFor(() => expect(screen.getAllByText('degraded')).toHaveLength(3))
    expect(screen.getByText('Docker check failed at last probe')).toBeInTheDocument()
    expect(screen.getByText('Load balancer healthcheck failed at last probe')).toBeInTheDocument()
    expect(screen.getByTestId('orchestrator-check-1-fail')).toBeInTheDocument()
    expect(screen.getByTestId('orchestrator-check-2-fail')).toBeInTheDocument()
    expect(screen.getByTestId('docker-check-0-fail')).toBeInTheDocument()
    expect(screen.getAllByText('degraded pressure')).toHaveLength(2)
    expect(screen.getAllByText(new RegExp(new Date(orchestratorUpdated).toLocaleString()), { selector: 'p' })).toHaveLength(5)
  })

  it('renders unavailable load balancer telemetry explicitly', async () => {
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
      loadBalancer: {
        state: 'unavailable',
      },
      docker: {
        state: 'healthy',
        lastUpdated: apiUpdated,
      },
      host: {
        cpu: {
          state: 'healthy',
          usagePercent: 52,
          lastUpdated: apiUpdated,
        },
        ram: {
          state: 'healthy',
          usagePercent: 65,
          lastUpdated: apiUpdated,
        },
      },
    })

    renderWithQuery(<SystemStatusPanel />)

    await waitFor(() => expect(screen.getByText('Load balancer')).toBeInTheDocument())
    expect(screen.getByText('No load balancer telemetry yet')).toBeInTheDocument()
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
      loadBalancer: {
        state: 'healthy',
        lastUpdated: apiUpdated,
      },
      docker: {
        state: 'unavailable',
      },
      host: {
        cpu: {
          state: 'unavailable',
        },
        ram: {
          state: 'unavailable',
        },
      },
    })

    renderWithQuery(<SystemStatusPanel />)

    await waitFor(() => expect(screen.getByText('Docker connectivity')).toBeInTheDocument())
    expect(screen.getByText('No Docker connectivity telemetry yet')).toBeInTheDocument()
    expect(screen.getByTestId('docker-status-icon')).toBeInTheDocument()
    expect(screen.getAllByText(/Reading: Unavailable/)).toHaveLength(2)
    expect(screen.getAllByText('unavailable telemetry')).toHaveLength(2)
    expect(screen.getAllByText(/No signal yet/, { selector: 'p' })).toHaveLength(3)
  })

  it('renders load balancer blocked IP telemetry', async () => {
    const apiUpdated = '2026-02-22T12:00:00.000Z'
    const blockedUntil = '2026-02-22T12:15:00.000Z'
    vi.mocked(api.getSystemStatus).mockResolvedValue({
      api: {
        state: 'healthy',
        lastUpdated: apiUpdated,
      },
      orchestrator: {
        state: 'healthy',
        lastUpdated: apiUpdated,
      },
      loadBalancer: {
        state: 'healthy',
        lastUpdated: apiUpdated,
        checks: {
          processRunning: true,
          healthcheckResponding: true,
        },
        traffic: {
          totalRequests: 1200,
          suspiciousRequests: 34,
          blockedRequests: 7,
          wafBlockedRequests: 5,
          uaBlockedRequests: 2,
          activeBlockedIps: 2,
          blockedIps: [
            { ip: '203.0.113.7', blockedUntil },
            { ip: '198.51.100.11', blockedUntil },
          ],
        },
      },
      docker: {
        state: 'healthy',
        lastUpdated: apiUpdated,
      },
      host: {
        cpu: {
          state: 'healthy',
          usagePercent: 20,
          lastUpdated: apiUpdated,
        },
        ram: {
          state: 'healthy',
          usagePercent: 30,
          lastUpdated: apiUpdated,
        },
      },
    })

    renderWithQuery(<SystemStatusPanel />)

    await waitFor(() => expect(screen.getByText('Traffic and security')).toBeInTheDocument())
    expect(screen.getByText(/Total requests:/)).toBeInTheDocument()
    expect(screen.getByText(/Active blocked IPs:/)).toBeInTheDocument()
    expect(screen.getByText(/WAF blocked:/)).toBeInTheDocument()
    expect(screen.getByText(/UA blocked:/)).toBeInTheDocument()
    expect(screen.getByText('203.0.113.7')).toBeInTheDocument()
  })

  it('renders fetch failure UI when endpoint errors', async () => {
    vi.mocked(api.getSystemStatus).mockRejectedValue(new Error('boom'))

    renderWithQuery(<SystemStatusPanel />)

    await waitFor(() =>
      expect(screen.getByText('Unable to fetch system status right now.')).toBeInTheDocument()
    )
  })
})
