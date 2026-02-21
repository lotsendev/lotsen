FROM oven/bun:1 AS build
WORKDIR /app
COPY package.json bun.lockb ./
RUN bun install --frozen-lockfile
COPY . .
RUN bun run build

FROM oven/bun:1-slim AS production
WORKDIR /app
COPY --from=build /app/dist ./dist
COPY --from=build /app/server.ts ./
EXPOSE 3000
CMD ["bun", "run", "server.ts"]
