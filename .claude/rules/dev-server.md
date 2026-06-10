# Dev server — port + exposure rule (BLOCKING)

This project runs on a **fixed port** and is exposed beyond localhost **only via `tailscale serve`**.

## Port

- **dig-web → port `3977`** (fixed; assigned deterministically to avoid collisions with other local projects).
- The port is wired into `web/package.json` (`dev`, `start`), `web/.env.example` (`PORT`), and this rule. Keep them in sync.
- Local dev: `pnpm -C web dev` → `http://localhost:3977`
- Never auto-fall-back to `3000`. If `3977` is busy, find the owning process and stop it — do not silently shift ports.

## Exposure beyond localhost (BLOCKING)

When the dev server must be reachable from another device, another machine, a teammate, or a
remote browser/E2E run, expose it with **Tailscale Serve** — tailnet-only HTTPS, valid certs,
stable hostname. **Never** a public bind or tunnel.

```bash
# Expose localhost:3977 over your tailnet (HTTPS, private to your devices)
tailscale serve --bg 3977
# or explicit:
tailscale serve --bg --https=443 localhost:3977

tailscale serve status          # show the https://<host>.<tailnet>.ts.net URL
tailscale serve reset           # stop serving (clear all)
```

Convenience scripts: `pnpm -C web serve:tailscale` / `pnpm -C web serve:tailscale:off`.

## NEVER

- `next dev -H 0.0.0.0` or any `0.0.0.0` bind — leaks the app to the whole LAN.
- `ngrok`, `cloudflared`, `localtunnel`, or other public tunnels for routine dev.
- `tailscale funnel` — that publishes to the **public internet**. Serve (private tailnet) is the rule;
  use Funnel only with explicit, deliberate intent for a public demo, never as the default.

## Why

- **No collisions**: every local project owns a distinct, stable port.
- **Private by default**: tailnet-only reach, no public exposure of an unfinished app.
- **Real HTTPS**: device testing, OAuth callbacks, and secure-context APIs work without cert hacks.
- **Stable URL**: the `*.ts.net` hostname doesn't churn between runs like tunnel URLs do.
