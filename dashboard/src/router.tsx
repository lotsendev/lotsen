import {
  Link,
  Outlet,
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
  useNavigate,
  useRouterState,
} from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Gauge, HardDrive, KeyRound, LogOut, Moon, PackageSearch, Radar, Rocket, ScrollText, Sun, UserRound, UserRoundCog } from 'lucide-react'
import { useEffect } from 'react'
import { Button } from './components/ui/button'
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
import { LoginPage } from './pages/LoginPage'
import { LogsPage } from './pages/LogsPage'
import { RegistriesPage } from './pages/RegistriesPage'
import { SettingsPage } from './pages/SettingsPage'
import { SystemStatusPage } from './pages/SystemStatusPage'
import { TrafficPage } from './pages/TrafficPage'
import { UsersPage } from './pages/UsersPage'
import { useAuth, useLogout } from './auth/useAuth'
import { getHostProfile } from './lib/api'
import { useVersionCheck } from './settings/useVersionCheck'
import { useTheme } from './theme'

function DashboardLayout() {
  const pathname = useRouterState({ select: state => state.location.pathname })
  const { theme, toggleTheme } = useTheme()
  const { upgradeAvailable } = useVersionCheck()
  const { isAuthenticated, isAuthDisabled, isLoading, username } = useAuth()
  const navigate = useNavigate()
  const logoutMutation = useLogout()
  const hostProfileQuery = useQuery({
    queryKey: ['hostProfile'],
    queryFn: getHostProfile,
    refetchInterval: 60_000,
  })

  useEffect(() => {
    if (!isLoading && !isAuthenticated && !isAuthDisabled) {
      navigate({ to: '/login', search: { redirect: pathname } })
    }
  }, [isLoading, isAuthenticated, isAuthDisabled, pathname, navigate])

  if (isLoading || (!isAuthenticated && !isAuthDisabled)) {
    return null
  }

  const isSystemStatusPage = pathname === '/system-status'
  const isHostPage = pathname === '/host' || pathname === '/settings'
  const isRegistriesPage = pathname === '/registries'
  const isUsersPage = pathname === '/users'
  const isTrafficPage = pathname === '/traffic'
  const isLogsPage = pathname === '/logs'
  const isDeploymentPage = pathname.startsWith('/deployments')
  const isDeploymentDetailPage = isDeploymentPage && pathname !== '/deployments'
  const hostName = hostProfileQuery.data?.displayName?.trim() || 'Unnamed host'
  const pageTitle = isSystemStatusPage
    ? 'System status'
    : isTrafficPage
      ? 'Traffic & security'
      : isLogsPage
      ? 'Logs'
      : isHostPage
      ? 'Host'
      : isRegistriesPage
      ? 'Registries'
      : isUsersPage
      ? 'Users'
      : isDeploymentDetailPage
        ? 'Deployment detail'
        : 'Deployments'
  const pageDescription = isSystemStatusPage
    ? 'Observe API health and freshness.'
    : isTrafficPage
      ? 'Inspect recent proxy traffic and effective protection settings.'
      : isLogsPage
      ? 'Inspect core service output and deployment log streams without SSH.'
      : isHostPage
      ? 'Manage host naming, metadata, and runtime upgrades.'
      : isRegistriesPage
      ? 'Manage private registry credentials used for deployment image pulls.'
      : isUsersPage
      ? 'Create users, rotate passwords, and revoke dashboard access.'
      : isDeploymentDetailPage
        ? 'Inspect deployment details and stream live logs.'
        : 'Create, edit, and monitor your active deployments.'

  return (
    <SidebarProvider className="chart-grid-overlay">
      <Sidebar collapsible="none">
        <SidebarHeader className="px-4 pt-6">
          <div className="mb-2 flex items-start justify-between gap-3">
            <div className="flex items-center gap-3">
              <div className="grid h-9 w-9 place-items-center rounded-xl border border-primary/30 bg-primary/12 text-primary">
                <Rocket className="h-4 w-4" />
              </div>
              <div>
                <p className="font-[family-name:var(--font-display)] text-lg font-semibold tracking-tight text-foreground">lotsen</p>
                <p className="font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground/80">control deck</p>
              </div>
            </div>
            <div className="flex items-center gap-1">
              <Button type="button" variant="ghost" size="icon" className="h-8 w-8" onClick={toggleTheme} aria-label="Toggle theme">
                {theme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
              </Button>
              {!isAuthDisabled && (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  onClick={() => logoutMutation.mutate()}
                  aria-label={username ? `Sign out ${username}` : 'Sign out'}
                  title={username ? `Sign out (${username})` : 'Sign out'}
                >
                  <LogOut className="h-4 w-4" />
                </Button>
              )}
            </div>
          </div>
          <div className="rounded-xl border border-border/70 bg-muted/45 p-3">
            <p className="font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground/85">Current host</p>
            <p className="mt-1 truncate text-sm font-semibold text-foreground">{hostName}</p>
          </div>
        </SidebarHeader>
        <SidebarContent className="px-4 pb-4 pt-1">
          <nav aria-label="Dashboard sections" className="p-1">
            <SidebarGroup className="p-0">
              <p className="px-2 pb-2 font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground/80">Operate</p>
              <SidebarGroupContent>
                <SidebarMenu>
                  <SidebarMenuItem>
                    <SidebarMenuButton asChild isActive={pathname.startsWith('/deployments')} size="lg" className="rounded-lg">
                      <Link to="/deployments">
                        <PackageSearch className="h-4 w-4 shrink-0" />
                        <span>Deployments</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                  <SidebarMenuItem>
                    <SidebarMenuButton asChild isActive={pathname === '/system-status'} size="lg" className="rounded-lg">
                      <Link to="/system-status">
                        <Gauge className="h-4 w-4 shrink-0" />
                        <span>System status</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                  <SidebarMenuItem>
                    <SidebarMenuButton asChild isActive={pathname === '/traffic'} size="lg" className="rounded-lg">
                      <Link to="/traffic">
                        <Radar className="h-4 w-4 shrink-0" />
                        <span>Traffic</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                  <SidebarMenuItem>
                    <SidebarMenuButton asChild isActive={pathname === '/logs'} size="lg" className="rounded-lg">
                      <Link to="/logs">
                        <ScrollText className="h-4 w-4 shrink-0" />
                        <span>Logs</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>

            <SidebarGroup className="mt-4 p-0">
              <p className="px-2 pb-2 font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground/80">Configure</p>
              <SidebarGroupContent>
                <SidebarMenu>
                  <SidebarMenuItem>
                    <SidebarMenuButton asChild isActive={pathname === '/users'} size="lg" className="rounded-lg">
                      <Link to="/users">
                        <UserRoundCog className="h-4 w-4 shrink-0" />
                        <span>Users</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                  <SidebarMenuItem>
                    <SidebarMenuButton asChild isActive={pathname === '/registries'} size="lg" className="rounded-lg">
                      <Link to="/registries">
                        <KeyRound className="h-4 w-4 shrink-0" />
                        <span>Registries</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                  <SidebarMenuItem>
                    <SidebarMenuButton asChild isActive={pathname === '/host' || pathname === '/settings'} size="lg" className="rounded-lg">
                      <Link to="/host">
                        <HardDrive className="h-4 w-4 shrink-0" />
                        <span>Host</span>
                        {upgradeAvailable && <span aria-label="Upgrade available" className="ml-auto h-2 w-2 rounded-full bg-primary" />}
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          </nav>

          <div className="mt-auto px-2 pt-4">
            <div className="flex items-center gap-2 border-t border-border/70 pt-3">
              <span className="grid h-6 w-6 place-items-center rounded-md border border-border/70 bg-background/80 text-muted-foreground">
                <UserRound className="h-3.5 w-3.5" />
              </span>
              <div className="min-w-0">
                <p className="font-mono text-[10px] uppercase tracking-[0.12em] text-muted-foreground/80">Signed in</p>
                <p className="truncate text-xs font-medium text-foreground">{isAuthDisabled ? 'Auth disabled' : username || 'Authenticated user'}</p>
              </div>
            </div>
          </div>
        </SidebarContent>
      </Sidebar>

      <SidebarInset>
        <div className="mx-auto w-full max-w-6xl">
          <section className="rounded-2xl border border-border/70 bg-card/92 p-4 shadow-sm backdrop-blur sm:p-5 lg:p-6">
              <p className="mb-2 font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
              {isSystemStatusPage || isTrafficPage || isLogsPage ? 'Observability' : isHostPage || isUsersPage || isRegistriesPage ? 'Configuration' : 'Deployments'}
              </p>
            <div className="mb-4">
              <h1 className="font-[family-name:var(--font-display)] text-2xl font-semibold tracking-tight text-foreground">{pageTitle}</h1>
              <p className="mt-1 text-sm text-muted-foreground">{pageDescription}</p>
            </div>
            {isDeploymentPage ? (
              <Outlet />
            ) : (
              <div className="space-y-4">
                <Outlet />
              </div>
            )}
          </section>
        </div>
      </SidebarInset>
    </SidebarProvider>
  )
}

const rootRoute = createRootRoute({
  component: () => <Outlet />,
})

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  component: LoginPage,
  validateSearch: (search: Record<string, unknown>) => ({
    redirect: typeof search.redirect === 'string' ? search.redirect : undefined,
  }),
})

const appRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: '_app',
  component: DashboardLayout,
})

const indexRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/',
  beforeLoad: () => {
    throw redirect({ to: '/deployments' })
  },
})

const deploymentsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/deployments',
  component: DeploymentList,
})

const deploymentDetailRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/deployments/$deploymentId',
  component: DeploymentDetailPage,
})

const systemStatusRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/system-status',
  component: SystemStatusPage,
})

const trafficRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/traffic',
  component: TrafficPage,
})

const logsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/logs',
  component: LogsPage,
})

const hostRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/host',
  component: SettingsPage,
})

const settingsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/settings',
  beforeLoad: () => {
    throw redirect({ to: '/host' })
  },
})

const usersRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/users',
  component: UsersPage,
})

const registriesRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/registries',
  component: RegistriesPage,
})

const routeTree = rootRoute.addChildren([
  loginRoute,
  appRoute.addChildren([
    indexRoute,
    deploymentsRoute,
    deploymentDetailRoute,
    systemStatusRoute,
    trafficRoute,
    logsRoute,
    usersRoute,
    registriesRoute,
    hostRoute,
    settingsRoute,
  ]),
])

export const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
