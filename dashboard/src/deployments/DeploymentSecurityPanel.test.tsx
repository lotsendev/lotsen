import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { DeploymentSecurityPanel } from './DeploymentSecurityPanel'
import * as api from '../lib/api'

vi.mock('../lib/api', () => ({
  getSecurityConfig: vi.fn(),
  patchDeployment: vi.fn(),
}))

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })

  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

const deployment: api.Deployment = {
  id: 'dep-1',
  name: 'app',
  image: 'nginx:latest',
  envs: {},
  ports: [],
  volumes: [],
  domain: 'app.example.com',
  status: 'healthy',
}

describe('DeploymentSecurityPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.getSecurityConfig).mockResolvedValue({
      profile: 'standard',
      suspiciousWindowSeconds: 30,
      suspiciousThreshold: 10,
      suspiciousBlockForSeconds: 120,
      wafEnabled: true,
      wafMode: 'detection',
      globalIpDenylist: [],
      globalIpAllowlist: [],
    })
  })

  it('rejects invalid CIDR entries client-side', async () => {
    const user = userEvent.setup()
    renderWithQuery(<DeploymentSecurityPanel deployment={deployment} />)

    await user.type(screen.getByLabelText('IP denylist'), 'not-an-ip{enter}')

    expect(screen.getByText('IP filters must be valid CIDR ranges or IP addresses.')).toBeInTheDocument()
    expect(vi.mocked(api.patchDeployment)).not.toHaveBeenCalled()
  })

  it('submits security config via patch endpoint', async () => {
    const user = userEvent.setup()
    vi.mocked(api.patchDeployment).mockResolvedValue({
      ...deployment,
      security: {
        waf_enabled: false,
        ip_denylist: ['10.0.0.0/8'],
        ip_allowlist: ['203.0.113.0/24'],
        custom_rules: ['SecRule REQUEST_URI "@contains blocked" "id:10001,phase:1,deny,status:403"'],
      },
    })

    renderWithQuery(<DeploymentSecurityPanel deployment={deployment} />)

    await user.click(screen.getByRole('checkbox', { name: /enable waf for this deployment/i }))
    await user.type(screen.getByLabelText('IP denylist'), '10.0.0.0/8{enter}')
    await user.type(screen.getByLabelText('IP allowlist'), '203.0.113.0/24{enter}')
    await user.type(
      screen.getByLabelText('Custom rules'),
      'SecRule REQUEST_URI "@contains blocked" "id:10001,phase:1,deny,status:403"'
    )
    await user.click(screen.getByRole('button', { name: /save security/i }))

    await waitFor(() => {
      expect(vi.mocked(api.patchDeployment)).toHaveBeenCalledWith('dep-1', {
        security: {
          waf_enabled: false,
          ip_denylist: ['10.0.0.0/8'],
          ip_allowlist: ['203.0.113.0/24'],
          custom_rules: ['SecRule REQUEST_URI "@contains blocked" "id:10001,phase:1,deny,status:403"'],
        },
      })
    })
  })
})
