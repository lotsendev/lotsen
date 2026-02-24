import { useEffect, useState } from 'react'
import type { UpgradeLogEvent } from '../lib/api'

export function useUpgradeLogsSSE(enabled: boolean) {
  const [lines, setLines] = useState<string[]>([])
  const [streamClosed, setStreamClosed] = useState(false)

  useEffect(() => {
    if (!enabled) {
      setLines([])
      setStreamClosed(false)
      return
    }

    setLines([])
    setStreamClosed(false)
    const es = new EventSource('/api/upgrade/logs')

    es.onmessage = (event: MessageEvent) => {
      try {
        const { line }: UpgradeLogEvent = JSON.parse(event.data)
        setLines(prev => [...prev, line])
      } catch {
        // ignore malformed events
      }
    }

    es.onerror = () => {
      setStreamClosed(true)
      es.close()
    }

    return () => es.close()
  }, [enabled])

  return { lines, streamClosed }
}
