import { useEffect, useRef } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../components/ui/card'

type Props = {
  lines: string[]
  isRunning: boolean
  streamClosed: boolean
}

export function UpgradeLogPanel({ lines, isRunning, streamClosed }: Props) {
  const logContainerRef = useRef<HTMLPreElement | null>(null)

  useEffect(() => {
    if (!logContainerRef.current) return
    logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight
  }, [lines])

  return (
    <Card>
      <CardHeader>
        <CardTitle>Upgrade logs</CardTitle>
        <CardDescription>
          {isRunning
            ? 'Live installer output while the upgrade runs.'
            : streamClosed
              ? 'Log stream closed.'
              : 'Logs appear here after an upgrade starts.'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <pre
          ref={logContainerRef}
          className="h-72 overflow-y-auto rounded-lg border bg-muted/30 p-4 font-mono text-xs leading-5 text-foreground whitespace-pre-wrap break-all"
        >
          {lines.length ? lines.join('\n') : 'Waiting for upgrade output...'}
        </pre>
      </CardContent>
    </Card>
  )
}
