# dig web — production image (Next.js 16 standalone output).
#
# Built by the vllnt platform (BuildKit) from ntk.yaml. The build context is
# web/ (the Go CLI at the repo root is NOT part of this image). NEXT_PUBLIC_*
# values arrive as build args (Next.js inlines them at build time).

# syntax=docker/dockerfile:1.6

FROM node:22-alpine@sha256:968df39aedcea65eeb078fb336ed7191baf48f972b4479711397108be0966920 AS builder
WORKDIR /src

RUN corepack enable && corepack prepare pnpm@10.28.2 --activate

COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY . .

ARG NEXT_PUBLIC_SITE_URL
ENV NEXT_PUBLIC_SITE_URL=$NEXT_PUBLIC_SITE_URL

RUN pnpm build

# ---

FROM node:22-alpine@sha256:968df39aedcea65eeb078fb336ed7191baf48f972b4479711397108be0966920 AS runtime
WORKDIR /app

RUN addgroup -S -g 65532 app && adduser -S -u 65532 -G app app

# Next.js standalone output (output:'standalone' in next.config.mjs).
COPY --from=builder --chown=65532:65532 /src/.next/standalone ./
COPY --from=builder --chown=65532:65532 /src/.next/static ./.next/static

USER 65532:65532
EXPOSE 3000
ENV PORT=3000
ENV HOSTNAME=0.0.0.0
ENV NODE_ENV=production

CMD ["node", "server.js"]
