import { useState } from 'react'
import { Check, Copy } from 'lucide-react'

interface CopyButtonProps {
  text: string
  className?: string
}

export function CopyButton({ text, className = '' }: CopyButtonProps) {
  const [copied, setCopied] = useState(false)

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // Clipboard API unavailable in this context
    }
  }

  return (
    <button
      onClick={handleCopy}
      aria-label={copied ? 'Copied to clipboard' : 'Copy to clipboard'}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: '5px',
        fontSize: '12px',
        fontFamily: 'JetBrains Mono, monospace',
        color: copied ? 'var(--clr-accent)' : 'var(--clr-muted)',
        background: 'none',
        border: 'none',
        cursor: 'pointer',
        padding: '2px 4px',
        borderRadius: '4px',
        transition: 'color 0.15s',
        flexShrink: 0,
      }}
      className={className}
      onMouseEnter={(e) => {
        if (!copied) e.currentTarget.style.color = 'var(--clr-subtle)'
      }}
      onMouseLeave={(e) => {
        if (!copied) e.currentTarget.style.color = 'var(--clr-muted)'
      }}
    >
      {copied ? <Check size={13} /> : <Copy size={13} />}
      <span>{copied ? 'Copied!' : 'Copy'}</span>
    </button>
  )
}
