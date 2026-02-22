import { useSystemStatus } from './useSystemStatus'

function formatTimestamp(timestamp: string) {
  const date = new Date(timestamp)
  if (Number.isNaN(date.getTime())) {
    return 'Unknown'
  }
  return date.toLocaleString()
}

export function SystemStatusPanel() {
  const { status, isLoading, isError } = useSystemStatus()

  return (
    <section className="mb-6 rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
      <h2 className="mb-3 text-sm font-medium text-gray-700">System status</h2>

      {isLoading && <p className="text-sm text-gray-500">Loading system status…</p>}

      {isError && (
        <p className="text-sm text-red-600">Unable to fetch system status right now.</p>
      )}

      {status && !isLoading && !isError && (
        <div className="space-y-1 text-sm text-gray-700">
          <p>
            API signal:{' '}
            <span className="font-medium text-gray-900">{status.api.state}</span>
          </p>
          <p>
            Last updated:{' '}
            <span className="font-medium text-gray-900">{formatTimestamp(status.api.lastUpdated)}</span>
          </p>
        </div>
      )}
    </section>
  )
}
