export default function DeploymentConfiguration() {
  return (
    <>
      <h2>Deployment Configuration</h2>
      <p>
        A deployment is the central object in Dirigent. It describes a container you want to
        keep running — the image to use, how to expose it, and what environment it needs. The
        orchestrator continuously reconciles your running containers against these declarations.
      </p>

      {/* Field reference */}
      <h2>Field reference</h2>

      <table>
        <thead>
          <tr>
            <th>Field</th>
            <th>Type</th>
            <th>Required</th>
            <th>Description</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>name</td>
            <td>string</td>
            <td>Yes</td>
            <td>
              A human-readable identifier for the deployment. Used as the container name in
              Docker. Must be unique across all deployments.
            </td>
          </tr>
          <tr>
            <td>image</td>
            <td>string</td>
            <td>Yes</td>
            <td>
              The Docker image to run, including tag. The orchestrator pulls this image before
              starting the container. Example: <code>nginx:1.27</code> or{' '}
              <code>ghcr.io/myorg/api:latest</code>.
            </td>
          </tr>
          <tr>
            <td>ports</td>
            <td>string[]</td>
            <td>No</td>
            <td>
              Port mappings in <code>host:container</code> format. Each entry maps a port on
              the VPS host to a port inside the container. Example:{' '}
              <code>["80:80", "443:443"]</code>.
            </td>
          </tr>
          <tr>
            <td>volumes</td>
            <td>string[]</td>
            <td>No</td>
            <td>
              Volume mounts in <code>host-path:container-path</code> format. The host path
              must be an absolute path on the VPS. Example:{' '}
              <code>["/data/postgres:/var/lib/postgresql/data"]</code>.
            </td>
          </tr>
          <tr>
            <td>envs</td>
            <td>object</td>
            <td>No</td>
            <td>
              Environment variables passed into the container as a key-value map. Values are
              stored in the Dirigent data file on disk. Example:{' '}
              <code>{'{"DATABASE_URL": "postgres://..."}'}</code>.
            </td>
          </tr>
          <tr>
            <td>domain</td>
            <td>string</td>
            <td>No</td>
            <td>
              A fully-qualified domain name to route to this container via the integrated
              reverse proxy. Point your DNS A record to the VPS IP, and Dirigent will forward
              HTTP traffic on port 80. Example: <code>api.example.com</code>.
            </td>
          </tr>
        </tbody>
      </table>

      {/* Ports */}
      <h2>Ports</h2>
      <p>Port mappings use the Docker format:</p>
      <pre>
        <code>{`host-port:container-port`}</code>
      </pre>
      <p>Examples:</p>
      <pre>
        <code>{`"8080:80"   // expose container port 80 as host port 8080
"3306:3306" // MySQL
"5432:5432" // Postgres`}</code>
      </pre>
      <p>
        Omitting ports means the container is not directly accessible from the host. Use the{' '}
        <code>domain</code> field with the reverse proxy instead for HTTP services.
      </p>

      {/* Volumes */}
      <h2>Volumes</h2>
      <p>Volumes persist data across container restarts and re-deployments:</p>
      <pre>
        <code>{`"/var/lib/dirigent/myapp:/data"  // host path : container path`}</code>
      </pre>

      <div className="callout">
        <strong>Note:</strong> The host path must exist before the deployment is created. Dirigent
        does not create directories on your behalf. Use <code>mkdir -p /path/to/dir</code> on
        the VPS first.
      </div>

      {/* Environment variables */}
      <h2>Environment variables</h2>
      <p>
        Environment variables are entered as key-value pairs in the dashboard. They are stored
        in the Dirigent state file at <code>/var/lib/dirigent/deployments.json</code> — a file
        on disk that is only readable by root.
      </p>

      <div className="callout">
        <strong>Security note:</strong> Do not store highly sensitive credentials (private keys,
        payment tokens) in environment variables if your VPS is not hardened. For production
        secrets management, consider mounting a secrets file as a volume instead.
      </div>

      <p>Common usage patterns:</p>
      <pre>
        <code>{`DATABASE_URL=postgres://user:pass@localhost:5432/mydb
NODE_ENV=production
PORT=3000`}</code>
      </pre>

      {/* Domain & proxy */}
      <h2>Domain and reverse proxy</h2>
      <p>
        The integrated reverse proxy listens on port 80 and routes incoming HTTP requests to
        the correct container based on the <code>Host</code> header.
      </p>
      <p>To expose a deployment via a domain:</p>
      <ol
        style={{
          paddingLeft: '20px',
          fontSize: '15px',
          lineHeight: 1.75,
          color: 'var(--clr-subtle)',
        }}
      >
        <li>
          Set the <strong>Domain</strong> field to your fully-qualified domain name (e.g.{' '}
          <code>app.example.com</code>).
        </li>
        <li>Create a DNS A record pointing that domain to your VPS IP address.</li>
        <li>
          Ensure the container exposes a port on its internal network (no host port mapping
          required — the proxy communicates over the <code>dirigent</code> Docker network).
        </li>
      </ol>

      <div className="callout">
        <strong>Note:</strong> The proxy currently handles HTTP only. TLS termination is on the
        roadmap for a future release.
      </div>

      {/* Deployment lifecycle */}
      <h2>Deployment lifecycle</h2>
      <p>A deployment moves through the following states:</p>
      <table>
        <thead>
          <tr>
            <th>Status</th>
            <th>Meaning</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>idle</td>
            <td>
              The deployment has been created but the orchestrator has not yet acted on it.
            </td>
          </tr>
          <tr>
            <td>deploying</td>
            <td>
              The orchestrator is pulling the image and starting the container.
            </td>
          </tr>
          <tr>
            <td>healthy</td>
            <td>The container is running and passing health checks.</td>
          </tr>
          <tr>
            <td>failed</td>
            <td>
              The container exited unexpectedly or the image pull failed. Check the logs panel
              on the deployment detail page for the error.
            </td>
          </tr>
        </tbody>
      </table>
    </>
  )
}
