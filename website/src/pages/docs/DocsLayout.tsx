import { NavLink, Outlet } from 'react-router-dom'
import { Navbar } from '@/components/Navbar'
import { Footer } from '@/components/Footer'

const docLinks = [
  { to: '/docs/getting-started', label: 'Getting Started' },
  { to: '/docs/deployment-configuration', label: 'Deployment Configuration' },
]

export default function DocsLayout() {
  return (
    <div style={{ backgroundColor: 'var(--clr-bg)', minHeight: '100vh', display: 'flex', flexDirection: 'column', borderTop: '1px solid var(--clr-line)' }}>
      <Navbar />

      <div
        style={{
          flex: 1,
          maxWidth: '1100px',
          margin: '0 auto',
          width: '100%',
          padding: '0 32px',
          display: 'flex',
          gap: '64px',
          alignItems: 'flex-start',
        }}
      >
        {/* Sidebar */}
        <aside
          style={{
            width: '200px',
            flexShrink: 0,
            paddingTop: '48px',
            position: 'sticky',
            top: '72px',
          }}
        >
          <p
            style={{
              fontSize: '11px',
              fontFamily: 'JetBrains Mono, monospace',
              color: 'var(--clr-muted)',
              letterSpacing: '0.1em',
              textTransform: 'uppercase',
              margin: '0 0 16px 0',
            }}
          >
            Docs
          </p>
          <nav style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            {docLinks.map(({ to, label }) => (
              <NavLink
                key={to}
                to={to}
                style={({ isActive }) => ({
                  fontSize: '14px',
                  color: isActive ? 'var(--clr-text)' : 'var(--clr-muted)',
                  textDecoration: 'none',
                  padding: '6px 10px',
                  borderRadius: '6px',
                  backgroundColor: isActive ? 'rgba(255,255,255,0.04)' : 'transparent',
                  borderLeft: isActive
                    ? '2px solid var(--clr-accent)'
                    : '2px solid transparent',
                  transition: 'color 0.15s, background-color 0.15s',
                  display: 'block',
                })}
                onMouseEnter={(e) => {
                  const el = e.currentTarget
                  if (el.style.color !== 'var(--clr-text)') {
                    el.style.color = 'var(--clr-subtle)'
                  }
                }}
                onMouseLeave={(e) => {
                  const el = e.currentTarget
                  if (el.style.color !== 'var(--clr-text)') {
                    el.style.color = 'var(--clr-muted)'
                  }
                }}
              >
                {label}
              </NavLink>
            ))}
          </nav>
        </aside>

        {/* Content */}
        <main
          className="prose"
          style={{
            flex: 1,
            minWidth: 0,
            paddingTop: '48px',
            paddingBottom: '80px',
          }}
        >
          <Outlet />
        </main>
      </div>

      <Footer />
    </div>
  )
}
