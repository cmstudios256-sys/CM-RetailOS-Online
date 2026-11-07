# CM RetailOS

CM RetailOS is a guest-first retail workspace for point of sale, inventory, sales, receipts, analytics, staff roles, and business settings. Connect a Google account to claim guest data into a private cloud shop.

## Features

- POS, products, stock adjustments, sales history, and receipts
- Owner, manager, and cashier roles with server-side authorization
- Offline guest mode with `localStorage`
- Google OAuth provisioning and guest-data claiming
- Camera barcode scanning, WhatsApp sharing, and installable PWA
- PostgreSQL/Supabase storage, versioned migrations, and HTTPS-ready cookies

## Screenshots

Screenshot placeholders:

- Dashboard: `docs/screenshots/dashboard.png`
- POS: `docs/screenshots/pos.png`
- Mobile: `docs/screenshots/mobile.png`

## Quickstart

Requirements: Go 1.25+, Node.js 20+, npm, and Docker.

```sh
cp .env.example .env
docker compose up --build
```

Open `http://localhost:8080`. The demo guest workspace is available without real credentials or personal data.

For local development:

```sh
cd api && go run ./cmd/server
cd web && npm install && npm run dev
```

The API listens on `127.0.0.1:8080` and Vite serves the frontend on port 5180 by default.

### Rich Demo Shop seed

The guest Demo Shop is seeded in browser `localStorage` automatically with 218 realistic products and 2,000 deterministic sales. This refresh happens only when the demo seed schema changes, so normal guest activity is not reset on startup.

For local PostgreSQL, run the API server once so migrations create the `shops.is_demo` flag, mark exactly one row as the Demo Shop (name `Demo Shop`, currency `UGX`, `is_demo = true`), then execute:

```powershell
cd api
$env:DATABASE_URL="postgres://retailos:retailos@localhost:5432/retailos?sslmode=disable"
.\seed-demo.ps1 -DemoShopId "<DEMO_SHOP_UUID>"
```

The seeder is transaction-based, deterministic, and idempotent: it refuses unless the target row is explicitly marked demo, then resets only that shop before inserting 218 products, 10 categories, and 2,000 sales. Demo seeding is intentionally manual rather than automatic at API startup; automatic startup mutations would make database behavior surprising and could accidentally affect deployments.

## Google sign-in

**Connect Google** and **Save to cloud** use Google OAuth to sign users in with a Gmail/Google account. OAuth must be configured on the API; the buttons cannot complete sign-in when the API or Google credentials are unavailable.

Create a Google OAuth client with application type **Web application**. For the Docker quickstart, configure:

- Redirect URI: `http://localhost:8080/api/v1/auth/google/callback`
- Authorized JavaScript origin: `http://localhost:8080`

Set `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` in the root `.env` file. Docker Compose passes them to the API along with the local callback and app URL. Restart with `docker compose up --build` after changing the values.

For Vite local development, use application origin `http://localhost:5180` for `APP_BASE_URL`; the callback remains `http://localhost:8080/api/v1/auth/google/callback`. The API must be running on port 8080.

For production, configure the OAuth client with:

- Redirect URI: `https://[DOMAIN]/api/v1/auth/google/callback` when same-origin hosting is used
- Authorized JavaScript origins: `https://[DOMAIN]`

Set `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URL`, and `APP_BASE_URL` as API environment variables. Production mode also requires `COOKIE_SECURE=true`. In Google Auth Platform, set the audience to **External** and publish the app to allow Google accounts outside the configured test users; while the consent screen is in testing, only listed test users can sign in.

## Email via Brevo SMTP

Optional password-reset email uses Brevo SMTP. Configure `SMTP_HOST=smtp-relay.brevo.com`, `SMTP_PORT=587`, `SMTP_USERNAME`, `SMTP_PASSWORD`, and `SMTP_FROM` together.

## Environment

Copy `.env.example` and fill only the variables required for the selected environment. The example contains dummy values; no production secrets are committed.

## Docker self-hosting

```sh
cp .env.example .env
docker compose up -d --build
```

The included PostgreSQL volume is persistent. Replace local placeholder secrets before exposing the service publicly and use HTTPS plus `COOKIE_SECURE=true` in production.

## Render preview deployment

The root `render.yaml` defines a free preview deployment of the combined PWA/API web service and PostgreSQL database. To create it, connect this GitHub repository to Render and create a Blueprint from the repository's `main` branch. Render assigns the app an HTTPS `onrender.com` URL; no custom domain is needed. The service deploys from Git pushes after it is linked.

This Blueprint is for preview only. Render's free web service can sleep when idle, and its free PostgreSQL database expires after 30 days. Do not store real business data in this preview. The preview starts without Google sign-in; configure production OAuth only after choosing persistent paid hosting and confirming the app URL. For Google OAuth, use that URL as the authorized JavaScript origin and set its `/api/v1/auth/google/callback` path as the authorized redirect URI.

## Verification

```sh
cd api && go test ./...
cd web && npm ci && npm run build
```

## Operations

See [RUNBOOK.md](RUNBOOK.md) for backups, secret rotation, monitoring, and incident response.

## License

MIT. See [LICENSE](LICENSE).
