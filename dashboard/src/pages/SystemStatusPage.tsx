import { SystemStatusPanel } from '../system-status/SystemStatusPanel'

export function SystemStatusPage() {
  return (
    <section>
      <h2 className="mb-5 text-xl font-semibold text-gray-900">System status</h2>
      <SystemStatusPanel />
    </section>
  )
}
