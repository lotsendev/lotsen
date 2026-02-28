import { useEffect, useRef } from 'react'

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
    <section className="rounded-xl border border-border/60 bg-card p-4 sm:p-5">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.13em] text-muted-foreground">Installer stream</p>
        <h2 className="mt-1 font-[family-name:var(--font-display)] text-xl font-semibold tracking-tight text-foreground">Upgrade logs</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          {isRunning
            ? 'Live installer output while the upgrade runs.'
            : streamClosed
              ? 'Log stream closed.'
              : 'Logs appear here after an upgrade starts.'}
        </p>
      </div>
      <div className="mt-4">
        <pre
          ref={logContainerRef}
          className="h-72 overflow-y-auto rounded-lg border border-border/60 bg-background/80 p-4 font-mono text-xs leading-5 text-foreground whitespace-pre-wrap break-all"
        >
          {lines.length ? lines.join('\n') : 'Waiting for upgrade output...'}
        </pre>
      </div>
    </section>
  )
}
