import { useEffect, useState } from 'react'
import CreateDeploymentForm from '../deployments/CreateDeploymentForm'
import EditDeploymentForm from '../deployments/EditDeploymentForm'
import { DeploymentTable } from '../deployments/DeploymentTable'
import { useDeploymentList } from '../deployments/useDeploymentList'
import { useDeploymentSSE } from '../deployments/useDeploymentSSE'
import type { Deployment } from '../lib/api'
import { SystemStatusPanel } from '../system-status/SystemStatusPanel'

type View = 'deployments' | 'system-status'

const VIEW_PATHS: Record<View, string> = {
  deployments: '/deployments',
  'system-status': '/system-status',
}

function getViewFromPath(pathname: string): View {
  if (pathname === VIEW_PATHS['system-status']) return 'system-status'
  return 'deployments'
}

export default function DeploymentList() {
  useDeploymentSSE()
  const listState = useDeploymentList()
  const [editingDeployment, setEditingDeployment] = useState<Deployment | null>(null)
  const [activeView, setActiveView] = useState<View>(() => getViewFromPath(window.location.pathname))

  useEffect(() => {
    if (window.location.pathname === '/') {
      window.history.replaceState({}, '', VIEW_PATHS.deployments)
      setActiveView('deployments')
    }

    const onPopState = () => setActiveView(getViewFromPath(window.location.pathname))
    window.addEventListener('popstate', onPopState)
    return () => window.removeEventListener('popstate', onPopState)
  }, [])

  function navigateTo(view: View) {
    const path = VIEW_PATHS[view]
    if (window.location.pathname !== path) {
      window.history.pushState({}, '', path)
    }
    setActiveView(view)
  }

  return (
    <main className="mx-auto max-w-6xl px-4 py-8 sm:px-6 sm:py-10">
      <h1 className="mb-6 text-2xl font-semibold text-gray-900">Dashboard</h1>
      <div className="grid gap-6 lg:grid-cols-[220px_1fr]">
        <aside className="rounded-lg border border-gray-200 bg-white p-2 shadow-sm lg:h-fit">
          <nav className="flex gap-2 lg:flex-col" aria-label="Dashboard sections">
            <button
              type="button"
              onClick={() => navigateTo('deployments')}
              className={`rounded-md px-3 py-2 text-left text-sm font-medium transition ${activeView === 'deployments' ? 'bg-gray-900 text-white' : 'text-gray-700 hover:bg-gray-100'}`}
              aria-current={activeView === 'deployments' ? 'page' : undefined}
            >
              Deployments
            </button>
            <button
              type="button"
              onClick={() => navigateTo('system-status')}
              className={`rounded-md px-3 py-2 text-left text-sm font-medium transition ${activeView === 'system-status' ? 'bg-gray-900 text-white' : 'text-gray-700 hover:bg-gray-100'}`}
              aria-current={activeView === 'system-status' ? 'page' : undefined}
            >
              System status
            </button>
          </nav>
        </aside>

        <section>
          {activeView === 'deployments' && (
            <>
              <h2 className="mb-5 text-xl font-semibold text-gray-900">Deployments</h2>
              {editingDeployment
                ? <EditDeploymentForm key={editingDeployment.id} deployment={editingDeployment} onClose={() => setEditingDeployment(null)} />
                : <CreateDeploymentForm />}
              <DeploymentTable {...listState} onEdit={setEditingDeployment} />
            </>
          )}

          {activeView === 'system-status' && (
            <>
              <h2 className="mb-5 text-xl font-semibold text-gray-900">System status</h2>
              <SystemStatusPanel />
            </>
          )}
        </section>
      </div>
    </main>
  )
}
