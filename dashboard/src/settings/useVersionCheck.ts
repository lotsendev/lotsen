import { useQuery } from '@tanstack/react-query'
import { getVersionInfo, type VersionInfo } from '../lib/api'

const VERSION_CHECK_INTERVAL_MS = 5 * 60 * 1000

const FALLBACK_VERSION_INFO: VersionInfo = {
  currentVersion: 'unknown',
  upgradeAvailable: false,
}

export function useVersionCheck() {
  const query = useQuery({
    queryKey: ['version-check'],
    queryFn: () => getVersionInfo(),
    staleTime: VERSION_CHECK_INTERVAL_MS,
    refetchInterval: VERSION_CHECK_INTERVAL_MS,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: true,
  })

  return {
    ...query,
    currentVersion: query.data?.currentVersion ?? FALLBACK_VERSION_INFO.currentVersion,
    latestVersion: query.data?.latestVersion,
    publishedAt: query.data?.publishedAt,
    releaseNotes: query.data?.releaseNotes,
    upgradeAvailable: query.data?.upgradeAvailable ?? FALLBACK_VERSION_INFO.upgradeAvailable,
    cachedAt: query.data?.cachedAt,
  }
}
