import { ArrowRight, Boxes, Globe, LifeBuoy, Rocket, ShieldCheck, Users } from 'lucide-react'
import { Footer } from '@/components/Footer'
import { Navbar } from '@/components/Navbar'
import { GITHUB_URL, INSTALL_COMMAND } from '@/constants'

const slides = [
  {
    kicker: '01 · Why now',
    title: 'Shipping on a VPS is common. Reliable operations are still too manual.',
    body:
      'Solo builders can ship product quickly with Docker, but upgrades, rollbacks, proxy setup, and service drift still demand constant SSH firefighting.',
  },
  {
    kicker: '02 · The gap',
    title: 'Kubernetes is powerful, but the operational tax is too high for small teams.',
    body:
      'Existing alternatives are often too opinionated or too heavy. Most teams want one clear path from Docker-on-a-server to stable production.',
  },
  {
    kicker: '03 · The solution',
    title: 'Lotsen is the lightweight control plane for single-server production.',
    body:
      'One command installs services, one dashboard manages deployments, and one reconciler keeps desired state in sync with Docker every cycle.',
  },
]

const featureCards = [
  {
    Icon: Rocket,
    title: 'Fast setup',
    text: 'Install and bootstrap in minutes using a single command path.',
  },
  {
    Icon: Boxes,
    title: 'Safer deploys',
    text: 'Zero-downtime rolling updates reduce risk during releases.',
  },
  {
    Icon: Globe,
    title: 'Built-in routing',
    text: 'Integrated proxy and HTTPS flow without manual reverse-proxy wiring.',
  },
  {
    Icon: ShieldCheck,
    title: 'Pragmatic security',
    text: 'Guided setup with hardening profiles for internet-facing hosts.',
  },
]

const communityPoints = [
  {
    Icon: Users,
    title: 'For teams like ours',
    text: 'Built for solo devs and 1-5 person teams running real workloads on VPS infrastructure.',
  },
  {
    Icon: LifeBuoy,
    title: 'Design-partner friendly',
    text: 'Early adopters can directly shape the roadmap through issues, docs feedback, and launch pilots.',
  },
]

function DeckSection({
  kicker,
  title,
  body,
}: {
  kicker: string
  title: string
  body: string
}) {
  return (
    <section
      style={{
        border: '1px solid var(--clr-line)',
        borderRadius: '16px',
        padding: '32px',
        background: 'var(--clr-surface)',
      }}
    >
      <p
        style={{
          margin: '0 0 10px 0',
          fontFamily: 'JetBrains Mono, monospace',
          fontSize: '11px',
          letterSpacing: '0.08em',
          color: 'var(--clr-accent)',
          textTransform: 'uppercase',
        }}
      >
        {kicker}
      </p>
      <h2
        style={{
          margin: '0 0 12px 0',
          fontFamily: 'Fraunces, serif',
          fontSize: 'clamp(26px, 4vw, 36px)',
          lineHeight: 1.2,
          letterSpacing: '-0.02em',
          color: 'var(--clr-navy)',
        }}
      >
        {title}
      </h2>
      <p style={{ margin: 0, color: 'var(--clr-muted)', fontSize: '16px', lineHeight: 1.7 }}>{body}</p>
    </section>
  )
}

export default function Deck() {
  return (
    <div style={{ minHeight: '100vh', background: 'var(--clr-bg)' }}>
      <Navbar />

      <main>
        <section
          style={{
            padding: '84px 32px 48px',
            borderBottom: '1px solid var(--clr-line)',
            background:
              'radial-gradient(circle at 15% 0%, rgba(26,150,224,0.08), transparent 42%), radial-gradient(circle at 85% 15%, rgba(200,80,24,0.12), transparent 40%), var(--clr-bg)',
          }}
        >
          <div style={{ maxWidth: '1100px', margin: '0 auto' }}>
            <p
              style={{
                margin: '0 0 18px 0',
                fontFamily: 'JetBrains Mono, monospace',
                fontSize: '12px',
                letterSpacing: '0.1em',
                color: 'var(--clr-accent)',
                textTransform: 'uppercase',
              }}
            >
              Lotsen pitch deck
            </p>
            <h1
              style={{
                margin: '0 0 16px 0',
                maxWidth: '840px',
                fontFamily: 'Fraunces, serif',
                fontSize: 'clamp(40px, 7vw, 64px)',
                lineHeight: 1.05,
                letterSpacing: '-0.03em',
                color: 'var(--clr-navy)',
              }}
            >
              Docker orchestration for teams that do not need Kubernetes complexity.
            </h1>
            <p
              style={{
                margin: '0 0 28px 0',
                maxWidth: '740px',
                color: 'var(--clr-muted)',
                fontSize: '18px',
                lineHeight: 1.65,
              }}
            >
              A launch-focused deck for the Lotsen community: what we are building, who it serves, and how to join as early users, contributors, and design partners.
            </p>

            <div
              style={{
                display: 'flex',
                flexWrap: 'wrap',
                gap: '12px',
                alignItems: 'center',
              }}
            >
              <a
                href="/docs/getting-started"
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '8px',
                  borderRadius: '10px',
                  background: 'var(--clr-accent)',
                  color: '#fff',
                  textDecoration: 'none',
                  fontSize: '14px',
                  fontWeight: 600,
                  padding: '11px 16px',
                }}
              >
                Read docs <ArrowRight size={15} />
              </a>
              <a
                href={GITHUB_URL}
                target="_blank"
                rel="noopener noreferrer"
                style={{
                  borderRadius: '10px',
                  border: '1px solid var(--clr-line)',
                  background: 'var(--clr-surface)',
                  color: 'var(--clr-text)',
                  textDecoration: 'none',
                  fontSize: '14px',
                  fontWeight: 600,
                  padding: '11px 16px',
                }}
              >
                View repository
              </a>
            </div>
          </div>
        </section>

        <section style={{ padding: '48px 32px' }}>
          <div style={{ maxWidth: '1100px', margin: '0 auto', display: 'grid', gap: '20px' }}>
            {slides.map((slide) => (
              <DeckSection key={slide.kicker} kicker={slide.kicker} title={slide.title} body={slide.body} />
            ))}
          </div>
        </section>

        <section style={{ padding: '24px 32px 48px' }}>
          <div style={{ maxWidth: '1100px', margin: '0 auto' }}>
            <div
              style={{
                border: '1px solid var(--clr-line)',
                borderRadius: '16px',
                padding: '24px',
                background: 'var(--clr-surface)',
              }}
            >
              <p
                style={{
                  margin: '0 0 12px 0',
                  fontFamily: 'JetBrains Mono, monospace',
                  fontSize: '11px',
                  letterSpacing: '0.08em',
                  color: 'var(--clr-accent)',
                  textTransform: 'uppercase',
                }}
              >
                04 · Architecture
              </p>
              <h2
                style={{
                  margin: '0 0 14px 0',
                  fontFamily: 'Fraunces, serif',
                  fontSize: 'clamp(26px, 4vw, 34px)',
                  lineHeight: 1.2,
                  letterSpacing: '-0.02em',
                  color: 'var(--clr-navy)',
                }}
              >
                Keep the control plane simple and inspectable.
              </h2>
              <p style={{ margin: '0 0 20px 0', color: 'var(--clr-muted)', fontSize: '16px', lineHeight: 1.7 }}>
                Dashboard writes desired state, API persists it, and the orchestrator reconciles Docker every 15 seconds. No YAML sprawl, no cluster overhead.
              </p>

              <div
                style={{
                  display: 'grid',
                  gap: '10px',
                  gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
                }}
              >
                {['Dashboard', 'API', 'JSON Store', 'Orchestrator', 'Docker'].map((step, index) => (
                  <div
                    key={step}
                    style={{
                      border: '1px solid var(--clr-line)',
                      borderRadius: '12px',
                      padding: '14px',
                      background: index % 2 === 0 ? 'var(--clr-surface-2)' : 'var(--clr-surface)',
                    }}
                  >
                    <p
                      style={{
                        margin: '0 0 5px 0',
                        fontFamily: 'JetBrains Mono, monospace',
                        fontSize: '10px',
                        letterSpacing: '0.08em',
                        color: 'var(--clr-subtle)',
                      }}
                    >
                      STEP {String(index + 1).padStart(2, '0')}
                    </p>
                    <p style={{ margin: 0, fontWeight: 600, color: 'var(--clr-text)' }}>{step}</p>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </section>

        <section style={{ padding: '24px 32px 48px' }}>
          <div style={{ maxWidth: '1100px', margin: '0 auto' }}>
            <div
              style={{
                display: 'grid',
                gap: '12px',
                gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
              }}
            >
              {featureCards.map(({ Icon, title, text }) => (
                <article
                  key={title}
                  style={{
                    border: '1px solid var(--clr-line)',
                    borderRadius: '14px',
                    padding: '18px',
                    background: 'var(--clr-surface)',
                  }}
                >
                  <div
                    style={{
                      width: '38px',
                      height: '38px',
                      borderRadius: '9px',
                      display: 'grid',
                      placeItems: 'center',
                      background: 'var(--clr-accent-dim)',
                      border: '1px solid var(--clr-accent-border)',
                      marginBottom: '10px',
                    }}
                  >
                    <Icon size={18} style={{ color: 'var(--clr-accent)' }} />
                  </div>
                  <h3
                    style={{
                      margin: '0 0 6px 0',
                      fontFamily: 'Fraunces, serif',
                      fontSize: '24px',
                      color: 'var(--clr-navy)',
                    }}
                  >
                    {title}
                  </h3>
                  <p style={{ margin: 0, color: 'var(--clr-muted)', lineHeight: 1.65 }}>{text}</p>
                </article>
              ))}
            </div>
          </div>
        </section>

        <section style={{ padding: '24px 32px 84px' }}>
          <div
            style={{
              maxWidth: '1100px',
              margin: '0 auto',
              borderRadius: '18px',
              border: '1px solid rgba(255,255,255,0.08)',
              background:
                'linear-gradient(135deg, rgba(17,29,42,1) 0%, rgba(25,38,56,1) 70%, rgba(22,44,63,1) 100%)',
              color: 'rgba(255,255,255,0.9)',
              padding: '34px',
            }}
          >
            <p
              style={{
                margin: '0 0 10px 0',
                fontFamily: 'JetBrains Mono, monospace',
                fontSize: '11px',
                letterSpacing: '0.08em',
                color: 'rgba(255,255,255,0.42)',
                textTransform: 'uppercase',
              }}
            >
              05 · Community launch
            </p>
            <h2
              style={{
                margin: '0 0 12px 0',
                fontFamily: 'Fraunces, serif',
                fontSize: 'clamp(30px, 5vw, 44px)',
                lineHeight: 1.1,
                letterSpacing: '-0.02em',
                color: '#fff',
              }}
            >
              Join as an early user and shape Lotsen in public.
            </h2>
            <p style={{ margin: '0 0 20px 0', maxWidth: '760px', color: 'rgba(255,255,255,0.6)', lineHeight: 1.75 }}>
              We are looking for real VPS workloads, candid feedback, and contributors who care about practical operations. If you run Docker in production, this is for you.
            </p>

            <div
              style={{
                display: 'grid',
                gap: '12px',
                gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))',
                marginBottom: '20px',
              }}
            >
              {communityPoints.map(({ Icon, title, text }) => (
                <article
                  key={title}
                  style={{
                    borderRadius: '12px',
                    border: '1px solid rgba(255,255,255,0.1)',
                    background: 'rgba(255,255,255,0.04)',
                    padding: '16px',
                  }}
                >
                  <div style={{ display: 'inline-flex', marginBottom: '10px' }}>
                    <Icon size={18} style={{ color: 'var(--clr-blue)' }} />
                  </div>
                  <h3 style={{ margin: '0 0 6px 0', fontSize: '20px', fontFamily: 'Fraunces, serif', color: '#fff' }}>{title}</h3>
                  <p style={{ margin: 0, color: 'rgba(255,255,255,0.62)', lineHeight: 1.65 }}>{text}</p>
                </article>
              ))}
            </div>

            <p
              style={{
                margin: 0,
                fontFamily: 'JetBrains Mono, monospace',
                fontSize: '12px',
                color: 'rgba(255,255,255,0.55)',
                lineHeight: 1.8,
                wordBreak: 'break-all',
              }}
            >
              {INSTALL_COMMAND}
            </p>
          </div>
        </section>
      </main>

      <Footer />
    </div>
  )
}
