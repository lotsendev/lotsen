import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { GITHUB_URL } from '@/constants'

export function Navbar() {
  const [scrolled, setScrolled] = useState(false)

  useEffect(() => {
    function onScroll() {
      setScrolled(window.scrollY > 24)
    }
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  return (
    <header
      style={{
        position: 'sticky',
        top: 0,
        zIndex: 50,
        borderBottom: scrolled ? '1px solid var(--clr-line)' : '1px solid transparent',
        backgroundColor: scrolled ? 'rgba(248,250,255,0.92)' : 'transparent',
        backdropFilter: scrolled ? 'blur(16px)' : 'none',
        WebkitBackdropFilter: scrolled ? 'blur(16px)' : 'none',
        transition: 'background-color 0.2s ease, border-color 0.2s ease',
      }}
    >
      <div
        style={{
          maxWidth: '1100px',
          margin: '0 auto',
          padding: '0 32px',
          height: '60px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
        }}
      >
        <Link
          to="/"
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            textDecoration: 'none',
          }}
        >
          <img
            src="/mascot.png"
            alt="Lotsen mascot"
            style={{ width: '32px', height: '32px', objectFit: 'contain' }}
          />
          <span
            style={{
              fontFamily: 'Fraunces, serif',
              fontSize: '19px',
              fontWeight: 700,
              color: 'var(--clr-navy)',
              letterSpacing: '-0.03em',
            }}
          >
            lotsen
          </span>
        </Link>

        <nav style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <Link
            to="/docs/getting-started"
            style={{
              fontSize: '14px',
              color: 'var(--clr-muted)',
              textDecoration: 'none',
              padding: '6px 12px',
              borderRadius: '8px',
              transition: 'color 0.15s, background-color 0.15s',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.color = 'var(--clr-text)'
              e.currentTarget.style.backgroundColor = 'var(--clr-surface-2)'
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.color = 'var(--clr-muted)'
              e.currentTarget.style.backgroundColor = 'transparent'
            }}
          >
            Docs
          </Link>
          <a
            href={GITHUB_URL}
            target="_blank"
            rel="noopener noreferrer"
            style={{
              fontSize: '14px',
              color: 'var(--clr-muted)',
              textDecoration: 'none',
              padding: '6px 12px',
              borderRadius: '8px',
              transition: 'color 0.15s, background-color 0.15s',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.color = 'var(--clr-text)'
              e.currentTarget.style.backgroundColor = 'var(--clr-surface-2)'
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.color = 'var(--clr-muted)'
              e.currentTarget.style.backgroundColor = 'transparent'
            }}
          >
            GitHub ↗
          </a>
          <Link
            to="/docs/getting-started"
            style={{
              fontSize: '14px',
              fontWeight: 600,
              color: '#fff',
              textDecoration: 'none',
              padding: '7px 16px',
              borderRadius: '8px',
              backgroundColor: 'var(--clr-accent)',
              transition: 'opacity 0.15s',
            }}
            onMouseEnter={(e) => (e.currentTarget.style.opacity = '0.88')}
            onMouseLeave={(e) => (e.currentTarget.style.opacity = '1')}
          >
            Get started
          </Link>
        </nav>
      </div>
    </header>
  )
}
