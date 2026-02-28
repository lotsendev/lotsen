import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes, Navigate } from 'react-router-dom'
import Landing from '@/pages/Landing'
import DocsLayout from '@/pages/docs/DocsLayout'
import GettingStarted from '@/pages/docs/GettingStarted'
import DeploymentConfiguration from '@/pages/docs/DeploymentConfiguration'
import StrictModeSetup from '@/pages/docs/StrictModeSetup'

function TestApp({ initialPath }: { initialPath: string }) {
  return (
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route path="/" element={<Landing />} />
        <Route path="/docs" element={<DocsLayout />}>
          <Route index element={<Navigate to="getting-started" replace />} />
          <Route path="getting-started" element={<GettingStarted />} />
          <Route path="deployment-configuration" element={<DeploymentConfiguration />} />
          <Route path="strict-mode-setup" element={<StrictModeSetup />} />
        </Route>
      </Routes>
    </MemoryRouter>
  )
}

describe('Routes', () => {
  it('renders the landing page at /', () => {
    render(<TestApp initialPath="/" />)
    expect(
      screen.getByRole('heading', { name: /kubernetes is overkill/i }),
    ).toBeInTheDocument()
  })

  it('renders the Getting Started doc at /docs/getting-started', () => {
    render(<TestApp initialPath="/docs/getting-started" />)
    expect(
      screen.getByRole('heading', { name: /getting started/i }),
    ).toBeInTheDocument()
  })

  it('renders the Deployment Configuration doc at /docs/deployment-configuration', () => {
    render(<TestApp initialPath="/docs/deployment-configuration" />)
    expect(
      screen.getByRole('heading', { name: /deployment configuration/i }),
    ).toBeInTheDocument()
  })

  it('renders the Strict Mode Setup doc at /docs/strict-mode-setup', () => {
    render(<TestApp initialPath="/docs/strict-mode-setup" />)
    expect(
      screen.getByRole('heading', { name: /strict mode setup/i }),
    ).toBeInTheDocument()
  })

  it('/docs redirects to /docs/getting-started', () => {
    render(<TestApp initialPath="/docs" />)
    expect(
      screen.getByRole('heading', { name: /getting started/i }),
    ).toBeInTheDocument()
  })


  it('renders markdown elements for docs pages', () => {
    render(<TestApp initialPath="/docs/getting-started" />)

    expect(screen.getAllByRole('link', { name: /deployment configuration/i })[1]).toHaveAttribute(
      'href',
      '/docs/deployment-configuration',
    )
    expect(screen.getAllByText(/curl -fsSL/i)[0].tagName).toBe('CODE')
    expect(screen.getByText(/OS:/i)).toBeInTheDocument()
  })
})
