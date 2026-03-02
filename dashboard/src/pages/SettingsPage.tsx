import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, RefreshCw } from 'lucide-react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Badge } from '../components/ui/badge'
import { Button } from '../components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '../components/ui/dialog'
import { Input } from '../components/ui/input'
import {
  getHostProfile,
  getVersionInfo,
  getVersionReleases,
  triggerUpgrade,
  updateHostProfile,
  type VersionRelease,
} from '../lib/api'
import { UpgradeLogPanel } from '../settings/UpgradeLogPanel'
import { useUpgradeLogsSSE } from '../settings/useUpgradeLogsSSE'
import { useVersionCheck } from '../settings/useVersionCheck'

const markdownComponents = {
  h1: ({ children }: { children?: ReactNode }) => (
    <h3 className="mb-2 text-base font-semibold tracking-tight text-foreground">{children}</h3>
  ),
  h2: ({ children }: { children?: ReactNode }) => (
    <h4 className="mb-2 text-sm font-semibold tracking-tight text-foreground">{children}</h4>
  ),
  h3: ({ children }: { children?: ReactNode }) => <h5 className="mb-1.5 text-sm font-semibold text-foreground">{children}</h5>,
  p: ({ children }: { children?: ReactNode }) => <p className="mb-2 leading-6 text-muted-foreground last:mb-0">{children}</p>,
  ul: ({ children }: { children?: ReactNode }) => <ul className="mb-2 list-disc space-y-1 pl-5 text-muted-foreground last:mb-0">{children}</ul>,
  ol: ({ children }: { children?: ReactNode }) => <ol className="mb-2 list-decimal space-y-1 pl-5 text-muted-foreground last:mb-0">{children}</ol>,
  li: ({ children }: { children?: ReactNode }) => <li className="leading-6">{children}</li>,
  a: ({ href, children }: { href?: string; children?: ReactNode }) => (
    <a href={href} target="_blank" rel="noreferrer" className="font-medium text-primary underline underline-offset-2 hover:text-primary/80">
      {children}
    </a>
  ),
  code: ({ children }: { children?: ReactNode }) => <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs text-foreground">{children}</code>,
  pre: ({ children }: { children?: ReactNode }) => (
    <pre className="mb-2 overflow-x-auto rounded-md border border-border/60 bg-background/80 p-3 font-mono text-xs text-foreground last:mb-0">
      {children}
    </pre>
  ),
  blockquote: ({ children }: { children?: ReactNode }) => (
    <blockquote className="mb-2 border-l-2 border-border pl-3 text-muted-foreground italic last:mb-0">{children}</blockquote>
  ),
}

function formatDate(value?: string) {
  if (!value) return 'Unavailable'
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return 'Unknown'
  return parsed.toLocaleString()
}

function formatBytes(value?: number) {
  if (!value || value <= 0) return 'Unavailable'

  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = value
  let unitIndex = 0
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex += 1
  }

  const precision = unitIndex < 2 ? 0 : 1
  return `${size.toFixed(precision)} ${units[unitIndex]}`
}

function parseSemver(raw?: string): [number, number, number] | null {
  if (!raw) return null
  const match = raw.trim().match(/^v?(\d+)\.(\d+)\.(\d+)$/)
  if (!match) return null
  return [Number(match[1]), Number(match[2]), Number(match[3])]
}

function compareSemver(a?: string, b?: string) {
  const parsedA = parseSemver(a)
  const parsedB = parseSemver(b)
  if (!parsedA || !parsedB) return 0

  if (parsedA[0] !== parsedB[0]) return parsedA[0] - parsedB[0]
  if (parsedA[1] !== parsedB[1]) return parsedA[1] - parsedB[1]
  return parsedA[2] - parsedB[2]
}

function getUpgradePath(currentVersion: string, latestVersion: string | undefined, releases: VersionRelease[], fallbackRelease?: VersionRelease) {
  if (!latestVersion) return []

  const deduped = new Map<string, VersionRelease>()
  for (const release of releases) {
    if (release.version) {
      deduped.set(release.version, release)
    }
  }
  if (fallbackRelease?.version && !deduped.has(fallbackRelease.version)) {
    deduped.set(fallbackRelease.version, fallbackRelease)
  }

  const all = [...deduped.values()]
  return all
    .filter(release => compareSemver(release.version, currentVersion) > 0 && compareSemver(release.version, latestVersion) <= 0)
    .sort((a, b) => compareSemver(b.version, a.version))
}

function FieldTile({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <article className="rounded-lg border border-border/55 bg-background/70 px-3 py-2.5">
      <p className="text-[10px] uppercase tracking-[0.14em] text-muted-foreground">{label}</p>
      <p className={mono ? 'mt-1 font-mono text-sm text-foreground' : 'mt-1 text-sm text-foreground'}>{value}</p>
    </article>
  )
}

function RunwayStat({ label, value, status }: { label: string; value: string; status?: ReactNode }) {
  return (
    <article className="rounded-lg border border-border/55 bg-background/70 p-3">
      <p className="text-[10px] uppercase tracking-[0.14em] text-muted-foreground">{label}</p>
      <div className="mt-1.5 flex items-center gap-2">
        <p className="font-mono text-sm text-foreground">{value}</p>
        {status}
      </div>
    </article>
  )
}

export function SettingsPage() {
  const queryClient = useQueryClient()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [selectedUpgradeTarget, setSelectedUpgradeTarget] = useState<string | null>(null)
  const [isUpgradeRunning, setIsUpgradeRunning] = useState(false)
  const [awaitingReconnect, setAwaitingReconnect] = useState(false)
  const [reconnectSawOffline, setReconnectSawOffline] = useState(false)
  const [reconnectReady, setReconnectReady] = useState(false)
  const [targetVersion, setTargetVersion] = useState<string | null>(null)
  const [upgradeError, setUpgradeError] = useState<string | null>(null)
  const [hostNameDraft, setHostNameDraft] = useState('')
  const [hostUpdateError, setHostUpdateError] = useState<string | null>(null)
  const [attemptId, setAttemptId] = useState(0)

  const { currentVersion, latestVersion, publishedAt, releaseNotes, upgradeAvailable, cachedAt, isLoading, isError } = useVersionCheck()
  const hostProfileQuery = useQuery({
    queryKey: ['hostProfile'],
    queryFn: getHostProfile,
  })
  const hostMetadata = hostProfileQuery.data?.metadata
  const hasHostMetadata = Boolean(
    hostMetadata?.ipAddress ||
      hostMetadata?.osName ||
      hostMetadata?.osVersion ||
      hostMetadata?.specs?.cpuCores ||
      hostMetadata?.specs?.memoryBytes ||
      hostMetadata?.specs?.diskBytes
  )
  const { data: releases = [], isLoading: releasesLoading } = useQuery({
    queryKey: ['version-releases'],
    queryFn: () => getVersionReleases(30),
    staleTime: 60 * 60 * 1000,
  })

  useEffect(() => {
    let active = true

    getVersionInfo({ forceRefresh: true })
      .then(snapshot => {
        if (!active) return
        queryClient.setQueryData(['version-check'], snapshot)
      })
      .catch(() => {
        // keep cached version info if force refresh fails
      })

    return () => {
      active = false
    }
  }, [queryClient])
  const { lines, streamClosed } = useUpgradeLogsSSE(isUpgradeRunning)

  const reconnectProbe = useQuery({
    queryKey: ['upgrade-reconnect-probe', attemptId],
    queryFn: () => getVersionInfo(),
    enabled: awaitingReconnect,
    retry: false,
    refetchInterval: 3000,
    refetchIntervalInBackground: true,
  })

  const fallbackLatestRelease = latestVersion
    ? {
        version: latestVersion,
        publishedAt,
        releaseNotes: releaseNotes ?? '',
      }
    : undefined

  const upgradePath = useMemo(
    () => getUpgradePath(currentVersion, latestVersion, releases, fallbackLatestRelease),
    [currentVersion, fallbackLatestRelease, latestVersion, releases],
  )

  const finishUpgradeRun = (reconnectedCurrentVersion?: string, reconnectedLatestVersion?: string) => {
    setAwaitingReconnect(false)
    setIsUpgradeRunning(false)
    setReconnectReady(true)

    if (reconnectProbe.data) {
      queryClient.setQueryData(['version-check'], reconnectProbe.data)
    }

    const targetReached = targetVersion && (reconnectedCurrentVersion === targetVersion || reconnectedLatestVersion === targetVersion)
    if (targetVersion && !targetReached) {
      const lastLines = lines.slice(-8).join('\n')
      setUpgradeError(lastLines ? `Upgrade did not reach ${targetVersion}. Last log lines:\n${lastLines}` : `Upgrade did not reach ${targetVersion}.`)
      return
    }

    setUpgradeError(null)
  }

  useEffect(() => {
    if (!awaitingReconnect) return

    if (reconnectProbe.isError) {
      setReconnectSawOffline(true)
      return
    }

    if (reconnectProbe.isSuccess && reconnectSawOffline) {
      finishUpgradeRun(reconnectProbe.data.currentVersion, reconnectProbe.data.latestVersion)
      return
    }
  }, [awaitingReconnect, reconnectProbe.isError, reconnectProbe.isSuccess, reconnectSawOffline, reconnectProbe.data])

  const startUpgrade = useMutation({
    mutationFn: (nextTarget: string) => triggerUpgrade(nextTarget),
    onSuccess: (_, requestedTarget) => {
      setConfirmOpen(false)
      setUpgradeError(null)
      setReconnectReady(false)
      setReconnectSawOffline(false)
      setAwaitingReconnect(true)
      setIsUpgradeRunning(true)
      setTargetVersion(requestedTarget || latestVersion || null)
      setAttemptId(prev => prev + 1)
    },
    onError: error => {
      setUpgradeError(error instanceof Error ? error.message : 'Failed to start upgrade')
      setConfirmOpen(false)
    },
  })

  useEffect(() => {
    setHostNameDraft(hostProfileQuery.data?.displayName ?? '')
  }, [hostProfileQuery.data?.displayName])

  const hostNameUpdate = useMutation({
    mutationFn: updateHostProfile,
    onSuccess: profile => {
      queryClient.setQueryData(['hostProfile'], profile)
      setHostUpdateError(null)
    },
    onError: error => {
      setHostUpdateError(error instanceof Error ? error.message : 'Failed to update host name')
    },
  })

  const activeTarget = selectedUpgradeTarget ?? (upgradeAvailable ? latestVersion ?? null : null)
  const canStartUpgrade = upgradeAvailable && Boolean(activeTarget) && !isUpgradeRunning && !startUpgrade.isPending

  return (
    <section className="space-y-4">
      {isLoading && <p className="text-sm text-muted-foreground">Checking version information...</p>}

      {isError && <p className="text-sm text-destructive">Unable to fetch version information right now.</p>}

      {reconnectReady && (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-[#2a7a64]/30 bg-[#2a7a64]/10 px-4 py-3 text-sm text-[#1f5f4f] dark:border-[#2a7a64]/40 dark:bg-[#2a7a64]/20 dark:text-[#93d0bc]">
          <p>Upgrade complete - reload to connect to the updated runtime.</p>
          <Button type="button" size="sm" onClick={() => window.location.reload()}>
            <RefreshCw className="h-4 w-4" />
            Reload dashboard
          </Button>
        </div>
      )}

      {upgradeError && (
        <div className="rounded-xl border border-destructive/40 bg-destructive/10 px-4 py-3 text-destructive">
          <div className="flex items-start gap-2">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
            <p className="text-sm font-medium">Upgrade failed</p>
          </div>
          <pre className="mt-2 overflow-x-auto whitespace-pre-wrap font-mono text-xs">{upgradeError}</pre>
        </div>
      )}

      <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Host dossier</p>
            <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">Identity and runtime facts</h2>
            <p className="mt-1 text-sm text-muted-foreground">Set the host label and confirm machine facts before making runtime changes.</p>
          </div>
          <Badge variant="outline" className="rounded-md border-border/70 bg-background/70 px-2.5 py-1 font-mono text-[11px]">
            {hostProfileQuery.data?.displayName?.trim() || 'Unnamed host'}
          </Badge>
        </div>

        <div className="mt-4 grid gap-3 lg:grid-cols-[minmax(0,1.1fr)_minmax(0,1.4fr)]">
          <section className="rounded-lg border border-border/55 bg-background/70 p-4">
            <p className="text-[10px] uppercase tracking-[0.14em] text-muted-foreground">Host identity</p>
            <p className="mt-1 text-sm text-muted-foreground">Name shown in navigation and logs.</p>

            <form
              className="mt-3 flex flex-col gap-2 sm:flex-row"
              onSubmit={event => {
                event.preventDefault()
                hostNameUpdate.mutate(hostNameDraft)
              }}
            >
              <Input
                value={hostNameDraft}
                onChange={event => setHostNameDraft(event.target.value)}
                placeholder="e.g. prod-eu-1"
                maxLength={64}
                className="sm:max-w-sm"
                aria-label="Host display name"
              />
              <Button type="submit" disabled={hostNameUpdate.isPending || hostProfileQuery.isLoading}>
                Save host name
              </Button>
            </form>

            {hostUpdateError && <p className="mt-2 text-sm text-destructive">{hostUpdateError}</p>}
          </section>

          <section className="rounded-lg border border-border/55 bg-background/70 p-4">
            <p className="text-[10px] uppercase tracking-[0.14em] text-muted-foreground">Machine runtime facts</p>
            <p className="mt-1 text-sm text-muted-foreground">Snapshot from the active host process.</p>

            {hasHostMetadata ? (
              <div className="mt-3 grid gap-2 sm:grid-cols-2">
                <FieldTile label="IP address" value={hostMetadata?.ipAddress || 'Unavailable'} mono />
                <FieldTile label="Operating system" value={[hostMetadata?.osName, hostMetadata?.osVersion].filter(Boolean).join(' ') || 'Unavailable'} />
                <FieldTile label="CPU" value={hostMetadata?.specs?.cpuCores ? `${hostMetadata.specs.cpuCores} cores` : 'Unavailable'} />
                <FieldTile label="Memory" value={formatBytes(hostMetadata?.specs?.memoryBytes)} mono />
                <FieldTile label="Disk" value={formatBytes(hostMetadata?.specs?.diskBytes)} mono />
              </div>
            ) : (
              <p className="mt-3 text-sm text-muted-foreground">Metadata unavailable.</p>
            )}
          </section>
        </div>
      </section>

      <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Release runway</p>
            <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">Installed and discovered versions</h2>
            <p className="mt-1 text-sm text-muted-foreground">Check runtime state and jump directly to the newest valid target.</p>
          </div>
          {latestVersion && upgradeAvailable && upgradePath.length > 0 && (
            <Button
              type="button"
              disabled={!canStartUpgrade}
              onClick={() => {
                setSelectedUpgradeTarget(latestVersion)
                setConfirmOpen(true)
              }}
            >
              {startUpgrade.isPending || isUpgradeRunning ? 'Upgrading...' : `Upgrade to ${latestVersion}`}
            </Button>
          )}
        </div>

        <div className="mt-4 grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
          <RunwayStat label="Installed version" value={currentVersion} />
          <RunwayStat
            label="Latest discovered"
            value={latestVersion ?? 'Unavailable'}
            status={upgradeAvailable ? <Badge variant="info">behind</Badge> : <Badge variant="secondary">current</Badge>}
          />
          <RunwayStat label="Latest published" value={formatDate(publishedAt)} />
          <RunwayStat label="Cache refreshed" value={formatDate(cachedAt)} />
        </div>
      </section>

      <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Upgrade sequence</p>
            <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">Available versions</h2>
            <p className="mt-1 text-sm text-muted-foreground">Runway ledger of releases between installed and latest.</p>
          </div>
          <Badge variant="outline" className="rounded-md border-border/70 bg-background/70 px-2.5 py-1 font-mono text-[11px]">
            {upgradePath.length} hop{upgradePath.length === 1 ? '' : 's'}
          </Badge>
        </div>

        {releasesLoading ? (
          <p className="mt-4 text-sm text-muted-foreground">Loading release list...</p>
        ) : upgradePath.length === 0 ? (
          <div className="mt-4 rounded-lg border border-border/60 bg-background/70 p-4 text-sm text-muted-foreground">
            {upgradeAvailable ? 'No upgrade path entries available yet.' : 'This installation already matches the latest discovered version.'}
          </div>
        ) : (
          <div className="mt-4 space-y-3">
            {upgradePath.map(release => {
              const isLatest = release.version === latestVersion
              const isCurrent = release.version === currentVersion
              const canUpgradeToRelease = upgradeAvailable && !isCurrent && !isUpgradeRunning && !startUpgrade.isPending

              return (
                <article key={release.version} className="rounded-lg border border-border/55 bg-background/70 p-3">
                  <div className="flex gap-3">
                    <div className="mt-1 flex shrink-0 flex-col items-center">
                      <span className="h-2.5 w-2.5 rounded-full bg-border" />
                      {!isLatest && <span className="mt-1 h-full w-px bg-border/70" />}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center justify-between gap-3">
                        <div className="flex flex-wrap items-center gap-2">
                          <p className="font-mono text-sm font-medium text-foreground">{release.version}</p>
                          {isCurrent && <Badge variant="secondary">installed</Badge>}
                          {isLatest && <Badge variant="info">latest</Badge>}
                          <span className="text-xs text-muted-foreground">Published {formatDate(release.publishedAt)}</span>
                        </div>
                        <Button
                          type="button"
                          size="sm"
                          disabled={!canUpgradeToRelease}
                          onClick={() => {
                            setSelectedUpgradeTarget(release.version)
                            setConfirmOpen(true)
                          }}
                        >
                          Upgrade to {release.version}
                        </Button>
                      </div>

                      <details className="mt-3 rounded-md border border-border/60 bg-background p-3">
                        <summary className="cursor-pointer text-xs font-medium uppercase tracking-[0.13em] text-muted-foreground">Release notes</summary>
                        <div className="mt-3 text-sm text-foreground">
                          {release.releaseNotes?.trim() ? (
                            <ReactMarkdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
                              {release.releaseNotes}
                            </ReactMarkdown>
                          ) : (
                            <p className="text-sm text-muted-foreground">No release notes available.</p>
                          )}
                        </div>
                      </details>
                    </div>
                  </div>
                </article>
              )
            })}
          </div>
        )}
      </section>

      {(isUpgradeRunning || lines.length > 0) && <UpgradeLogPanel lines={lines} isRunning={isUpgradeRunning} streamClosed={streamClosed} />}

      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>Confirm upgrade</DialogTitle>
            <DialogDescription>
              Lotsen services will briefly restart while the installer runs. Continue only if a short interruption is safe.
            </DialogDescription>
          </DialogHeader>
          <div className="rounded-lg border border-border/60 bg-background/70 p-3 text-sm text-muted-foreground">
            <p className="font-medium text-foreground">Target version</p>
            <p className="mt-1 font-mono text-sm text-foreground">{activeTarget ?? 'latest'}</p>
            <ul className="mt-2 space-y-1 text-xs">
              <li>- API and orchestrator restart briefly.</li>
              <li>- Dashboard may disconnect for a short window.</li>
              <li>- A reconnect check confirms upgrade completion.</li>
            </ul>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setConfirmOpen(false)}>
              Cancel
            </Button>
            <Button
              type="button"
              onClick={() => {
                if (!activeTarget) return
                startUpgrade.mutate(activeTarget)
              }}
              disabled={!activeTarget || startUpgrade.isPending}
            >
              Start upgrade
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  )
}
