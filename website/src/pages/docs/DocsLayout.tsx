import { NavLink, Outlet, useLocation } from 'react-router-dom'
import { Navbar } from '@/components/Navbar'
import { Footer } from '@/components/Footer'
import gettingStartedMarkdown from '@/content/docs/getting-started.md?raw'
import deploymentConfigurationMarkdown from '@/content/docs/deployment-configuration.md?raw'
import strictModeSetupMarkdown from '@/content/docs/strict-mode-setup.md?raw'
import productionReadinessMarkdown from '@/content/docs/production-readiness.md?raw'

function toSlug(text: string): string {
  return text.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, '')
}

function extractH2s(markdown: string): string[] {
  return markdown
    .split('\n')
    .filter((line) => line.startsWith('## '))
    .map((line) => line.slice(3))
}

const docLinks = [
  { to: '/docs/getting-started', label: 'Getting Started', markdown: gettingStartedMarkdown },
  {
    to: '/docs/deployment-configuration',
    label: 'Deployment Configuration',
    markdown: deploymentConfigurationMarkdown,
  },
  { to: '/docs/strict-mode-setup', label: 'Strict Mode Setup', markdown: strictModeSetupMarkdown },
  {
    to: '/docs/production-readiness',
    label: 'Production Readiness',
    markdown: productionReadinessMarkdown,
  },
]

export default function DocsLayout() {
  const { pathname } = useLocation()

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
            {docLinks.map(({ to, label, markdown }) => {
              const isActive = pathname === to
              const sections = isActive ? extractH2s(markdown) : []
              return (
                <div key={to}>
                  <NavLink
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
                  {sections.length > 0 && (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '2px', marginTop: '2px', marginBottom: '4px' }}>
                      {sections.map((heading) => (
                        <a
                          key={heading}
                          href={`#${toSlug(heading)}`}
                          style={{
                            fontSize: '12px',
                            color: 'var(--clr-muted)',
                            textDecoration: 'none',
                            padding: '3px 10px 3px 18px',
                            borderRadius: '4px',
                            display: 'block',
                            transition: 'color 0.15s',
                            lineHeight: '1.4',
                          }}
                          onMouseEnter={(e) => {
                            e.currentTarget.style.color = 'var(--clr-subtle)'
                          }}
                          onMouseLeave={(e) => {
                            e.currentTarget.style.color = 'var(--clr-muted)'
                          }}
                        >
                          {heading}
                        </a>
                      ))}
                    </div>
                  )}
                </div>
              )
            })}
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
