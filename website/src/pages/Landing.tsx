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

// ── Nautical chart grid overlay ───────────────────────────────────
function ChartGrid({ opacity = 0.055, size = 48, dark = false }: {
  opacity?: number
  size?: number
  dark?: boolean
}) {
  const rgb = dark ? '255,255,255' : '26,58,82'
  return (
    <div
      style={{
        position: 'absolute',
        inset: 0,
        backgroundImage: `
          linear-gradient(rgba(${rgb},${opacity}) 1px, transparent 1px),
          linear-gradient(90deg, rgba(${rgb},${opacity}) 1px, transparent 1px)
        `,
        backgroundSize: `${size}px ${size}px`,
        pointerEvents: 'none',
      }}
    />
  )
}

// ── Wave divider ──────────────────────────────────────────────────
function WaveDivider({ fill, bg }: { fill: string; bg: string }) {
  return (
    <div style={{ lineHeight: 0, backgroundColor: bg }}>
      <svg viewBox="0 0 1440 48" preserveAspectRatio="none" style={{ display: 'block', width: '100%', height: '48px' }} aria-hidden>
        <path
          d="M0,20 C180,38 360,4 540,22 C720,40 900,6 1080,24 C1260,40 1380,14 1440,20 L1440,48 L0,48 Z"
          fill={fill}
        />
      </svg>
    </div>
  )
}

// ── Cargo ship SVG ────────────────────────────────────────────────
function CargoShipSVG({ opacity = 1 }: { opacity?: number }) {
  const o = (v: number) => v * opacity
  const containers: Array<{ x: number; colors: string[]; }> = [
    { x: 70,  colors: ['#2a7a64', '#c85018', '#687d90'] },
    { x: 138, colors: ['#c85018', '#b8c4cc', '#2a7a64'] },
    { x: 206, colors: ['#687d90', '#c85018'            ] },
    { x: 270, colors: ['#2a7a64', '#687d90', '#c85018' ] },
    { x: 338, colors: ['#c85018', '#2a7a64'            ] },
    { x: 402, colors: ['#b8c4cc', '#687d90', '#c85018' ] },
    { x: 470, colors: ['#2a7a64', '#c85018', '#b8c4cc' ] },
    { x: 538, colors: ['#c85018', '#2a7a64'            ] },
    { x: 602, colors: ['#687d90', '#c85018', '#2a7a64' ] },
    { x: 670, colors: ['#b8c4cc', '#c85018'            ] },
  ]
  const cH = 20   // container height
  const cW = 58   // container width
  const deckY = 90

  return (
    <svg viewBox="0 0 900 145" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden style={{ width: '100%' }}>
      {/* Water shimmer */}
      <path d="M0,112 Q180,108 360,114 Q540,120 720,112 Q900,106 900,112 L900,145 L0,145 Z" fill={`rgba(26,150,224,${o(0.07)})`} />
      <path d="M0,116 Q90,113 180,116 Q270,119 360,116 Q450,113 540,116" stroke={`rgba(26,150,224,${o(0.12)})`} strokeWidth="1" fill="none" />
      <path d="M400,118 Q490,115 580,118 Q670,121 760,118 Q850,115 900,118" stroke={`rgba(26,150,224,${o(0.09)})`} strokeWidth="1" fill="none" />

      {/* Hull */}
      <path d="M48,90 L828,90 L856,110 L28,110 Z" fill={`rgba(17,29,42,${o(0.82)})`} />
      {/* Waterline rust stripe */}
      <rect x="28" y="106" width="830" height="5" fill={`rgba(200,80,24,${o(0.55)})`} />
      {/* Bow tip */}
      <path d="M828,90 L864,100 L856,110 Z" fill={`rgba(17,29,42,${o(0.65)})`} />
      {/* Stern detail */}
      <rect x="26" y="95" width="22" height="15" fill={`rgba(17,29,42,${o(0.5)})`} rx="1" />

      {/* Container stacks */}
      {containers.map(({ x, colors }, gi) => (
        <g key={gi}>
          {colors.map((color, ci) => {
            const y = deckY - (colors.length - ci) * cH
            return (
              <g key={ci}>
                <rect x={x} y={y} width={cW} height={cH - 1} fill={color} opacity={o(0.68)} rx="1" />
                <line x1={x + 19} y1={y} x2={x + 19} y2={y + cH} stroke="rgba(0,0,0,0.18)" strokeWidth="1" />
                <line x1={x + 39} y1={y} x2={x + 39} y2={y + cH} stroke="rgba(0,0,0,0.18)" strokeWidth="1" />
                <line x1={x}      y1={y + cH - 1} x2={x + cW} y2={y + cH - 1} stroke="rgba(0,0,0,0.12)" strokeWidth="1" />
              </g>
            )
          })}
        </g>
      ))}

      {/* Crane boom */}
      <rect x="448" y="12" width="3"   height="78" fill={`rgba(17,29,42,${o(0.5)})`} />
      <line x1="358" y1="14" x2="450" y2="14" stroke={`rgba(17,29,42,${o(0.42)})`} strokeWidth="2.5" />
      <line x1="450" y1="14" x2="508" y2="14" stroke={`rgba(17,29,42,${o(0.35)})`} strokeWidth="2.5" />
      <line x1="375" y1="16" x2="393" y2="68" stroke={`rgba(17,29,42,${o(0.28)})`} strokeWidth="1" />
      <rect x="504" y="10" width="10"  height="7"  fill={`rgba(17,29,42,${o(0.4)})`} rx="1" />

      {/* Bridge superstructure */}
      <rect x="748" y="52" width="82" height="38" fill={`rgba(17,29,42,${o(0.88)})`} rx="1" />
      <rect x="754" y="35" width="68" height="17" fill={`rgba(17,29,42,${o(0.82)})`} rx="1" />
      <rect x="760" y="24" width="52" height="11" fill={`rgba(17,29,42,${o(0.76)})`} rx="1" />

      {/* Bridge windows — lower deck */}
      {[0, 1, 2, 3].map((i) => (
        <rect key={i} x={752 + i * 18} y={57} width={13} height={10} fill={`rgba(26,150,224,${o(0.42)})`} rx="1" />
      ))}
      {/* Bridge windows — upper deck */}
      {[0, 1, 2].map((i) => (
        <rect key={i} x={757 + i * 18} y={39} width={11} height={8}  fill={`rgba(26,150,224,${o(0.32)})`} rx="1" />
      ))}

      {/* Nav mast */}
      <rect x="782" y="7"  width="2.5" height="24" fill={`rgba(17,29,42,${o(0.6)})`} />
      <line x1="774" y1="9" x2="792" y2="9" stroke={`rgba(17,29,42,${o(0.5)})`} strokeWidth="1.5" />
      <circle cx="783" cy="7" r="2" fill={`rgba(200,80,24,${o(0.6)})`} />

      {/* Funnel */}
      <rect x="756" y="39" width="15" height="13" fill={`rgba(25,38,56,${o(0.95)})`} rx="1" />
      <rect x="757" y="36" width="13" height="5"  fill={`rgba(200,80,24,${o(0.5)})`}  rx="1" />

      {/* Anchor chain */}
      <path d="M42,110 Q38,114 34,110 Q30,106 26,110" stroke={`rgba(184,196,204,${o(0.35)})`} strokeWidth="1.5" fill="none" strokeDasharray="3 2" />
    </svg>
  )
}

// ── Lighthouse SVG ────────────────────────────────────────────────
function LighthouseSVG({ height = 200 }: { height?: number }) {
  return (
    <svg viewBox="0 0 90 220" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden style={{ height, width: 'auto' }}>
      {/* Light beam */}
      <path d="M56,72 L110,35 L115,65 Z" fill="rgba(26,150,224,0.07)" />
      <path d="M56,72 L115,50 Q118,62 118,75 L56,78 Z" fill="rgba(26,150,224,0.04)" />

      {/* Rocky base */}
      <path d="M5,220 Q15,202 32,206 Q44,198 58,204 Q70,198 80,206 Q88,202 90,220 Z" fill="rgba(26,52,82,0.55)" />
      <path d="M8,215 Q18,205 34,208 Q46,202 60,206 Q72,200 82,208" stroke="rgba(184,196,204,0.25)" strokeWidth="1" fill="none" />

      {/* Tower platform / base */}
      <rect x="20" y="195" width="44" height="8" fill="rgba(26,52,82,0.7)" rx="1" />

      {/* Tower body — tapered */}
      <path d="M24,195 L29,88 L55,88 L60,195 Z" fill="rgba(26,52,82,0.68)" />

      {/* Stripe bands (alternating white) */}
      <path d="M25.5,175 L26.5,158 L57.5,158 L58.5,175 Z" fill="rgba(255,255,255,0.1)" />
      <path d="M27,148 L28,131 L56,131 L57,148 Z"         fill="rgba(255,255,255,0.1)" />
      <path d="M28.5,121 L29.5,104 L54.5,104 L55.5,121 Z" fill="rgba(255,255,255,0.1)" />

      {/* Small windows */}
      <rect x="38" y="165" width="8" height="9" fill="rgba(26,150,224,0.22)" rx="1" />
      <rect x="38" y="138" width="8" height="9" fill="rgba(26,150,224,0.22)" rx="1" />
      <rect x="38" y="111" width="8" height="9" fill="rgba(26,150,224,0.22)" rx="1" />

      {/* Door */}
      <rect x="37" y="182" width="10" height="13" fill="rgba(17,29,42,0.55)" rx="1" />

      {/* Gallery railing */}
      <rect x="22" y="84" width="40" height="4" fill="rgba(104,125,144,0.7)" rx="1" />
      <line x1="22" y1="84" x2="62" y2="84" stroke="rgba(184,196,204,0.4)" strokeWidth="1" />
      {[0, 1, 2, 3, 4, 5].map((i) => (
        <line key={i} x1={24 + i * 7} y1="84" x2={24 + i * 7} y2="88" stroke="rgba(184,196,204,0.3)" strokeWidth="1" />
      ))}

      {/* Lantern room */}
      <rect x="25" y="62" width="34" height="22" fill="rgba(17,29,42,0.92)" rx="1" />
      {/* Lantern glass */}
      <rect x="27" y="64" width="12" height="16" fill="rgba(26,150,224,0.48)" rx="1" />
      <rect x="44" y="64" width="12" height="16" fill="rgba(26,150,224,0.52)" rx="1" />
      {/* Lantern inner glow */}
      <rect x="30" y="67" width="6"  height="10" fill="rgba(26,150,224,0.18)" rx="1" />
      <rect x="47" y="67" width="6"  height="10" fill="rgba(26,150,224,0.22)" rx="1" />

      {/* Dome / cap */}
      <path d="M25,62 Q42,48 59,62 Z" fill="rgba(200,80,24,0.6)" />
      {/* Ventilation ball */}
      <circle cx="42" cy="49" r="3" fill="rgba(17,29,42,0.7)" />
      {/* Flagpole */}
      <rect x="41" y="34" width="2" height="16" fill="rgba(17,29,42,0.55)" />
      <path d="M43,34 L52,38 L43,42 Z" fill="rgba(200,80,24,0.45)" />
    </svg>
  )
}

// ── Heavy sea ─────────────────────────────────────────────────────
function HeavySea() {
  // Period must divide evenly into 1440 for seamless loop.
  // Back: period=480 (3 cycles), middle: period=360 (4 cycles), front: period=288 (5 cycles)
  const back = [
    // M0,8 then 6 full cycles to x=2880 (period=480, half=240, ctrl≈88)
    'M0,8',
    'C88,8 152,118 240,118 C328,118 392,8 480,8',
    'C568,8 632,118 720,118 C808,118 872,8 960,8',
    'C1048,8 1112,118 1200,118 C1288,118 1352,8 1440,8',
    'C1528,8 1592,118 1680,118 C1768,118 1832,8 1920,8',
    'C2008,8 2072,118 2160,118 C2248,118 2312,8 2400,8',
    'C2488,8 2552,118 2640,118 C2728,118 2792,8 2880,8',
    'L2880,150 L0,150 Z',
  ].join(' ')

  const mid = [
    // period=360, half=180, ctrl≈66, peak=20, trough=92
    'M0,20',
    'C66,20 114,92 180,92 C246,92 294,20 360,20',
    'C426,20 474,92 540,92 C606,92 654,20 720,20',
    'C786,20 834,92 900,92 C966,92 1014,20 1080,20',
    'C1146,20 1194,92 1260,92 C1326,92 1374,20 1440,20',
    'C1506,20 1554,92 1620,92 C1686,92 1734,20 1800,20',
    'C1866,20 1914,92 1980,92 C2046,92 2094,20 2160,20',
    'C2226,20 2274,92 2340,92 C2406,92 2454,20 2520,20',
    'C2586,20 2634,92 2700,92 C2766,92 2814,20 2880,20',
    'L2880,112 L0,112 Z',
  ].join(' ')

  const front = [
    // period=288, half=144, ctrl≈53, peak=16, trough=62
    'M0,16',
    'C53,16 91,62 144,62 C197,62 235,16 288,16',
    'C341,16 379,62 432,62 C485,62 523,16 576,16',
    'C629,16 667,62 720,62 C773,62 811,16 864,16',
    'C917,16 955,62 1008,62 C1061,62 1099,16 1152,16',
    'C1205,16 1243,62 1296,62 C1349,62 1387,16 1440,16',
    'C1493,16 1531,62 1584,62 C1637,62 1675,16 1728,16',
    'C1781,16 1819,62 1872,62 C1925,62 1963,16 2016,16',
    'C2069,16 2107,62 2160,62 C2213,62 2251,16 2304,16',
    'C2357,16 2395,62 2448,62 C2501,62 2539,16 2592,16',
    'C2645,16 2683,62 2736,62 C2789,62 2827,16 2880,16',
    'L2880,78 L0,78 Z',
  ].join(' ')

  // White cap positions aligned to front wave crests (every 288px)
  const capXs = [0, 288, 576, 864, 1152, 1440, 1728, 2016, 2304, 2592, 2880]

  return (
    <div
      style={{
        position: 'relative',
        height: '190px',
        backgroundColor: 'var(--clr-bg)',
        overflow: 'hidden',
      }}
    >
      {/* Horizon glow — bright blue where sky meets sea */}
      <div
        style={{
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          height: '90px',
          background: 'linear-gradient(to bottom, rgba(26,150,224,0.06) 0%, transparent 100%)',
          pointerEvents: 'none',
        }}
      />

      {/* Back wave — lightest opacity, slowest, largest amplitude */}
      <div
        style={{
          position: 'absolute',
          bottom: 0,
          left: 0,
          width: '200%',
          height: '150px',
          animation: 'waveScroll 55s linear infinite',
        }}
      >
        <svg viewBox="0 0 2880 150" preserveAspectRatio="none" style={{ width: '100%', height: '100%' }} aria-hidden>
          <path d={back} fill="rgba(17,29,42,0.52)" />
        </svg>
      </div>

      {/* Middle wave — medium, reverse direction */}
      <div
        style={{
          position: 'absolute',
          bottom: 0,
          left: 0,
          width: '200%',
          height: '112px',
          animation: 'waveScroll 38s linear infinite reverse',
        }}
      >
        <svg viewBox="0 0 2880 112" preserveAspectRatio="none" style={{ width: '100%', height: '100%' }} aria-hidden>
          <path d={mid} fill="rgba(17,29,42,0.74)" />
        </svg>
      </div>

      {/* Front wave — darkest, fastest, nearly solid */}
      <div
        style={{
          position: 'absolute',
          bottom: 0,
          left: 0,
          width: '200%',
          height: '78px',
          animation: 'waveScroll 24s linear infinite',
        }}
      >
        <svg viewBox="0 0 2880 78" preserveAspectRatio="none" style={{ width: '100%', height: '100%' }} aria-hidden>
          <path d={front} fill="#111d2a" />
        </svg>
      </div>

      {/* White caps — sync with front wave speed */}
      <div
        style={{
          position: 'absolute',
          bottom: 60,
          left: 0,
          width: '200%',
          animation: 'waveScroll 24s linear infinite',
          pointerEvents: 'none',
        }}
      >
        <svg viewBox="0 0 2880 20" style={{ width: '100%', height: '20px', display: 'block' }} aria-hidden>
          {capXs.map((cx, i) => (
            <g key={i}>
              <path
                d={`M${cx + 28},10 Q${cx + 46},2 ${cx + 64},10`}
                stroke="rgba(255,255,255,0.28)"
                strokeWidth="1.5"
                fill="none"
              />
              <path
                d={`M${cx + 110},12 Q${cx + 132},3 ${cx + 154},12`}
                stroke="rgba(255,255,255,0.18)"
                strokeWidth="1"
                fill="none"
              />
            </g>
          ))}
        </svg>
      </div>
    </div>
  )
}

// ── Harbor scene ──────────────────────────────────────────────────
function HarborScene() {
  return (
    <section
      style={{
        position: 'relative',
        backgroundColor: '#0a1520',
        overflow: 'hidden',
        padding: '0',
      }}
    >
      {/* Atmospheric fog gradient */}
      <div style={{ position: 'absolute', inset: 0, background: 'radial-gradient(ellipse 80% 60% at 50% 40%, rgba(26,150,224,0.05) 0%, transparent 70%), radial-gradient(ellipse 40% 80% at 90% 50%, rgba(42,122,100,0.06) 0%, transparent 60%)', pointerEvents: 'none' }} />

      <div style={{ maxWidth: '1100px', margin: '0 auto', padding: '48px 32px 0', position: 'relative' }}>
        {/* Chart header */}
        <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '12px' }}>
          <span style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: '10px', letterSpacing: '0.08em', color: 'rgba(255,255,255,0.15)' }}>
            VESSEL IN TRANSIT · LOTSEN-001
          </span>
          <span style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: '10px', letterSpacing: '0.08em', color: 'rgba(255,255,255,0.15)' }}>
            63°24'N · 10°22'E
          </span>
        </div>
      </div>

      {/* Full-width ship + lighthouse scene */}
      <div style={{ position: 'relative', display: 'flex', alignItems: 'flex-end', justifyContent: 'center', padding: '0 24px', gap: '0', overflow: 'hidden' }}>
        <div style={{ flex: 1, maxWidth: '940px' }}>
          <CargoShipSVG opacity={0.82} />
        </div>
        <div style={{ flexShrink: 0, marginBottom: '28px', marginLeft: '-16px' }}>
          <LighthouseSVG height={160} />
        </div>
      </div>

      {/* Water base */}
      <div style={{ height: '32px', background: 'linear-gradient(to bottom, rgba(26,150,224,0.06), rgba(26,150,224,0.02))', borderBottom: '1px solid rgba(26,150,224,0.08)' }} />
    </section>
  )
}

// ── Status badge ──────────────────────────────────────────────────
const mockDeployments = [
  { name: 'nginx',  image: 'nginx:1.27',        status: 'healthy',   uptime: '2d 4h'  },
  { name: 'api',    image: 'myapp/api:latest',   status: 'deploying', uptime: 'just now' },
  { name: 'redis',  image: 'redis:7-alpine',     status: 'healthy',   uptime: '5d 12h' },
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

// ── Dashboard mockup ──────────────────────────────────────────────
function DashboardMockup() {
  return (
    <div
      style={{
        borderRadius: '12px',
        overflow: 'hidden',
        border: '1px solid rgba(26,58,82,0.12)',
        boxShadow: '0 20px 60px rgba(26,52,82,0.14), 0 4px 16px rgba(26,52,82,0.07)',
        backgroundColor: '#ffffff',
      }}
    >
      {/* Browser chrome */}
      <div
        style={{
          backgroundColor: '#f0f4f7',
          padding: '10px 16px',
          borderBottom: '1px solid rgba(26,58,82,0.08)',
          display: 'flex',
          alignItems: 'center',
          gap: '12px',
        }}
      >
        <div style={{ display: 'flex', gap: '6px', flexShrink: 0 }}>
          {['#ff5f57', '#febc2e', '#28c840'].map((c) => (
            <div key={c} style={{ width: '10px', height: '10px', borderRadius: '50%', backgroundColor: c }} />
          ))}
        </div>
        <div
          style={{
            flex: 1,
            background: 'rgba(255,255,255,0.8)',
            border: '1px solid rgba(26,58,82,0.1)',
            borderRadius: '6px',
            padding: '3px 12px',
            fontSize: '11px',
            color: '#7e9aaa',
            fontFamily: 'JetBrains Mono, monospace',
            textAlign: 'center',
          }}
        >
          lotsen.local:3000
        </div>
      </div>

      {/* App shell */}
      <div style={{ display: 'flex', backgroundColor: '#f8fafc', minHeight: '240px' }}>
        {/* Sidebar */}
        <div
          style={{
            width: '160px',
            borderRight: '1px solid rgba(26,58,82,0.08)',
            padding: '14px 10px',
            flexShrink: 0,
            backgroundColor: '#ffffff',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '4px 8px', marginBottom: '10px' }}>
            <div
              style={{
                width: '22px',
                height: '22px',
                borderRadius: '5px',
                backgroundColor: '#c85018',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: '11px',
              }}
            >
              ⚓
            </div>
            <span style={{ fontSize: '13px', fontFamily: 'Fraunces, serif', fontWeight: 700, color: '#1a3449', letterSpacing: '-0.02em' }}>
              lotsen
            </span>
          </div>

          {[
            { label: 'Deployments', active: true },
            { label: 'System Status', active: false },
            { label: 'Proxy Routes', active: false },
            { label: 'Logs', active: false },
          ].map(({ label, active }) => (
            <div
              key={label}
              style={{
                fontSize: '12px',
                padding: '5px 8px',
                borderRadius: '5px',
                color: active ? '#1a3449' : '#7e9aaa',
                backgroundColor: active ? 'rgba(104,125,144,0.1)' : 'transparent',
                fontWeight: active ? 500 : 400,
                marginBottom: '1px',
              }}
            >
              {label}
            </div>
          ))}

          <div style={{ marginTop: '20px', padding: '8px', borderRadius: '6px', backgroundColor: 'rgba(42,122,100,0.08)', border: '1px solid rgba(42,122,100,0.15)' }}>
            <div style={{ fontSize: '9px', fontFamily: 'JetBrains Mono, monospace', color: '#2a7a64', letterSpacing: '0.07em', marginBottom: '3px' }}>HEALTH</div>
            <div style={{ fontSize: '11px', color: '#2a7a64', fontWeight: 500 }}>● All nominal</div>
          </div>
        </div>

        {/* Main content */}
        <div style={{ flex: 1, padding: '16px 20px', overflow: 'hidden' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '14px' }}>
            <span style={{ fontSize: '14px', fontWeight: 600, color: '#1a3449', fontFamily: 'Fraunces, serif' }}>
              Deployments
            </span>
            <div style={{ fontSize: '11px', padding: '4px 12px', borderRadius: '6px', backgroundColor: '#c85018', color: '#fff', fontWeight: 600, cursor: 'default' }}>
              + Deploy
            </div>
          </div>

          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '11px' }}>
            <thead>
              <tr>
                {['Name', 'Image', 'Status', 'Uptime'].map((h) => (
                  <th
                    key={h}
                    style={{
                      textAlign: 'left',
                      padding: '5px 8px',
                      color: '#b8c4cc',
                      fontWeight: 500,
                      borderBottom: '1px solid rgba(26,58,82,0.08)',
                      fontFamily: 'JetBrains Mono, monospace',
                      fontSize: '9px',
                      textTransform: 'uppercase',
                      letterSpacing: '0.06em',
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
                  <td style={{ padding: '9px 8px', color: '#1a3449', fontWeight: 600, fontSize: '12px' }}>{d.name}</td>
                  <td style={{ padding: '9px 8px', color: '#7e9aaa', fontFamily: 'JetBrains Mono, monospace', fontSize: '10px' }}>{d.image}</td>
                  <td style={{ padding: '9px 8px' }}><StatusBadge status={d.status} /></td>
                  <td style={{ padding: '9px 8px', color: '#7e9aaa', fontFamily: 'JetBrains Mono, monospace', fontSize: '10px' }}>{d.uptime}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}

// ── Log stream mockup ─────────────────────────────────────────────
const logEntries = [
  { time: '08:42:09', svc: 'proxy',  msg: 'TLS cert renewed · api.dev',         ok: true  },
  { time: '08:42:11', svc: 'nginx',  msg: 'health check passed',                 ok: true  },
  { time: '08:42:13', svc: 'api',    msg: 'rolling deploy v2.1.4 — 0ms downtime', ok: true  },
  { time: '08:42:17', svc: 'redis',  msg: 'restart recovered in 1.2s',            ok: null  },
  { time: '08:42:21', svc: 'nginx',  msg: 'health check passed',                 ok: true  },
]

function LogMockup() {
  return (
    <div
      style={{
        borderRadius: '10px',
        overflow: 'hidden',
        border: '1px solid rgba(26,58,82,0.12)',
        boxShadow: '0 8px 30px rgba(26,52,82,0.1)',
        backgroundColor: '#111d2a',
      }}
    >
      <div
        style={{
          backgroundColor: '#192638',
          padding: '8px 14px',
          borderBottom: '1px solid rgba(255,255,255,0.05)',
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
        }}
      >
        <Activity size={11} style={{ color: '#2a7a64' }} />
        <span style={{ fontSize: '10px', fontFamily: 'JetBrains Mono, monospace', letterSpacing: '0.08em', color: 'rgba(255,255,255,0.3)' }}>
          SYSTEM LOG — LIVE
        </span>
        <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '5px' }}>
          <div style={{ width: '5px', height: '5px', borderRadius: '50%', backgroundColor: '#2a7a64', animation: 'blink 2s step-end infinite' }} />
          <span style={{ fontSize: '9px', fontFamily: 'JetBrains Mono, monospace', color: '#2a7a64', letterSpacing: '0.06em' }}>STREAMING</span>
        </div>
      </div>
      <div style={{ padding: '8px 0' }}>
        {logEntries.map((entry, i) => (
          <div
            key={i}
            style={{
              padding: '3px 14px',
              display: 'flex',
              gap: '10px',
              alignItems: 'baseline',
              fontSize: '11px',
              fontFamily: 'JetBrains Mono, monospace',
            }}
          >
            <span style={{ color: 'rgba(255,255,255,0.18)', flexShrink: 0 }}>{entry.time}</span>
            <span style={{ color: entry.ok === true ? '#2a7a64' : entry.ok === false ? '#c85018' : '#b8860b', width: '38px', flexShrink: 0 }}>
              {entry.svc}
            </span>
            <span style={{ color: 'rgba(255,255,255,0.42)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {entry.msg}
            </span>
          </div>
        ))}
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
        backgroundColor: dark ? 'rgba(255,255,255,0.05)' : 'var(--clr-surface)',
        border: dark ? '1px solid rgba(255,255,255,0.1)' : '1px solid var(--clr-line)',
        borderRadius: '10px',
        padding: '13px 18px',
        maxWidth: '100%',
        minWidth: 0,
      }}
    >
      <span style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: '13px', color: 'var(--clr-accent)', flexShrink: 0, userSelect: 'none' }}>$</span>
      <code style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: '13px', color: dark ? 'rgba(255,255,255,0.55)' : 'var(--clr-muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', flex: 1, minWidth: 0 }}>
        {INSTALL_COMMAND}
      </code>
      <CopyButton text={INSTALL_COMMAND} />
    </div>
  )
}

// ── Instrument panel ──────────────────────────────────────────────
const instruments = [
  { label: 'BEARING',    value: 'ROLLING DEPLOY', accent: false },
  { label: 'CONTAINERS', value: '87',              accent: false },
  { label: 'DOWNTIME',   value: '0 ms',            accent: true  },
  { label: 'INSTALL',    value: '1 CMD',           accent: false },
  { label: 'PROXY',      value: 'ACTIVE',          accent: false },
  { label: 'SIGNAL',     value: 'NOMINAL',         blue: true },
]

function InstrumentBar() {
  return (
    <div
      style={{
        backgroundColor: 'var(--clr-dark)',
        borderTop: '1px solid rgba(255,255,255,0.04)',
        borderBottom: '1px solid rgba(255,255,255,0.04)',
        overflowX: 'auto',
      }}
    >
      <div
        style={{
          maxWidth: '1100px',
          margin: '0 auto',
          padding: '0 32px',
          display: 'flex',
          alignItems: 'stretch',
        }}
      >
        {/* Chart designation — left anchor */}
        <div
          style={{
            padding: '20px 28px 20px 0',
            borderRight: '1px solid rgba(255,255,255,0.05)',
            flexShrink: 0,
            display: 'flex',
            flexDirection: 'column',
            justifyContent: 'center',
            gap: '3px',
            marginRight: '4px',
          }}
        >
          <div style={{ fontSize: '9px', fontFamily: 'JetBrains Mono, monospace', letterSpacing: '0.12em', color: 'rgba(255,255,255,0.18)', textTransform: 'uppercase' }}>
            CHART · LOTSEN-001
          </div>
          <div style={{ fontSize: '10px', fontFamily: 'JetBrains Mono, monospace', color: 'rgba(255,255,255,0.22)', letterSpacing: '0.06em' }}>
            63°24'N · 10°22'E
          </div>
        </div>

        {instruments.map((inst, i) => (
          <div
            key={i}
            style={{
              padding: '20px 24px',
              borderRight: i < instruments.length - 1 ? '1px solid rgba(255,255,255,0.05)' : 'none',
              flexShrink: 0,
            }}
          >
            <div style={{ fontSize: '9px', fontFamily: 'JetBrains Mono, monospace', letterSpacing: '0.12em', color: 'rgba(255,255,255,0.18)', textTransform: 'uppercase', marginBottom: '5px' }}>
              {inst.label}
            </div>
            <div
              style={{
                fontSize: '15px',
                fontFamily: 'JetBrains Mono, monospace',
                fontWeight: 500,
                letterSpacing: '-0.01em',
                color: (inst as Record<string, unknown>).blue ? '#1a96e0'
                  : (inst as Record<string, unknown>).accent ? '#c85018'
                  : 'rgba(255,255,255,0.78)',
              }}
            >
              {inst.value}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

// ── Ticker ────────────────────────────────────────────────────────
const tickerItems = [
  'Zero-downtime deploys',
  'Web dashboard',
  'Real-time logs',
  'Auto restart',
  'Integrated reverse proxy',
  'VPS-friendly',
  'Systemd-managed',
  'No YAML sprawl',
  'One install command',
  'Rolling updates',
]

function Ticker() {
  const repeated = [...tickerItems, ...tickerItems]
  return (
    <div
      style={{
        backgroundColor: 'var(--clr-surface-2)',
        borderTop: '1px solid var(--clr-line)',
        borderBottom: '1px solid var(--clr-line)',
        overflow: 'hidden',
        padding: '14px 0',
      }}
    >
      <div
        className="ticker-track"
        style={{
          display: 'flex',
          gap: '0',
          width: 'max-content',
        }}
      >
        {repeated.map((item, i) => (
          <span
            key={i}
            style={{
              padding: '0 32px',
              fontSize: '12px',
              fontFamily: 'JetBrains Mono, monospace',
              color: i % 2 === 0 ? 'var(--clr-muted)' : 'var(--clr-silver)',
              letterSpacing: '0.04em',
              display: 'flex',
              alignItems: 'center',
              gap: '32px',
              whiteSpace: 'nowrap',
            }}
          >
            {item}
            <span style={{ color: i % 3 === 0 ? 'var(--clr-blue)' : 'var(--clr-accent)', fontSize: '8px' }}>◆</span>
          </span>
        ))}
      </div>
    </div>
  )
}

// ── Comparison ────────────────────────────────────────────────────
const compareRows = [
  { label: 'Web dashboard',          docker: false, lotsen: true, k8s: true  },
  { label: 'Zero-downtime deploys',  docker: false, lotsen: true, k8s: true  },
  { label: 'Automatic restarts',     docker: false, lotsen: true, k8s: true  },
  { label: 'Integrated reverse proxy', docker: false, lotsen: true, k8s: true },
  { label: 'VPS-friendly (1 server)', docker: true, lotsen: true, k8s: false },
  { label: 'Simple setup',           docker: true,  lotsen: true, k8s: false },
  { label: 'No YAML sprawl',         docker: true,  lotsen: true, k8s: false },
  { label: 'No cluster required',    docker: true,  lotsen: true, k8s: false },
]

// ── Features ──────────────────────────────────────────────────────
const features = [
  { Icon: Terminal,       title: 'One-command install',       description: 'A single curl command sets up three systemd services and starts everything. No manual steps.' },
  { Icon: LayoutDashboard, title: 'Web dashboard',            description: 'Deploy, edit, and remove containers from any browser. No SSH required after install.' },
  { Icon: RefreshCw,      title: 'Zero-downtime deploys',     description: 'Rolling updates keep your service running during upgrades, with automatic rollback on failure.' },
  { Icon: Network,        title: 'Integrated reverse proxy',  description: 'Routes HTTP traffic to containers by domain. Point DNS, set the domain field, done.' },
  { Icon: ScrollText,     title: 'Real-time logs',            description: 'Stream container logs directly in the dashboard. No SSH, no docker logs commands.' },
  { Icon: Activity,       title: 'System health',             description: 'Monitor API, orchestrator, Docker, and host CPU / RAM — all from one panel.' },
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
          padding: '80px 32px 90px',
          backgroundColor: 'var(--clr-bg)',
        }}
      >
        <ChartGrid opacity={0.055} size={48} />

        {/* Chart header labels */}
        <div
          style={{
            position: 'absolute',
            top: 18,
            left: 0,
            right: 0,
            display: 'flex',
            justifyContent: 'space-between',
            padding: '0 36px',
            pointerEvents: 'none',
          }}
        >
          <span style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: '10px', letterSpacing: '0.07em', color: 'rgba(26,52,82,0.2)' }}>
            CHART · LOTSEN-001
          </span>
          <span style={{ fontFamily: 'JetBrains Mono, monospace', fontSize: '10px', letterSpacing: '0.07em', color: 'rgba(26,52,82,0.2)' }}>
            63°24'N · 10°22'E
          </span>
        </div>

        <div
          style={{
            maxWidth: '1100px',
            margin: '0 auto',
            display: 'grid',
            gridTemplateColumns: '1fr 1fr',
            gap: '60px',
            alignItems: 'start',
          }}
        >
          {/* Left: copy */}
          <div style={{ paddingTop: '24px' }}>
            {/* Badge */}
            <div className="fade-up delay-1" style={{ marginBottom: '28px' }}>
              <span
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '7px',
                  fontSize: '11px',
                  fontFamily: 'JetBrains Mono, monospace',
                  color: 'var(--clr-accent)',
                  backgroundColor: 'var(--clr-accent-dim)',
                  padding: '5px 12px',
                  borderRadius: '4px',
                  border: '1px solid var(--clr-accent-border)',
                  fontWeight: 500,
                  letterSpacing: '0.04em',
                }}
              >
                <span className="cursor-blink">▋</span>
                v0.1 · ALPHA RELEASE — OPEN SOURCE
              </span>
            </div>

            {/* Headline */}
            <h1
              className="fade-up delay-2"
              style={{
                fontFamily: 'Fraunces, serif',
                fontStyle: 'italic',
                fontSize: 'clamp(36px, 4.5vw, 60px)',
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
              <span style={{ color: 'var(--clr-accent)' }}>Lotsen</span> guides the way.
            </h1>

            {/* Sub */}
            <p
              className="fade-up delay-3"
              style={{
                fontSize: '17px',
                lineHeight: 1.65,
                color: 'var(--clr-muted)',
                maxWidth: '460px',
                margin: '0 0 36px 0',
              }}
            >
              All the orchestration you need for a VPS — web dashboard,
              zero-downtime deployments, integrated proxy — with none of the
              Kubernetes learning curve. One install command.
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
                style={{ fontSize: '14px', color: 'var(--clr-subtle)', textDecoration: 'none', display: 'inline-flex', alignItems: 'center', gap: '5px', transition: 'color 0.15s' }}
                onMouseEnter={(e) => (e.currentTarget.style.color = 'var(--clr-muted)')}
                onMouseLeave={(e) => (e.currentTarget.style.color = 'var(--clr-subtle)')}
              >
                View on GitHub ↗
              </a>
            </div>
          </div>

          {/* Right: stacked mockups */}
          <div className="fade-up delay-3" style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
            <DashboardMockup />
            <LogMockup />
          </div>
        </div>

        {/* Bottom chart annotation */}
        <div
          style={{
            position: 'absolute',
            bottom: 16,
            right: 36,
            fontFamily: 'JetBrains Mono, monospace',
            fontSize: '10px',
            letterSpacing: '0.06em',
            color: 'rgba(26,52,82,0.16)',
            pointerEvents: 'none',
          }}
        >
          SCALE 1:250 000 · WGS84
        </div>
      </section>

      {/* ── Heavy sea ─────────────────────────────────────────── */}
      <HeavySea />

      {/* ── Instrument bar ────────────────────────────────────── */}
      <InstrumentBar />

      {/* ── Comparison ────────────────────────────────────────── */}
      <section
        style={{
          padding: '100px 32px',
          backgroundColor: 'var(--clr-dark)',
          position: 'relative',
          overflow: 'hidden',
        }}
      >
        <ChartGrid opacity={0.035} size={48} dark />

        {/* Subtle rust glow top-right */}
        <div
          style={{
            position: 'absolute',
            top: '-80px',
            right: '-80px',
            width: '400px',
            height: '400px',
            borderRadius: '50%',
            background: 'radial-gradient(circle, rgba(200,80,24,0.1) 0%, transparent 70%)',
            pointerEvents: 'none',
          }}
        />

        <div style={{ maxWidth: '1100px', margin: '0 auto', position: 'relative' }}>
          <p
            style={{
              fontSize: '10px',
              fontFamily: 'JetBrains Mono, monospace',
              color: 'var(--clr-accent)',
              letterSpacing: '0.14em',
              textTransform: 'uppercase',
              margin: '0 0 12px 0',
            }}
          >
            Navigation log · comparison
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
            <div style={{ padding: '32px', backgroundColor: 'rgba(255,255,255,0.02)' }}>
              <div style={{ marginBottom: '24px' }}>
                <p style={{ fontSize: '10px', fontFamily: 'JetBrains Mono, monospace', color: 'rgba(255,255,255,0.22)', letterSpacing: '0.1em', textTransform: 'uppercase', margin: '0 0 8px 0' }}>
                  Bare Docker
                </p>
                <p style={{ fontSize: '14px', color: 'rgba(255,255,255,0.35)', margin: 0, lineHeight: 1.5 }}>
                  Works, but everything breaks without you.
                </p>
              </div>
              {compareRows.map((r) => (
                <div key={r.label} style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: '8px 0', borderTop: '1px solid rgba(255,255,255,0.05)' }}>
                  {r.docker
                    ? <CheckCircle2 size={15} style={{ color: 'rgba(255,255,255,0.28)', flexShrink: 0 }} />
                    : <XCircle size={15} style={{ color: 'rgba(255,255,255,0.12)', flexShrink: 0 }} />
                  }
                  <span style={{ fontSize: '13px', color: r.docker ? 'rgba(255,255,255,0.48)' : 'rgba(255,255,255,0.2)' }}>{r.label}</span>
                </div>
              ))}
            </div>

            {/* Lotsen — highlighted */}
            <div
              style={{
                padding: '32px',
                backgroundColor: 'rgba(200,80,24,0.07)',
                borderLeft: '1px solid rgba(200,80,24,0.28)',
                borderRight: '1px solid rgba(200,80,24,0.28)',
                position: 'relative',
              }}
            >
              <div style={{ position: 'absolute', top: 0, left: 0, right: 0, height: '2px', backgroundColor: 'var(--clr-accent)' }} />
              <div style={{ marginBottom: '24px' }}>
                <p style={{ fontSize: '10px', fontFamily: 'JetBrains Mono, monospace', color: 'var(--clr-accent)', letterSpacing: '0.1em', textTransform: 'uppercase', margin: '0 0 8px 0' }}>
                  Lotsen ✦
                </p>
                <p style={{ fontSize: '14px', color: 'rgba(255,255,255,0.75)', margin: 0, lineHeight: 1.5 }}>
                  Production-grade orchestration that fits on one VPS.
                </p>
              </div>
              {compareRows.map((r) => (
                <div key={r.label} style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: '8px 0', borderTop: '1px solid rgba(200,80,24,0.1)' }}>
                  <CheckCircle2 size={15} style={{ color: 'var(--clr-accent)', flexShrink: 0 }} />
                  <span style={{ fontSize: '13px', color: 'rgba(255,255,255,0.85)' }}>{r.label}</span>
                </div>
              ))}
            </div>

            {/* Kubernetes */}
            <div style={{ padding: '32px', backgroundColor: 'rgba(255,255,255,0.02)' }}>
              <div style={{ marginBottom: '24px' }}>
                <p style={{ fontSize: '10px', fontFamily: 'JetBrains Mono, monospace', color: 'rgba(255,255,255,0.22)', letterSpacing: '0.1em', textTransform: 'uppercase', margin: '0 0 8px 0' }}>
                  Kubernetes
                </p>
                <p style={{ fontSize: '14px', color: 'rgba(255,255,255,0.35)', margin: 0, lineHeight: 1.5 }}>
                  Powerful, but built for teams with dedicated DevOps.
                </p>
              </div>
              {compareRows.map((r) => (
                <div key={r.label} style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: '8px 0', borderTop: '1px solid rgba(255,255,255,0.05)' }}>
                  {r.k8s
                    ? <CheckCircle2 size={15} style={{ color: 'rgba(255,255,255,0.28)', flexShrink: 0 }} />
                    : <XCircle size={15} style={{ color: 'rgba(255,255,255,0.12)', flexShrink: 0 }} />
                  }
                  <span style={{ fontSize: '13px', color: r.k8s ? 'rgba(255,255,255,0.48)' : 'rgba(255,255,255,0.2)' }}>{r.label}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </section>

      {/* ── Wave: comparison → features ───────────────────────── */}
      <WaveDivider bg="#111d2a" fill="#eef3f7" />

      {/* ── Ticker ────────────────────────────────────────────── */}
      <Ticker />

      {/* ── Features ──────────────────────────────────────────── */}
      <section style={{ padding: '100px 32px', backgroundColor: 'var(--clr-bg)', position: 'relative', overflow: 'hidden' }}>
        <ChartGrid opacity={0.04} size={48} />
        <div style={{ maxWidth: '1100px', margin: '0 auto', position: 'relative' }}>
          <p
            style={{
              fontSize: '10px',
              fontFamily: 'JetBrains Mono, monospace',
              color: 'var(--clr-accent)',
              letterSpacing: '0.14em',
              textTransform: 'uppercase',
              margin: '0 0 12px 0',
            }}
          >
            Ship's manifest · what's included
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
            {features.map(({ Icon, title, description }, i) => (
              <div key={title} style={{ padding: '32px', position: 'relative' }}>
                {/* Chart-style sequence number */}
                <div
                  style={{
                    position: 'absolute',
                    top: '20px',
                    right: '20px',
                    fontFamily: 'JetBrains Mono, monospace',
                    fontSize: '11px',
                    color: 'var(--clr-silver)',
                    letterSpacing: '0.04em',
                  }}
                >
                  {String(i + 1).padStart(2, '0')}
                </div>
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
                    border: '1px solid var(--clr-accent-border)',
                  }}
                >
                  <Icon size={18} style={{ color: 'var(--clr-accent)' }} />
                </div>
                <h3 style={{ fontSize: '15px', fontWeight: 600, color: 'var(--clr-text)', margin: '0 0 8px 0', letterSpacing: '-0.01em' }}>
                  {title}
                </h3>
                <p style={{ fontSize: '14px', lineHeight: 1.65, color: 'var(--clr-muted)', margin: 0 }}>
                  {description}
                </p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Harbor scene ──────────────────────────────────────── */}
      <WaveDivider bg="#eef3f7" fill="#0a1520" />
      <HarborScene />

      {/* ── CTA ───────────────────────────────────────────────── */}
      <section
        style={{
          padding: '80px 32px 100px',
          backgroundColor: 'var(--clr-dark)',
          position: 'relative',
          overflow: 'hidden',
          textAlign: 'center',
        }}
      >
        <ChartGrid opacity={0.03} size={48} dark />

        {/* Teal glow bottom center */}
        <div
          style={{
            position: 'absolute',
            bottom: '-60px',
            left: '50%',
            transform: 'translateX(-50%)',
            width: '500px',
            height: '300px',
            borderRadius: '50%',
            background: 'radial-gradient(circle, rgba(42,122,100,0.1) 0%, transparent 70%)',
            pointerEvents: 'none',
          }}
        />

        <div style={{ maxWidth: '640px', margin: '0 auto', position: 'relative' }}>
          {/* Lighthouse decoration */}
          <div style={{ display: 'flex', justifyContent: 'center', marginBottom: '16px' }}>
            <LighthouseSVG height={96} />
          </div>

          <h2
            style={{
              fontFamily: 'Fraunces, serif',
              fontStyle: 'italic',
              fontSize: 'clamp(30px, 5vw, 52px)',
              fontWeight: 800,
              letterSpacing: '-0.03em',
              color: '#ffffff',
              margin: '0 0 16px 0',
              lineHeight: 1.1,
            }}
          >
            Your VPS, finally orchestrated.
          </h2>
          <p
            style={{
              fontSize: '17px',
              color: 'rgba(255,255,255,0.5)',
              margin: '0 0 40px 0',
              lineHeight: 1.65,
            }}
          >
            Install in under a minute.
            Supports Ubuntu 22.04+ and Debian 11+.
          </p>
          <div style={{ display: 'flex', justifyContent: 'center', marginBottom: '20px' }}>
            <InstallBlock dark />
          </div>
          <p style={{ fontSize: '12px', fontFamily: 'JetBrains Mono, monospace', color: 'rgba(255,255,255,0.2)', letterSpacing: '0.06em' }}>
            63°24'N · 10°22'E · SIGNAL NOMINAL
          </p>
        </div>
      </section>

      <Footer />
    </div>
  )
}
