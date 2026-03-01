import { join } from "path"

const dist = join(import.meta.dir, "dist")
const index = Bun.file(join(dist, "index.html"))

const API_URL = (process.env.LOTSEN_API_URL ?? "http://localhost:8080").replace(/\/$/, "")

Bun.serve({
  port: 3000,
  async fetch(req) {
    const url = new URL(req.url)
    const { pathname, search } = url

    // Forward /api/* requests to the backend. Preserves method, headers, and
    // body. SSE responses (text/event-stream) stream through naturally because
    // the upstream fetch body is a ReadableStream — Bun does not buffer it.
    if (pathname.startsWith("/api")) {
      const headers = new Headers(req.headers)
      headers.delete("host")
      return fetch(`${API_URL}${pathname}${search}`, {
        method: req.method,
        headers,
        body: req.body,
      })
    }

    const file = Bun.file(join(dist, pathname))

    // Serve the file if it exists, otherwise fall back to index.html
    // so the React router can handle client-side navigation.
    if (pathname !== "/" && (await file.exists())) {
      return new Response(file)
    }

    return new Response(index)
  },
})

console.log("Dashboard listening on :3000")
