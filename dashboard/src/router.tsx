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
import { SystemStatusPage } from './pages/SystemStatusPage'
import { useTheme } from './theme'

function DashboardLayout() {
  const pathname = useRouterState({ select: state => state.location.pathname })
  const { theme, toggleTheme } = useTheme()

  return (
    <SidebarProvider>
      <Sidebar collapsible="none">
        <SidebarHeader className="px-4 pt-6">
          <div className="mb-2 flex items-start justify-between gap-3">
            <div className="flex items-center gap-3">
              <div className="grid h-10 w-10 place-items-center rounded-xl bg-foreground text-background">
                <Rocket className="h-4 w-4" />
              </div>
              <div>
                <p className="text-base font-semibold leading-tight">Dirigent</p>
                <p className="text-sm text-muted-foreground">Dashboard</p>
              </div>
            </div>
            <Button type="button" variant="ghost" size="icon" className="h-8 w-8" onClick={toggleTheme} aria-label="Toggle theme">
              {theme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
            </Button>
          </div>
        </SidebarHeader>
        <SidebarContent className="px-4 pb-4 pt-1">
          <nav aria-label="Dashboard sections" className="rounded-2xl border bg-card p-3">
            <SidebarGroup className="p-0">
              <SidebarGroupContent>
                <SidebarMenu>
                  <SidebarMenuItem>
                    <SidebarMenuButton asChild isActive={pathname === '/deployments'} size="lg" className="rounded-lg">
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
        <p className="mb-4 text-sm text-muted-foreground">{pathname === '/system-status' ? 'Observability' : 'Deployments'}</p>
        <Card className="mx-auto w-full max-w-5xl">
          <CardHeader>
            <CardTitle>{pathname === '/system-status' ? 'System status' : 'Deployments'}</CardTitle>
            <CardDescription>
              {pathname === '/system-status'
                ? 'Observe API health and freshness.'
                : 'Create, edit, and monitor your active deployments.'}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Outlet />
          </CardContent>
        </Card>
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
