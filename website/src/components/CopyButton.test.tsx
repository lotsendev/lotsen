import { act, fireEvent, render, screen } from '@testing-library/react'
import { CopyButton } from './CopyButton'

describe('CopyButton', () => {
  let writeText: ReturnType<typeof vi.fn>

  beforeEach(() => {
    writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      writable: true,
      configurable: true,
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders with Copy label initially', () => {
    render(<CopyButton text="hello" />)
    expect(screen.getByRole('button', { name: /copy to clipboard/i })).toBeInTheDocument()
    expect(screen.getByText('Copy')).toBeInTheDocument()
  })

  it('calls clipboard.writeText with the provided text on click', async () => {
    render(<CopyButton text="some install command" />)
    await act(async () => {
      fireEvent.click(screen.getByRole('button'))
    })
    expect(writeText).toHaveBeenCalledWith('some install command')
    expect(writeText).toHaveBeenCalledTimes(1)
  })

  it('shows Copied! after a successful click', async () => {
    render(<CopyButton text="test" />)
    fireEvent.click(screen.getByRole('button'))
    expect(await screen.findByText('Copied!')).toBeInTheDocument()
  })
})
