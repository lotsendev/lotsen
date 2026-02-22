import {
  Link,
  Outlet,
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
  useRouterState,
} from '@tanstack/react-router'
import { Boxes, Moon, Rocket, Server, Sun } from 'lucide-react'
import { Button } from './components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './components/ui/card'
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
} from './components/ui/sidebar'
import DeploymentList from './pages/DeploymentList'
import { DeploymentDetailPage } from './pages/DeploymentDetailPage'
import { SystemStatusPage } from './pages/SystemStatusPage'
import { useTheme } from './theme'

function DashboardLayout() {
  const pathname = useRouterState({ select: state => state.location.pathname })
  const { theme, toggleTheme } = useTheme()
  const isSystemStatusPage = pathname === '/system-status'
  const isDeploymentPage = pathname.startsWith('/deployments')
  const isDeploymentDetailPage = isDeploymentPage && pathname !== '/deployments'
  const pageTitle = isSystemStatusPage ? 'System status' : isDeploymentDetailPage ? 'Deployment detail' : 'Deployments'
  const pageDescription = isSystemStatusPage
    ? 'Observe API health and freshness.'
    : isDeploymentDetailPage
      ? 'Inspect deployment details and stream live logs.'
      : 'Create, edit, and monitor your active deployments.'

  return (
    <SidebarProvider>
      <Sidebar collapsible="none">
        <SidebarHeader className="px-4 pt-6">
          <div className="mb-2 flex items-start justify-between gap-3">
            <div className="flex items-center gap-3">
              <div className="grid h-9 w-9 place-items-center rounded-xl bg-primary text-primary-foreground">
                <Rocket className="h-4 w-4" />
              </div>
              <p className="font-[family-name:var(--font-display)] text-lg font-bold tracking-tight text-foreground">
                dirigent
              </p>
            </div>
            <Button type="button" variant="ghost" size="icon" className="h-8 w-8" onClick={toggleTheme} aria-label="Toggle theme">
              {theme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
            </Button>
          </div>
        </SidebarHeader>
        <SidebarContent className="px-4 pb-4 pt-1">
          <nav aria-label="Dashboard sections" className="p-1">
            <SidebarGroup className="p-0">
              <SidebarGroupContent>
                <SidebarMenu>
                  <SidebarMenuItem>
                    <SidebarMenuButton asChild isActive={pathname.startsWith('/deployments')} size="lg" className="rounded-lg">
                      <Link to="/deployments">
                        <Boxes className="h-4 w-4 shrink-0" />
                        <span>Deployments</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                  <SidebarMenuItem>
                    <SidebarMenuButton asChild isActive={pathname === '/system-status'} size="lg" className="rounded-lg">
                      <Link to="/system-status">
                        <Server className="h-4 w-4 shrink-0" />
                        <span>System status</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          </nav>
        </SidebarContent>
      </Sidebar>

      <SidebarInset>
        <p className="mb-4 text-sm text-muted-foreground">{isSystemStatusPage ? 'Observability' : 'Deployments'}</p>
        {isDeploymentDetailPage ? (
          <div className="mx-auto w-full max-w-5xl space-y-4">
            <div>
              <h1 className="font-[family-name:var(--font-display)] text-2xl font-semibold tracking-tight">{pageTitle}</h1>
              <p className="text-sm text-muted-foreground">{pageDescription}</p>
            </div>
            <Outlet />
          </div>
        ) : (
          <Card className="mx-auto w-full max-w-5xl">
            <CardHeader>
              <CardTitle>{pageTitle}</CardTitle>
              <CardDescription>{pageDescription}</CardDescription>
            </CardHeader>
            <CardContent>
              <Outlet />
            </CardContent>
          </Card>
        )}
      </SidebarInset>
    </SidebarProvider>
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

const deploymentDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/deployments/$deploymentId',
  component: DeploymentDetailPage,
})

const systemStatusRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/system-status',
  component: SystemStatusPage,
})

const routeTree = rootRoute.addChildren([indexRoute, deploymentsRoute, deploymentDetailRoute, systemStatusRoute])

export const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
