import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import CreateDeploymentForm from './CreateDeploymentForm'
import * as api from '../lib/api'

vi.mock('../lib/api', () => ({
  createDeployment: vi.fn(),
  getDeployments: vi.fn(),
}))

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={qc}>{ui}</QueryClientProvider>
  )
}

describe('CreateDeploymentForm', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('blocks empty submission and shows required field errors', async () => {
    const user = userEvent.setup()
    renderWithQuery(<CreateDeploymentForm />)

    await user.click(screen.getByRole('button', { name: /^create$/i }))

    expect(screen.getByText('Name is required')).toBeInTheDocument()
    expect(screen.getByText('Image is required')).toBeInTheDocument()
    expect(vi.mocked(api.createDeployment)).not.toHaveBeenCalled()
  })

  it('submits a valid form and calls createDeployment with correct payload', async () => {
    const mockCreate = vi.mocked(api.createDeployment).mockResolvedValue({
      id: '1',
      name: 'my-app',
      image: 'nginx:latest',
      status: 'idle',
      envs: {},
      ports: [],
      volumes: [],
      domain: '',
      public: false,
    })
    const user = userEvent.setup()
    renderWithQuery(<CreateDeploymentForm />)

    await user.type(screen.getByLabelText(/name \*/i), 'my-app')
    await user.type(screen.getByLabelText(/image \*/i), 'nginx:latest')
    await user.click(screen.getByRole('button', { name: /^create$/i }))

    await waitFor(() =>
      expect(mockCreate.mock.calls[0][0]).toEqual({
        name: 'my-app',
        image: 'nginx:latest',
        envs: {},
        ports: [],
        volumes: [],
        domain: '',
      })
    )
  })

  it('adds and removes an env var row', async () => {
    const user = userEvent.setup()
    renderWithQuery(<CreateDeploymentForm />)

    await user.click(screen.getByRole('button', { name: /add env var/i }))
    expect(screen.getByPlaceholderText('KEY')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /remove env var/i }))
    expect(screen.queryByPlaceholderText('KEY')).not.toBeInTheDocument()
  })

  it('blocks submission when an env var row has an empty key', async () => {
    const user = userEvent.setup()
    renderWithQuery(<CreateDeploymentForm />)

    await user.type(screen.getByLabelText(/name \*/i), 'my-app')
    await user.type(screen.getByLabelText(/image \*/i), 'nginx:latest')
    await user.click(screen.getByRole('button', { name: /add env var/i }))
    // Leave key blank
    await user.click(screen.getByRole('button', { name: /^create$/i }))

    expect(screen.getByText('Key is required')).toBeInTheDocument()
    expect(vi.mocked(api.createDeployment)).not.toHaveBeenCalled()
  })

  it('adds and removes a port mapping row', async () => {
    const user = userEvent.setup()
    renderWithQuery(<CreateDeploymentForm />)

    await user.click(screen.getByRole('button', { name: /add port/i }))
    expect(screen.getByPlaceholderText('Container port')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /remove port mapping/i }))
    expect(screen.queryByPlaceholderText('Container port')).not.toBeInTheDocument()
  })

  it('adds and removes a volume mount row', async () => {
    const user = userEvent.setup()
    renderWithQuery(<CreateDeploymentForm />)

    await user.click(screen.getByRole('button', { name: /add volume/i }))
    expect(screen.getByPlaceholderText('/host/path')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /remove volume mount/i }))
    expect(screen.queryByPlaceholderText('/host/path')).not.toBeInTheDocument()
  })

  it('includes env vars, ports, and volumes in the submitted payload', async () => {
    const mockCreate = vi.mocked(api.createDeployment).mockResolvedValue({
      id: '2',
      name: 'full-app',
      image: 'alpine:3',
      status: 'idle',
      envs: { DEBUG: 'true' },
      ports: ['8080:80'],
      volumes: ['/data:/app/data'],
      domain: 'example.com',
      public: false,
    })
    const user = userEvent.setup()
    renderWithQuery(<CreateDeploymentForm />)

    await user.type(screen.getByLabelText(/name \*/i), 'full-app')
    await user.type(screen.getByLabelText(/image \*/i), 'alpine:3')
    await user.type(screen.getByLabelText(/domain/i), 'example.com')

    await user.click(screen.getByRole('button', { name: /add env var/i }))
    await user.type(screen.getByPlaceholderText('KEY'), 'DEBUG')
    await user.type(screen.getByPlaceholderText('value'), 'true')

    await user.click(screen.getByRole('button', { name: /add port/i }))
    await user.type(screen.getByPlaceholderText('Container port'), '80')

    await user.click(screen.getByRole('button', { name: /add volume/i }))
    await user.type(screen.getByPlaceholderText('/host/path'), '/data')
    await user.type(screen.getByPlaceholderText('/container/path'), '/app/data')

    await user.click(screen.getByRole('button', { name: /^create$/i }))

    await waitFor(() =>
      expect(mockCreate.mock.calls[0][0]).toEqual({
        name: 'full-app',
        image: 'alpine:3',
        envs: { DEBUG: 'true' },
        ports: ['80'],
        volumes: ['/data:/app/data'],
        domain: 'example.com',
      })
    )
  })
})
