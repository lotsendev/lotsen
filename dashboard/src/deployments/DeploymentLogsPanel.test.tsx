import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { act, render, screen } from '@testing-library/react'
import { DeploymentLogsPanel } from './DeploymentLogsPanel'

class MockEventSource {
  static instances: MockEventSource[] = []
  onmessage: ((event: MessageEvent) => void) | null = null
  close = vi.fn()

  constructor(public readonly url: string) {
    MockEventSource.instances.push(this)
  }
}

describe('DeploymentLogsPanel', () => {
  beforeEach(() => {
    MockEventSource.instances = []
    vi.stubGlobal('EventSource', MockEventSource)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('renders incoming SSE lines and closes stream on unmount', () => {
    const { unmount } = render(<DeploymentLogsPanel deploymentId="dep-1" status="healthy" />)

    expect(screen.getByText('Waiting for log output...')).toBeInTheDocument()
    expect(MockEventSource.instances[0]?.url).toBe('/api/deployments/dep-1/logs')

    act(() => {
      MockEventSource.instances[0]?.onmessage?.({
        data: JSON.stringify({ line: 'server started' }),
      } as MessageEvent)
      MockEventSource.instances[0]?.onmessage?.({
        data: JSON.stringify({ line: 'listening on :8080' }),
      } as MessageEvent)
    })

    expect(screen.getByText(/server started/)).toHaveTextContent('server started')
    expect(screen.getByText(/server started/)).toHaveTextContent('listening on :8080')

    unmount()
    expect(MockEventSource.instances[0]?.close).toHaveBeenCalledTimes(1)
  })

  it('shows failed empty state when no lines are available', () => {
    render(<DeploymentLogsPanel deploymentId="dep-2" status="failed" error="container exited with code 1" />)

    expect(screen.getByText(/No container logs were captured/)).toHaveTextContent(
      'No container logs were captured for this failed deployment. Last error: container exited with code 1',
    )
  })
})
