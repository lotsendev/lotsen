import {
  Activity,
  CheckCircle2,
  LayoutDashboard,
  Network,
  RefreshCw,
  ScrollText,
  Terminal,
  XCircle,
} from 'lucide-react'
import { CopyButton } from '@/components/CopyButton'
import { Footer } from '@/components/Footer'
import { Navbar } from '@/components/Navbar'
import { GITHUB_URL, INSTALL_COMMAND } from '@/constants'

// ── Dashboard mockup ──────────────────────────────────────────────
const mockDeployments = [
  { name: 'nginx', image: 'nginx:1.27', status: 'healthy', uptime: '2 days' },
  { name: 'api', image: 'myapp/api:latest', status: 'deploying', uptime: 'just now' },
  { name: 'redis', image: 'redis:7-alpine', status: 'healthy', uptime: '5 days' },
]

function StatusBadge({ status }: { status: string }) {
  const deploying = status === 'deploying'
  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: '5px',
        fontSize: '11px',
        fontFamily: 'JetBrains Mono, monospace',
        padding: '2px 8px',
        borderRadius: '4px',
        backgroundColor: deploying ? 'rgba(251,191,36,0.12)' : 'rgba(34,197,94,0.10)',
        color: deploying ? '#d97706' : '#16a34a',
        fontWeight: 500,
      }}
    >
      <span
        style={{
          width: '5px',
          height: '5px',
          borderRadius: '50%',
          backgroundColor: deploying ? '#f59e0b' : '#22c55e',
          display: 'inline-block',
          flexShrink: 0,
          ...(deploying ? { animation: 'blink 1.1s step-end infinite' } : {}),
        }}
      />
      {status}
    </span>
  )
}

function DashboardMockup({ compact = false }: { compact?: boolean }) {
  const scale = compact ? 0.82 : 1
  return (
    <div
      style={{
        borderRadius: '12px',
        overflow: 'hidden',
        border: '1px solid var(--clr-line)',
        boxShadow: '0 24px 60px rgba(30,45,110,0.12), 0 4px 16px rgba(30,45,110,0.06)',
        transform: `scale(${scale})`,
        transformOrigin: 'top center',
        backgroundColor: '#ffffff',
      }}
    >
      {/* Browser chrome */}
      <div
        style={{
          backgroundColor: '#f0f3f9',
          padding: '10px 16px',
          borderBottom: '1px solid var(--clr-line)',
          display: 'flex',
          alignItems: 'center',
          gap: '12px',
        }}
      >
        <div style={{ display: 'flex', gap: '6px', flexShrink: 0 }}>
          {['#ff5f57', '#febc2e', '#28c840'].map((c) => (
            <div
              key={c}
              style={{ width: '10px', height: '10px', borderRadius: '50%', backgroundColor: c }}
            />
          ))}
        </div>
        <div
          style={{
            flex: 1,
            background: 'rgba(255,255,255,0.8)',
            border: '1px solid var(--clr-line)',
            borderRadius: '6px',
            padding: '3px 12px',
            fontSize: '11px',
            color: 'var(--clr-subtle)',
            fontFamily: 'JetBrains Mono, monospace',
            textAlign: 'center',
          }}
        >
          localhost:3000
        </div>
      </div>

      {/* App shell */}
      <div style={{ display: 'flex', backgroundColor: '#fafbff', minHeight: '260px' }}>
        {/* Sidebar */}
        <div
          style={{
            width: '168px',
            borderRight: '1px solid var(--clr-line)',
            padding: '16px 12px',
            flexShrink: 0,
            backgroundColor: '#ffffff',
          }}
        >
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              padding: '4px 8px',
              marginBottom: '12px',
            }}
          >
            <div
              style={{
                width: '24px',
                height: '24px',
                borderRadius: '6px',
                backgroundColor: 'var(--clr-accent)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: '12px',
              }}
            >
              🎼
            </div>
            <span
              style={{
                fontSize: '13px',
                fontFamily: 'Fraunces, serif',
                fontWeight: 700,
                color: 'var(--clr-navy)',
                letterSpacing: '-0.02em',
              }}
            >
              lotsen
            </span>
          </div>
          {[
            { label: 'Deployments', active: true },
            { label: 'System Status', active: false },
          ].map(({ label, active }) => (
            <div
              key={label}
              style={{
                fontSize: '13px',
                padding: '6px 8px',
                borderRadius: '6px',
                color: active ? 'var(--clr-navy)' : 'var(--clr-subtle)',
                backgroundColor: active ? 'var(--clr-sky-dim)' : 'transparent',
                fontWeight: active ? 500 : 400,
                marginBottom: '2px',
              }}
            >
              {label}
            </div>
          ))}
        </div>

        {/* Main content */}
        <div style={{ flex: 1, padding: '20px 24px', overflow: 'hidden' }}>
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              marginBottom: '16px',
            }}
          >
            <span
              style={{
                fontSize: '15px',
                fontWeight: 600,
                color: 'var(--clr-text)',
                fontFamily: 'Fraunces, serif',
              }}
            >
              Deployments
            </span>
            <div
              style={{
                fontSize: '12px',
                padding: '5px 14px',
                borderRadius: '7px',
                backgroundColor: 'var(--clr-accent)',
                color: '#fff',
                fontWeight: 600,
                cursor: 'default',
              }}
            >
              + Create
            </div>
          </div>

          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '12px' }}>
            <thead>
              <tr>
                {['Name', 'Image', 'Status', 'Uptime'].map((h) => (
                  <th
                    key={h}
                    style={{
                      textAlign: 'left',
                      padding: '6px 8px',
                      color: 'var(--clr-subtle)',
                      fontWeight: 500,
                      borderBottom: '1px solid var(--clr-line)',
                      fontFamily: 'JetBrains Mono, monospace',
                      fontSize: '10px',
                      textTransform: 'uppercase',
                      letterSpacing: '0.05em',
                    }}
                  >
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {mockDeployments.map((d) => (
                <tr key={d.name}>
                  <td
                    style={{
                      padding: '10px 8px',
                      color: 'var(--clr-text)',
                      fontWeight: 600,
                      fontSize: '13px',
                    }}
                  >
                    {d.name}
                  </td>
                  <td
                    style={{
                      padding: '10px 8px',
                      color: 'var(--clr-subtle)',
                      fontFamily: 'JetBrains Mono, monospace',
                      fontSize: '11px',
                    }}
                  >
                    {d.image}
                  </td>
                  <td style={{ padding: '10px 8px' }}>
                    <StatusBadge status={d.status} />
                  </td>
                  <td
                    style={{
                      padding: '10px 8px',
                      color: 'var(--clr-subtle)',
                      fontFamily: 'JetBrains Mono, monospace',
                      fontSize: '11px',
                    }}
                  >
                    {d.uptime}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}

// ── Install block ─────────────────────────────────────────────────
function InstallBlock({ dark = false }: { dark?: boolean }) {
  return (
    <div
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: '10px',
        backgroundColor: dark ? 'rgba(255,255,255,0.06)' : 'var(--clr-surface)',
        border: dark ? '1px solid rgba(255,255,255,0.1)' : '1px solid var(--clr-line)',
        borderRadius: '10px',
        padding: '13px 18px',
        maxWidth: '100%',
        minWidth: 0,
      }}
    >
      <span
        style={{
          fontFamily: 'JetBrains Mono, monospace',
          fontSize: '13px',
          color: 'var(--clr-accent)',
          flexShrink: 0,
          userSelect: 'none',
        }}
      >
        $
      </span>
      <code
        style={{
          fontFamily: 'JetBrains Mono, monospace',
          fontSize: '13px',
          color: dark ? 'rgba(255,255,255,0.6)' : 'var(--clr-muted)',
          whiteSpace: 'nowrap',
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          flex: 1,
          minWidth: 0,
        }}
      >
        {INSTALL_COMMAND}
      </code>
      <CopyButton text={INSTALL_COMMAND} />
    </div>
  )
}

// ── Comparison data ───────────────────────────────────────────────
const compareRows = [
  { label: 'Web dashboard', docker: false, lotsen: true, k8s: true },
  { label: 'Zero-downtime deploys', docker: false, lotsen: true, k8s: true },
  { label: 'Automatic restarts', docker: false, lotsen: true, k8s: true },
  { label: 'Integrated reverse proxy', docker: false, lotsen: true, k8s: true },
  { label: 'VPS-friendly (1 server)', docker: true, lotsen: true, k8s: false },
  { label: 'Simple setup', docker: true, lotsen: true, k8s: false },
  { label: 'No YAML sprawl', docker: true, lotsen: true, k8s: false },
  { label: 'No cluster required', docker: true, lotsen: true, k8s: false },
]

// ── Features ──────────────────────────────────────────────────────
const features = [
  {
    Icon: Terminal,
    title: 'One-command install',
    description:
      'A single curl command sets up three systemd services and starts everything. No manual steps.',
  },
  {
    Icon: LayoutDashboard,
    title: 'Web dashboard',
    description:
      'Deploy, edit, and remove containers from any browser. No SSH required after install.',
  },
  {
    Icon: RefreshCw,
    title: 'Zero-downtime deploys',
    description:
      'Rolling updates keep your service running during upgrades, with automatic rollback on failure.',
  },
  {
    Icon: Network,
    title: 'Integrated reverse proxy',
    description:
      'Routes HTTP traffic to containers by domain. Point DNS, set the domain field, done.',
  },
  {
    Icon: ScrollText,
    title: 'Real-time logs',
    description:
      'Stream container logs directly in the dashboard. No SSH, no docker logs commands.',
  },
  {
    Icon: Activity,
    title: 'System health',
    description:
      'Monitor API, orchestrator, Docker, and host CPU / RAM — all from one panel.',
  },
]

// ── Page ──────────────────────────────────────────────────────────
export default function Landing() {
  return (
    <div style={{ backgroundColor: 'var(--clr-bg)', minHeight: '100vh' }}>
      <Navbar />

      {/* ── Hero ──────────────────────────────────────────────── */}
      <section
        style={{
          position: 'relative',
          overflow: 'hidden',
          padding: '72px 32px 80px',
          background:
            'radial-gradient(ellipse 70% 60% at 85% 40%, rgba(212,230,248,0.5) 0%, transparent 70%), radial-gradient(ellipse 50% 50% at 10% 80%, rgba(245,146,62,0.07) 0%, transparent 70%), var(--clr-bg)',
        }}
      >
        <div
          style={{
            maxWidth: '1100px',
            margin: '0 auto',
            display: 'grid',
            gridTemplateColumns: '1fr 1fr',
            gap: '64px',
            alignItems: 'center',
          }}
        >
          {/* Left: copy */}
          <div>
            {/* Mascot + badge row */}
            <div
              className="fade-up delay-1"
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '16px',
                marginBottom: '28px',
              }}
            >
              <img
                src="/mascot.png"
                alt="Lotsen mascot — a conducting corgi"
                className="mascot-float"
                style={{
                  width: '80px',
                  height: '80px',
                  objectFit: 'contain',
                  flexShrink: 0,
                }}
              />
              <span
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '6px',
                  fontSize: '12px',
                  fontFamily: 'JetBrains Mono, monospace',
                  color: 'var(--clr-accent)',
                  backgroundColor: 'var(--clr-accent-dim)',
                  padding: '5px 12px',
                  borderRadius: '20px',
                  border: '1px solid var(--clr-accent-border)',
                  fontWeight: 500,
                }}
              >
                <span className="cursor-blink">▋</span>
                v0.1 alpha — open source
              </span>
            </div>

            {/* Headline */}
            <h1
              className="fade-up delay-2"
              style={{
                fontFamily: 'Fraunces, serif',
                fontStyle: 'italic',
                fontSize: 'clamp(38px, 5vw, 62px)',
                fontWeight: 800,
                lineHeight: 1.08,
                letterSpacing: '-0.03em',
                color: 'var(--clr-navy)',
                margin: '0 0 20px 0',
              }}
            >
              Kubernetes is overkill.
              <br />
              Bare Docker is fragile.
              <br />
              <span style={{ color: 'var(--clr-accent)' }}>Lotsen</span> is just right.
            </h1>

            {/* Sub */}
            <p
              className="fade-up delay-3"
              style={{
                fontSize: '17px',
                lineHeight: 1.65,
                color: 'var(--clr-muted)',
                maxWidth: '480px',
                margin: '0 0 36px 0',
              }}
            >
              All the orchestration you need for a VPS — web dashboard, zero-downtime
              deployments, integrated proxy — with none of the Kubernetes learning curve.
              One install command.
            </p>

            {/* Install */}
            <div className="fade-up delay-4" style={{ marginBottom: '16px' }}>
              <InstallBlock />
            </div>

            {/* GitHub link */}
            <div className="fade-up delay-5">
              <a
                href={GITHUB_URL}
                target="_blank"
                rel="noopener noreferrer"
                style={{
                  fontSize: '14px',
                  color: 'var(--clr-subtle)',
                  textDecoration: 'none',
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '5px',
                  transition: 'color 0.15s',
                }}
                onMouseEnter={(e) => (e.currentTarget.style.color = 'var(--clr-muted)')}
                onMouseLeave={(e) => (e.currentTarget.style.color = 'var(--clr-subtle)')}
              >
                View on GitHub ↗
              </a>
            </div>
          </div>

          {/* Right: app preview */}
          <div
            className="fade-up delay-3"
            style={{ position: 'relative' }}
          >
            {/* Decorative sky-blue blob behind the mockup */}
            <div
              style={{
                position: 'absolute',
                inset: '-24px',
                background:
                  'radial-gradient(ellipse at 60% 40%, rgba(212,230,248,0.7) 0%, transparent 70%)',
                borderRadius: '50%',
                pointerEvents: 'none',
                zIndex: 0,
              }}
            />
            <div style={{ position: 'relative', zIndex: 1 }}>
              <DashboardMockup />
            </div>
          </div>
        </div>
      </section>

      {/* ── "Just right" comparison ───────────────────────────── */}
      <section
        style={{
          padding: '100px 32px',
          backgroundColor: 'var(--clr-dark)',
          position: 'relative',
          overflow: 'hidden',
        }}
      >
        {/* Subtle orange glow top-right */}
        <div
          style={{
            position: 'absolute',
            top: '-80px',
            right: '-80px',
            width: '400px',
            height: '400px',
            borderRadius: '50%',
            background: 'radial-gradient(circle, rgba(245,146,62,0.15) 0%, transparent 70%)',
            pointerEvents: 'none',
          }}
        />

        <div style={{ maxWidth: '1100px', margin: '0 auto', position: 'relative' }}>
          <p
            style={{
              fontSize: '11px',
              fontFamily: 'JetBrains Mono, monospace',
              color: 'var(--clr-accent)',
              letterSpacing: '0.12em',
              textTransform: 'uppercase',
              margin: '0 0 12px 0',
            }}
          >
            The sweet spot
          </p>
          <h2
            style={{
              fontFamily: 'Fraunces, serif',
              fontSize: 'clamp(28px, 4vw, 46px)',
              fontWeight: 700,
              letterSpacing: '-0.025em',
              color: '#ffffff',
              margin: '0 0 56px 0',
              lineHeight: 1.2,
            }}
          >
            More than bare Docker.
            <br />
            Simpler than Kubernetes.
          </h2>

          <div className="compare-grid">
            {/* Bare Docker */}
            <div style={{ padding: '32px', backgroundColor: 'rgba(255,255,255,0.03)' }}>
              <div style={{ marginBottom: '24px' }}>
                <p
                  style={{
                    fontSize: '11px',
                    fontFamily: 'JetBrains Mono, monospace',
                    color: 'rgba(255,255,255,0.35)',
                    letterSpacing: '0.1em',
                    textTransform: 'uppercase',
                    margin: '0 0 8px 0',
                  }}
                >
                  Bare Docker
                </p>
                <p
                  style={{
                    fontSize: '14px',
                    color: 'rgba(255,255,255,0.4)',
                    margin: 0,
                    lineHeight: 1.5,
                  }}
                >
                  Works, but everything breaks without you.
                </p>
              </div>
              {compareRows.map((r) => (
                <div
                  key={r.label}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '10px',
                    padding: '8px 0',
                    borderTop: '1px solid rgba(255,255,255,0.06)',
                  }}
                >
                  {r.docker ? (
                    <CheckCircle2 size={15} style={{ color: 'rgba(255,255,255,0.35)', flexShrink: 0 }} />
                  ) : (
                    <XCircle size={15} style={{ color: 'rgba(255,255,255,0.2)', flexShrink: 0 }} />
                  )}
                  <span
                    style={{
                      fontSize: '13px',
                      color: r.docker ? 'rgba(255,255,255,0.5)' : 'rgba(255,255,255,0.25)',
                    }}
                  >
                    {r.label}
                  </span>
                </div>
              ))}
            </div>

            {/* Lotsen — highlighted */}
            <div
              style={{
                padding: '32px',
                backgroundColor: 'rgba(245,146,62,0.08)',
                borderLeft: '1px solid rgba(245,146,62,0.3)',
                borderRight: '1px solid rgba(245,146,62,0.3)',
                position: 'relative',
              }}
            >
              {/* Top accent bar */}
              <div
                style={{
                  position: 'absolute',
                  top: 0,
                  left: 0,
                  right: 0,
                  height: '3px',
                  backgroundColor: 'var(--clr-accent)',
                  borderRadius: '0',
                }}
              />
              <div style={{ marginBottom: '24px' }}>
                <p
                  style={{
                    fontSize: '11px',
                    fontFamily: 'JetBrains Mono, monospace',
                    color: 'var(--clr-accent)',
                    letterSpacing: '0.1em',
                    textTransform: 'uppercase',
                    margin: '0 0 8px 0',
                  }}
                >
                  Lotsen ✦
                </p>
                <p
                  style={{
                    fontSize: '14px',
                    color: 'rgba(255,255,255,0.75)',
                    margin: 0,
                    lineHeight: 1.5,
                  }}
                >
                  Production-grade orchestration that fits on one VPS.
                </p>
              </div>
              {compareRows.map((r) => (
                <div
                  key={r.label}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '10px',
                    padding: '8px 0',
                    borderTop: '1px solid rgba(245,146,62,0.12)',
                  }}
                >
                  <CheckCircle2 size={15} style={{ color: 'var(--clr-accent)', flexShrink: 0 }} />
                  <span style={{ fontSize: '13px', color: 'rgba(255,255,255,0.85)' }}>
                    {r.label}
                  </span>
                </div>
              ))}
            </div>

            {/* Kubernetes */}
            <div style={{ padding: '32px', backgroundColor: 'rgba(255,255,255,0.03)' }}>
              <div style={{ marginBottom: '24px' }}>
                <p
                  style={{
                    fontSize: '11px',
                    fontFamily: 'JetBrains Mono, monospace',
                    color: 'rgba(255,255,255,0.35)',
                    letterSpacing: '0.1em',
                    textTransform: 'uppercase',
                    margin: '0 0 8px 0',
                  }}
                >
                  Kubernetes
                </p>
                <p
                  style={{
                    fontSize: '14px',
                    color: 'rgba(255,255,255,0.4)',
                    margin: 0,
                    lineHeight: 1.5,
                  }}
                >
                  Powerful, but built for teams with dedicated DevOps.
                </p>
              </div>
              {compareRows.map((r) => (
                <div
                  key={r.label}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '10px',
                    padding: '8px 0',
                    borderTop: '1px solid rgba(255,255,255,0.06)',
                  }}
                >
                  {r.k8s ? (
                    <CheckCircle2 size={15} style={{ color: 'rgba(255,255,255,0.35)', flexShrink: 0 }} />
                  ) : (
                    <XCircle size={15} style={{ color: 'rgba(255,255,255,0.2)', flexShrink: 0 }} />
                  )}
                  <span
                    style={{
                      fontSize: '13px',
                      color: r.k8s ? 'rgba(255,255,255,0.5)' : 'rgba(255,255,255,0.25)',
                    }}
                  >
                    {r.label}
                  </span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </section>

      {/* ── Features ──────────────────────────────────────────── */}
      <section style={{ padding: '100px 32px', backgroundColor: 'var(--clr-bg)' }}>
        <div style={{ maxWidth: '1100px', margin: '0 auto' }}>
          <p
            style={{
              fontSize: '11px',
              fontFamily: 'JetBrains Mono, monospace',
              color: 'var(--clr-accent)',
              letterSpacing: '0.12em',
              textTransform: 'uppercase',
              margin: '0 0 12px 0',
            }}
          >
            What's included
          </p>
          <h2
            style={{
              fontFamily: 'Fraunces, serif',
              fontSize: 'clamp(28px, 4vw, 46px)',
              fontWeight: 700,
              letterSpacing: '-0.025em',
              color: 'var(--clr-navy)',
              margin: '0 0 56px 0',
              lineHeight: 1.2,
            }}
          >
            Everything you need.
            <br />
            Nothing you don't.
          </h2>

          <div className="feature-grid">
            {features.map(({ Icon, title, description }) => (
              <div key={title} style={{ padding: '32px' }}>
                <div
                  style={{
                    width: '40px',
                    height: '40px',
                    borderRadius: '10px',
                    backgroundColor: 'var(--clr-accent-dim)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    marginBottom: '16px',
                  }}
                >
                  <Icon size={18} style={{ color: 'var(--clr-accent)' }} />
                </div>
                <h3
                  style={{
                    fontSize: '15px',
                    fontWeight: 600,
                    color: 'var(--clr-text)',
                    margin: '0 0 8px 0',
                    letterSpacing: '-0.01em',
                  }}
                >
                  {title}
                </h3>
                <p
                  style={{
                    fontSize: '14px',
                    lineHeight: 1.65,
                    color: 'var(--clr-muted)',
                    margin: 0,
                  }}
                >
                  {description}
                </p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── CTA ───────────────────────────────────────────────── */}
      <section
        style={{
          padding: '100px 32px',
          backgroundColor: 'var(--clr-surface-2)',
          borderTop: '1px solid var(--clr-line)',
          textAlign: 'center',
        }}
      >
        <div style={{ maxWidth: '640px', margin: '0 auto' }}>
          {/* Mascot */}
          <img
            src="/mascot.png"
            alt=""
            aria-hidden
            style={{
              width: '96px',
              height: '96px',
              objectFit: 'contain',
              marginBottom: '24px',
            }}
          />

          <h2
            style={{
              fontFamily: 'Fraunces, serif',
              fontStyle: 'italic',
              fontSize: 'clamp(30px, 5vw, 52px)',
              fontWeight: 800,
              letterSpacing: '-0.03em',
              color: 'var(--clr-navy)',
              margin: '0 0 16px 0',
              lineHeight: 1.1,
            }}
          >
            Your VPS, finally orchestrated.
          </h2>
          <p
            style={{
              fontSize: '17px',
              color: 'var(--clr-muted)',
              margin: '0 0 40px 0',
              lineHeight: 1.65,
            }}
          >
            Install in under a minute.
            Supports Ubuntu 22.04+ and Debian 11+.
          </p>
          <div style={{ display: 'flex', justifyContent: 'center' }}>
            <InstallBlock />
          </div>
        </div>
      </section>

      <Footer />
    </div>
  )
}
