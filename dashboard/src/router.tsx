import {
  Link,
  Outlet,
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
  useRouterState,
} from '@tanstack/react-router'
import DeploymentList from './pages/DeploymentList'
import { SystemStatusPage } from './pages/SystemStatusPage'

function DashboardLayout() {
  const pathname = useRouterState({ select: state => state.location.pathname })

  const navItemClass = (path: string) => {
    const isActive = pathname === path
    return `rounded-md px-3 py-2 text-left text-sm font-medium transition ${isActive ? 'bg-gray-900 text-white' : 'text-gray-700 hover:bg-gray-100'}`
  }

  return (
    <main className="mx-auto max-w-6xl px-4 py-8 sm:px-6 sm:py-10">
      <h1 className="mb-6 text-2xl font-semibold text-gray-900">Dashboard</h1>
      <div className="grid gap-6 lg:grid-cols-[220px_1fr]">
        <aside className="rounded-lg border border-gray-200 bg-white p-2 shadow-sm lg:h-fit">
          <nav className="flex gap-2 lg:flex-col" aria-label="Dashboard sections">
            <Link to="/deployments" className={navItemClass('/deployments')}>
              Deployments
            </Link>
            <Link to="/system-status" className={navItemClass('/system-status')}>
              System status
            </Link>
          </nav>
        </aside>
        <Outlet />
      </div>
    </main>
  )
}

const rootRoute = createRootRoute({
  component: DashboardLayout,
})

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  beforeLoad: () => {
    throw redirect({ to: '/deployments' })
  },
})

const deploymentsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/deployments',
  component: DeploymentList,
})

const systemStatusRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/system-status',
  component: SystemStatusPage,
})

const routeTree = rootRoute.addChildren([indexRoute, deploymentsRoute, systemStatusRoute])

export const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
