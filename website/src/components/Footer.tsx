import { GITHUB_URL } from '@/constants'

export function Footer() {
  return (
    <footer
      style={{
        borderTop: '1px solid var(--clr-line)',
        padding: '40px 32px',
        backgroundColor: 'var(--clr-surface)',
      }}
    >
      <div
        style={{
          maxWidth: '1100px',
          margin: '0 auto',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          flexWrap: 'wrap',
          gap: '16px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <img
            src="/mascot.png"
            alt="Lotsen"
            style={{ width: '28px', height: '28px', objectFit: 'contain', opacity: 0.7 }}
          />
          <span
            style={{
              fontSize: '13px',
              color: 'var(--clr-subtle)',
              fontFamily: 'DM Sans, sans-serif',
            }}
          >
            © {new Date().getFullYear()} Lotsen — MIT License
          </span>
        </div>
        <a
          href={GITHUB_URL}
          target="_blank"
          rel="noopener noreferrer"
          style={{
            fontSize: '13px',
            color: 'var(--clr-subtle)',
            textDecoration: 'none',
            fontFamily: 'JetBrains Mono, monospace',
            transition: 'color 0.15s',
          }}
          onMouseEnter={(e) => (e.currentTarget.style.color = 'var(--clr-muted)')}
          onMouseLeave={(e) => (e.currentTarget.style.color = 'var(--clr-subtle)')}
        >
          ercadev/lotsen ↗
        </a>
      </div>
    </footer>
  )
}
