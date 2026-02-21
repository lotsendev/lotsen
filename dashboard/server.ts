import { join } from "path"

const dist = join(import.meta.dir, "dist")
const index = Bun.file(join(dist, "index.html"))

Bun.serve({
  port: 3000,
  async fetch(req) {
    const pathname = new URL(req.url).pathname
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
