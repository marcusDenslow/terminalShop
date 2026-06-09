# Terminal Coffee Shop

```bash
ssh sshops.uk
```

## demo
https://github.com/user-attachments/assets/daf895ac-2c01-4323-bf27-8d7829b9ddd5

it's a coffee shop but in your terminal. you ssh in, browse the menu, chuck stuff in your cart, put in your address and card, and buy coffee without ever touching a browser. your ssh key is your account so there's no login or signup or any of that.

made it because i thought it would be fun to see if you could build a real e-commerce flow that lives entirely in a terminal. turns out you can. heavily inspired by [terminal.shop](https://terminal.shop).

## what it actually does

ssh in and you get a TUI. browse coffee, add to cart, save addresses, save cards. checkout charges your card via stripe and creates a real order. addresses get validated against shippo (US) or bring (Norway) before they're allowed in. cards are tokenized server side so the secret key never touches your terminal session. adding cards through the tui is coming soon, i just need to register a business and i have no money. webhooks from stripe reconcile order state if anything goes weird mid charge, and a cron job catches orders that got charged but never recorded because the server crashed.

it's a real shop. you can actually buy coffee.

## the stack

**TUI + SSH.** Go with [Bubbletea](https://github.com/charmbracelet/bubbletea) for the terminal UI, [Wish](https://github.com/charmbracelet/wish) for the ssh server. every ssh session spawns its own bubbletea program. lipgloss for styling.

**API.** Go again, [Chi](https://github.com/go-chi/chi) for routing. JWT auth where the token is exchanged for an ssh fingerprint, so the TUI authenticates by proving it has the same key the user ssh'd in with. no passwords anywhere.

**Database.** SQLite via GORM. yeah just a file. it's fine. i back it up nightly with restic and the SQLite backup API so it's WAL safe even if a write is in flight. when traffic actually demands postgres i'll switch. the DSN prefix logic in `pkg/database/db.go` already handles both drivers.

**Payments.** [Stripe](https://stripe.com). cards are tokenized server side, never raw card data on disk. webhooks handle async confirmations like 3DS, deferred charges, refunds. idempotency keys tied to order IDs so retries can't double charge.

**Address validation.** [Shippo](https://goshippo.com) for US addresses, [Bring](https://www.bring.com) for Norway. invalid addresses get rejected before they hit the cart.

## where it runs

production lives on a single Hetzner VPS in Helsinki, behind [Caddy](https://caddyserver.com) which handles auto TLS so there's no nginx config to babysit. everything in docker compose. the API, caddy, and the whole observability stack run as separate compose stacks on the same box. public DNS only resolves the API and the marketing site, everything else is tailnet only.

backups go to a second machine sitting in my house. that machine runs **Linux From Scratch** with a hand compiled kernel and a hand rolled ports repo i maintain myself with [scratchpkg](https://github.com/emmett1/scratchpkg). no docker, no apt, every binary built from a recipe i wrote. it also runs my self hosted [Gitea](https://gitea.com). it's got like 4 GB of RAM so it has to run lean. picked it because i wanted to learn how a linux box actually fits together when nobody's holding your hand.

backup pipeline runs nightly. cron on the Hetzner box snapshots the SQLite db via the backup API, ships it over Tailscale SFTP to the LFS box with [restic](https://restic.net), encrypted and deduped. dual dead man switches, [healthchecks.io](https://healthchecks.io) and a Uptime Kuma push monitor, both go off if the cron silently dies.

## observability

probably the over engineered part. honestly built it because i wanted to learn the stack.

**Prometheus** scrapes the API for the usual RED metrics plus custom counters like cart conversions, auth attempts, stripe events, log levels, orders created.

**Grafana** sits in front of everything. tailnet only, no public DNS.

**Loki + Promtail** for logs. promtail auto discovers every container via docker labels so anything new on the box is logged for free.

**Tempo** for traces. the API exports OTLP via [otelchi](https://github.com/riandyrn/otelchi) on the inbound side and [otelhttp](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp) on outbound calls to stripe, shippo, bring. the GORM tracing plugin emits a span per SQL query.

**Uptime Kuma** for blackbox checks. API health, TLS, backup target reachable.

**Alerts** route through Grafana to a ntfy webhook to my phone.

end to end, a request to `/api/v1/cart` shows up as one trace with the chi handler as the root, gorm queries as children, and outbound HTTP to stripe as a sibling. all one tempo span tree.

## local dev

```bash
git clone https://github.com/marcusDenslow/terminalShop
cd terminalShop
cp .env.example .env  # fill in stripe test keys
go run main.go        # ssh server on :23457
go run api/main.go    # API on :8080
ssh localhost -p 23457
```

uses SQLite by default so there's nothing to spin up.

## inspiration

[terminal.shop](https://terminal.shop) for the original idea and most of the patterns in `pkg/tui/`. their source is open and worth reading.
# something
