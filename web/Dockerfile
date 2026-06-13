# dig web — production image (Next.js 16 standalone output).
#
# Built by the vllnt platform (BuildKit) from ntk.yaml. The deployer builds from
# a git context rooted at the REPO ROOT (it ignores build.context for git
# builds), so all COPY paths are repo-root-relative and select the web/ subdir
# explicitly — the Go CLI at the repo root is simply never COPYed in.
# NEXT_PUBLIC_* values arrive as build args (Next.js inlines them at build time).

# syntax=docker/dockerfile:1.6

FROM node:22-alpine@sha256:968df39aedcea65eeb078fb336ed7191baf48f972b4479711397108be0966920 AS builder
WORKDIR /src

RUN corepack enable && corepack prepare pnpm@10.28.2 --activate

COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY web/ ./

ARG NEXT_PUBLIC_SITE_URL
ENV NEXT_PUBLIC_SITE_URL=$NEXT_PUBLIC_SITE_URL

RUN pnpm build

# ---

FROM node:22-alpine@sha256:968df39aedcea65eeb078fb336ed7191baf48f972b4479711397108be0966920 AS runtime
WORKDIR /app

RUN addgroup -S -g 65532 app && adduser -S -u 65532 -G app app

# Next.js standalone output (output:'standalone' in next.config.mjs). Standalone
# bundles server.js + traced node_modules but NOT .next/static or public/ —
# both must be copied in by hand, or static assets (llms.txt, install.sh, …) 404.
COPY --from=builder --chown=65532:65532 /src/.next/standalone ./
COPY --from=builder --chown=65532:65532 /src/.next/static ./.next/static
COPY --from=builder --chown=65532:65532 /src/public ./public

USER 65532:65532
EXPOSE 3000
ENV PORT=3000
ENV HOSTNAME=0.0.0.0
ENV NODE_ENV=production

CMD ["node", "server.js"]
