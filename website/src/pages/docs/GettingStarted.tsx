import { CopyButton } from '@/components/CopyButton'
import { INSTALL_COMMAND } from '@/constants'

function CodeBlock({ children }: { children: string }) {
  return (
    <pre>
      <code>{children}</code>
    </pre>
  )
}

function InlineCmd({ children }: { children: string }) {
  return <code>{children}</code>
}

export default function GettingStarted() {
  return (
    <>
      <h2>Getting Started</h2>
      <p>
        Dirigent installs as four systemd services on your VPS. This guide walks you from a
        fresh Ubuntu or Debian server to running your first container in under five minutes.
      </p>

      {/* Prerequisites */}
      <h2>Prerequisites</h2>
      <ul>
        <li>
          <strong>OS:</strong> Ubuntu 22.04 (Jammy) or later, or Debian 11 (Bullseye) or later.
        </li>
        <li>
          <strong>Architecture:</strong> x86_64 or aarch64.
        </li>
        <li>
          <strong>Root access:</strong> the installer must run as root or with{' '}
          <InlineCmd>sudo</InlineCmd>.
        </li>
        <li>
          <strong>Open ports:</strong> <InlineCmd>80</InlineCmd> (reverse proxy) and{' '}
          <InlineCmd>8080</InlineCmd> (API).
        </li>
        <li>
          <strong>Docker:</strong> if not already installed, the installer will install it for
          you.
        </li>
      </ul>

      {/* Installation */}
      <h2>Installation</h2>
      <p>Run the following command on your VPS:</p>

      <div
        style={{
          background: 'var(--clr-surface)',
          border: '1px solid var(--clr-line)',
          borderRadius: '8px',
          padding: '16px 20px',
          display: 'flex',
          alignItems: 'center',
          gap: '10px',
          margin: '20px 0',
        }}
      >
        <span
          style={{
            fontFamily: 'JetBrains Mono, monospace',
            fontSize: '13px',
            color: 'var(--clr-accent)',
            userSelect: 'none',
            flexShrink: 0,
          }}
        >
          $
        </span>
        <code
          style={{
            fontFamily: 'JetBrains Mono, monospace',
            fontSize: '13px',
            color: 'var(--clr-subtle)',
            flex: 1,
            minWidth: 0,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
            background: 'none',
            border: 'none',
            padding: 0,
          }}
        >
          {INSTALL_COMMAND}
        </code>
        <CopyButton text={INSTALL_COMMAND} />
      </div>

      <p>The installer will:</p>
      <ul>
        <li>Install Docker if it is not already present.</li>
        <li>Install Bun (required to run the dashboard server).</li>
        <li>Download and install the four Dirigent binaries.</li>
        <li>Create a Docker bridge network named <InlineCmd>dirigent</InlineCmd>.</li>
        <li>
          Write and enable four systemd units:{' '}
          <InlineCmd>dirigent-api</InlineCmd>, <InlineCmd>dirigent-orchestrator</InlineCmd>,{' '}
          <InlineCmd>dirigent-proxy</InlineCmd>, and <InlineCmd>dirigent-dashboard</InlineCmd>.
        </li>
        <li>
          Prompt for optional dashboard domain + Basic Auth setup (works in normal SSH sessions,
          including piped install commands).
        </li>
      </ul>

      <div className="callout">
        <strong>Tip:</strong> To pin a specific version, prefix the command with{' '}
        <InlineCmd>DIRIGENT_VERSION=v0.0.2</InlineCmd> before the curl.
      </div>

      {/* Verify */}
      <h2>Verify the installation</h2>
      <p>
        Once the installer completes, confirm all four services are running:
      </p>
      <CodeBlock>{`systemctl status dirigent-api
systemctl status dirigent-orchestrator
systemctl status dirigent-proxy
systemctl status dirigent-dashboard`}</CodeBlock>

      <p>
        Each service should report <InlineCmd>active (running)</InlineCmd>. If one has failed,
        inspect its logs:
      </p>
      <CodeBlock>{`journalctl -u dirigent-api -n 50`}</CodeBlock>

      {/* Accessing the dashboard */}
      <h2>Access the dashboard</h2>
      <p>
        By default, the dashboard is available directly on port <InlineCmd>3000</InlineCmd>:
      </p>
      <CodeBlock>{`http://<your-vps-ip>:3000`}</CodeBlock>

      <p>
        The dashboard connects to the local API on port 8080. The orchestrator has no public
        inbound port.
      </p>

      <h3>Expose dashboard publicly (HTTPS + Basic Auth)</h3>
      <p>
        You can configure or update dashboard proxy exposure any time:
      </p>
      <CodeBlock>{`sudo dirigent setup`}</CodeBlock>
      <p>
        The setup command writes values to <InlineCmd>/etc/dirigent/dirigent.env</InlineCmd>,
        restarts the proxy, and enables dashboard access at{' '}
        <InlineCmd>https://dashboard.example.com</InlineCmd> and protected by HTTP Basic Auth.
      </p>

      {/* First deployment */}
      <h2>Your first deployment</h2>

      <h3>1. Open the Deployments page</h3>
      <p>
        In the sidebar, click <strong>Deployments</strong>. This lists all containers
        Dirigent is currently managing.
      </p>

      <h3>2. Click "Create deployment"</h3>
      <p>A dialog opens with fields for your container configuration.</p>

      <h3>3. Fill in the details</h3>
      <p>
        For a quick test, deploy a simple nginx container:
      </p>
      <ul>
        <li>
          <strong>Name:</strong> <InlineCmd>nginx</InlineCmd>
        </li>
        <li>
          <strong>Image:</strong> <InlineCmd>nginx:latest</InlineCmd>
        </li>
        <li>
          <strong>Ports:</strong> <InlineCmd>80:80</InlineCmd>
        </li>
      </ul>

      <h3>4. Save</h3>
      <p>
        Click <strong>Create</strong>. Dirigent stores the deployment, and the orchestrator
        pulls the image and starts the container within a few seconds. The status badge in the
        list will transition from <InlineCmd>deploying</InlineCmd> to{' '}
        <InlineCmd>healthy</InlineCmd>.
      </p>

      <h3>5. Verify</h3>
      <p>
        With a port mapping of <InlineCmd>80:80</InlineCmd>, nginx will be reachable at{' '}
        <InlineCmd>http://&lt;your-vps-ip&gt;</InlineCmd>.
      </p>

      <div className="callout">
        <strong>Next:</strong> To route traffic by domain name instead of port, add a{' '}
        <strong>Domain</strong> to your deployment and point your DNS A record to the VPS. The
        integrated proxy will handle the rest.
      </div>
    </>
  )
}
