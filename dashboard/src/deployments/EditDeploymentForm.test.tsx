import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import EditDeploymentForm from './EditDeploymentForm'
import * as api from '../lib/api'

vi.mock('../lib/api', () => ({
  updateDeployment: vi.fn(),
}))

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

const deployment: api.Deployment = {
  id: 'dep-1',
  name: 'my-app',
  image: 'nginx:latest',
  envs: {},
  ports: ['32768:80'],
  proxy_port: 80,
  volumes: [],
  domain: 'app.example.com',
  public: false,
  status: 'healthy',
}

describe('EditDeploymentForm', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('submits public=true when public access is enabled', async () => {
    const onClose = vi.fn()
    const mockUpdate = vi.mocked(api.updateDeployment).mockResolvedValue({
      ...deployment,
      public: true,
    })
    const user = userEvent.setup()

    renderWithQuery(<EditDeploymentForm deployment={deployment} onClose={onClose} />)

    await user.click(screen.getByRole('switch', { name: /public deployment/i }))
    await user.click(screen.getByRole('button', { name: /^save$/i }))

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith('dep-1', {
        name: 'my-app',
        image: 'nginx:latest',
        envs: {},
        ports: ['80'],
        proxy_port: 80,
        volume_mounts: [],
        domain: 'app.example.com',
        public: true,
        basic_auth: undefined,
        security: undefined,
      })
    })
  })
})
