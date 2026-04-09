# Deployment Plan: Docker + VPS + Domain

**Goal:** Get the app running publicly so anyone can connect with `ssh yourdomain.com`

---

## Overview

Reference uses AWS ECS + SST (overkill). We use:

- **Hetzner VPS** (~€4/month) — Linux server with a real public IP
- **Docker Compose** — runs all services, replaces `dev.sh`/Procfile
- **Caddy** — reverse proxy for the API, handles HTTPS automatically (free Let's Encrypt)
- **Cloudflare** — DNS (free tier, fast propagation)
- **Your domain** — e.g. `yourcoffee.shop`

Port layout on the VPS:
- `:22` → SSH TUI server (users connect here)
- `:443` → API (Caddy terminates TLS, proxies to API container on :8080)
- `:80` → Caddy (redirects to HTTPS)

---

## Step 1: Prerequisites

### 1a. Buy a domain
- Recommended: Cloudflare Registrar (cheapest, DNS included, no upsells)
- Or: Namecheap, then point nameservers to Cloudflare

### 1b. Get a VPS
- **Hetzner Cloud** → Create account → New Server
  - Location: closest to you (Falkenstein/Helsinki for EU)
  - OS: Ubuntu 24.04
  - Type: CX22 (2 vCPU, 4GB RAM, €3.79/month) — plenty for this
  - Add your SSH public key during setup so you can SSH in
- Note the public IP address

### 1c. DNS (Cloudflare)
Add these A records pointing to your VPS IP:

| Type | Name | Value |
|------|------|-------|
| A | `@` | `<VPS IP>` |
| A | `www` | `<VPS IP>` |
| A | `api` | `<VPS IP>` |

Set proxy status to **DNS only** (grey cloud) — not proxied. Cloudflare proxying breaks raw TCP (SSH).

---

## Step 2: Files to Create

### `Dockerfile.ssh`

Directly based on reference's `packages/go/Dockerfile`. Two-stage build: compile in Go Alpine, run in minimal Alpine.

The reference uses `CGO_ENABLED=0` but our SQLite driver needs CGO. Fix: switch to the pure-Go SQLite driver (`modernc.org/sqlite`) so we can keep `CGO_ENABLED=0` and get tiny images.

```dockerfile
FROM golang:alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o ssh ./main.go

FROM alpine:3.21
WORKDIR /root/
COPY --from=builder /app/ssh .
CMD TERM=xterm-256color ./ssh
```

### `Dockerfile.api`

```dockerfile
FROM golang:alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o api ./api/main.go

FROM alpine:3.21
WORKDIR /root/
COPY --from=builder /app/api .
CMD ["./api"]
```

### `docker-compose.prod.yml`

```yaml
services:
  api:
    build:
      context: .
      dockerfile: Dockerfile.api
    restart: unless-stopped
    env_file: .env.prod
    volumes:
      - ./data:/data
    expose:
      - "8080"

  ssh:
    build:
      context: .
      dockerfile: Dockerfile.ssh
    restart: unless-stopped
    env_file: .env.prod
    ports:
      - "22:22"
    volumes:
      - ./.ssh:/root/.ssh
    depends_on:
      - api

  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - caddy_data:/data
      - caddy_config:/config

volumes:
  caddy_data:
  caddy_config:
```

### `Caddyfile`

```
api.yourdomain.com {
    reverse_proxy api:8080
}
```

Caddy automatically gets a Let's Encrypt cert for the domain. No manual SSL setup.

### `.env.prod`

```
# API
API_PORT=8080
DATABASE_URL=/data/terminalshop.db
JWT_SECRET=<generate with: openssl rand -hex 32>
AUTH_FINGERPRINT_KEY=<generate with: openssl rand -hex 32>
ENVIRONMENT=production
APP_URL=https://api.yourdomain.com

# Stripe (live keys in production)
STRIPE_SECRET_KEY=sk_live_...
STRIPE_PUBLIC_KEY=pk_live_...
STRIPE_WEBHOOK_SECRET=<from stripe dashboard>

# Shipping
SHIPPO_API_KEY=...
BRING_API_UID=...
BRING_API_KEY=...

# SSH server
API_URL=http://api:8080
```

Note: `API_URL` uses `api` (the Docker Compose service name) not `localhost` — containers talk to each other by service name.

### `.dockerignore`

```
.git
.env
.env.*
!.env.example
terminal-shop-source/
*.db
.ssh/
docs/
```

---

## Step 3: Code Changes

### 3a. SSH server host binding

`main.go` currently listens on `localhost`. In Docker it must listen on `0.0.0.0` to accept connections from outside the container.

```go
// Change:
host = "localhost"
port = "23456"

// To (read from env):
host = "0.0.0.0"
port = os.Getenv("SSH_PORT")  // default "22" in prod, "23456" in dev
```

Or just hardcode `0.0.0.0` and keep port as env var.

### 3b. SSH host key path

Currently hardcoded to `.ssh/term_info_ed25519`. In Docker this needs to be an absolute path or a path relative to the working directory that gets volume-mounted. The volume mount in `docker-compose.prod.yml` handles this — generate the key once on the server and it persists.

Generate on the VPS before first run:
```bash
mkdir -p .ssh
ssh-keygen -t ed25519 -f .ssh/term_info_ed25519 -N ""
```

### 3c. Switch SQLite driver to pure Go

In `pkg/database/db.go`, swap the import:

```go
// Remove:
_ "github.com/mattn/go-sqlite3"  // or however it's imported

// Add:
_ "modernc.org/sqlite"
```

And update go.mod:
```bash
go get modernc.org/sqlite
go mod tidy
```

This lets us use `CGO_ENABLED=0` in both Dockerfiles, keeping images small and the build simple.

---

## Step 4: VPS Setup

SSH into your new server:

```bash
ssh root@<VPS IP>
```

Install Docker:
```bash
curl -fsSL https://get.docker.com | sh
```

Clone your repo:
```bash
git clone https://github.com/yourusername/terminalShop.git
cd terminalShop
```

Create data directory and generate SSH host key:
```bash
mkdir -p data .ssh
ssh-keygen -t ed25519 -f .ssh/term_info_ed25519 -N ""
```

Create `.env.prod` with your production secrets (copy template above, fill in real values).

Open firewall:
```bash
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw enable
```

**Important:** Port 22 is now used by your app. Change admin SSH to a different port first (e.g. 2222) so you don't lock yourself out:

```bash
# In /etc/ssh/sshd_config, change:
Port 2222

systemctl restart sshd
# Open new terminal, verify: ssh root@<VPS IP> -p 2222
# Then: ufw allow 2222/tcp
```

---

## Step 5: First Deploy

On the VPS:
```bash
docker compose -f docker-compose.prod.yml up -d --build
```

Check logs:
```bash
docker compose -f docker-compose.prod.yml logs -f
```

Test:
```bash
ssh yourdomain.com
```

---

## Step 6: Stripe Webhook

In Stripe Dashboard → Developers → Webhooks:
- Add endpoint: `https://api.yourdomain.com/api/v1/webhooks/stripe`
- Copy the signing secret into `.env.prod` as `STRIPE_WEBHOOK_SECRET`
- No more ngrok needed in production

---

## Step 7: Ongoing Deploys

When you push changes:
```bash
# On VPS:
git pull
docker compose -f docker-compose.prod.yml up -d --build
```

Or automate with a GitHub Action that SSHes into the VPS and runs this on every push to main.

---

## Dev vs Prod Summary

| | Dev | Prod |
|--|-----|------|
| Process manager | Goreman + Procfile | Docker Compose |
| SSH port | 23456 | 22 |
| API URL | localhost:8080 | api.yourdomain.com (HTTPS) |
| Database | `terminalshop.db` (local) | `/data/terminalshop.db` (volume) |
| Stripe | ngrok + test keys | real domain + live keys |
| Webhooks | stripe CLI listener | Stripe dashboard endpoint |

---

## TODO After Deployment

- [ ] Switch from SQLite to PostgreSQL (add `postgres` service to compose, update connection string)
- [ ] Set up GitHub Actions for automatic deploy on push to main
- [ ] Add health check endpoint to Docker Compose so containers restart if app hangs
- [ ] Structured logging (write to file + stdout in prod)
