# CM RetailOS Online

Separate browser-based product for CM RetailOS. The existing Windows desktop app remains independent and continues to use its local SQLite database.

## Project layout

- `web/` - responsive Vue browser client.
- `api/` - Go HTTP API and PostgreSQL schema.

The browser client communicates with the API over same-origin `/api/v1` routes. Production hosting should route `/api/*` to the Go service and serve the built web client from the same app origin. This keeps the session cookie first-party; the official marketing website can link to the app without hosting application data itself.

## Local development

Requirements: Go 1.25 or newer, Node.js 20 or newer, npm, and PostgreSQL 15 or newer.

1. Install PostgreSQL and make sure its service is running.
2. From the project root, run `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\api\configure-local.ps1`. It prompts for the PostgreSQL `postgres` password without echoing it, verifies the local connection, creates the development database if needed, and writes the connection URL into the ignored `.env.local` file.
3. Run `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\api\run-local.ps1`. It applies the API schema and starts the Go API.
4. In `web/`, run `npm install` once, then `npm run dev`.

The Vite development server proxies `/api` requests to `http://127.0.0.1:8080`.

The API applies the checked-in idempotent schema at startup. Set `PUBLIC_SHOP_ID` to one shop UUID to show that shop's dashboard publicly without login. The public view is read-only and exposes dashboard sales/profit summaries, inventory value, top products, and stock-alert details to anyone who can reach the app; do not select a shop whose data should remain private. Other shops remain session-protected. `PUBLIC_LANDING_ONLY` defaults to `false`; set it to `true` to temporarily expose only the health endpoint while preserving account and shop data. Never use development credentials or disable secure cookies in production. Before public launch, configure production HTTPS and secure cookies, managed database backups, request rate limiting, and monitoring.

## Google sign-in setup

The public browser view opens directly to the configured dashboard and does not display sign-in or signup pages. Owner account endpoints remain available for authenticated workflows and Google OAuth, but public dashboard access is read-only. Email signup creates a shop and pending owner account, then requires a one-time email verification link before sign-in. Google sign-in uses an OpenID Connect authorization-code flow with state, nonce, and PKCE checks. When Google provides a profile photo, the app displays it in the account avatar and falls back to initials if it is unavailable. A first-time Google sign-in creates a verified owner and a shop; returning identities open their existing shop. Google verifies identity only; it does not create a Supabase project or database.

1. In Google Cloud Console, configure the OAuth consent screen and create an OAuth client of type **Web application**.
2. Add `http://localhost:8080/api/v1/auth/google/callback` as an authorized redirect URI. The app and callback must use the same hostname so the first-party session cookie works.
3. Run `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\api\configure-google.ps1` from the project root. It stores the Client ID and Client Secret in `.env.local`; the secret is entered in a masked local dialog. Never commit or share this file.
4. Restart the API with the command in step 3 of Local development and reload the browser preview.

For production, register the production HTTPS callback URI with Google and set `GOOGLE_REDIRECT_URL` and `APP_BASE_URL` to HTTPS URLs on the app's hostname. Keep the Client Secret in the hosting provider's secret manager, not in source control.

## Email verification and password recovery

Email/password signup and password recovery require a working mail service. Google-created accounts can also set a password using the **Forgot password?** link on the sign-in form; the one-time link is sent only to a verified account email and lets the owner set or reset the password. Afterward, the same email and password work as an alternative to Google sign-in. For Brevo, use SMTP host `smtp-relay.brevo.com`, port `587`, your Brevo SMTP login, an SMTP key (not your Brevo account password), and a sender address verified in Brevo. Locally, run `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\api\configure-email.ps1`; it prompts for these values and saves them to ignored `.env.local`. Set the app origin to `http://localhost:5180` for local preview, or to your production HTTPS app origin when deploying. For production, put the values in the API host's secret manager; never commit or share the SMTP key. The API requires SMTP STARTTLS, stores only token hashes, and uses one-time verification links that expire after 24 hours and password-reset links that expire after 30 minutes. Without SMTP configuration, email signup and password recovery are disabled and the interface explains why.

## Shared Supabase database

The API uses PostgreSQL through the server-side `DATABASE_URL` setting. To host shop data in Supabase, create one Supabase project for the service and set the API's `DATABASE_URL` to that project's PostgreSQL connection string (use the Supabase pooler settings appropriate to the host and require TLS). Keep this value in the API host's secret manager or the ignored local `.env.local`; never expose it in browser/Vite variables. The API applies its schema at startup.

All Google-created shops use this shared database. The API associates each user with a `shop_id` and scopes retail reads and writes to that ID; the browser connects only to the API, not directly to Supabase. Supabase does not provision a separate project for each Google user. Google authentication also does not configure the database: until the deployed API is pointed at a Supabase PostgreSQL URL, its database remains whichever PostgreSQL service `DATABASE_URL` currently specifies. Do not grant browser clients direct table access; tenant isolation is enforced by the API.

## Current milestone

The app opens directly to the read-only dashboard for the shop configured by `PUBLIC_SHOP_ID`. All public dashboard data is selected for that one shop; all write operations, account records, team, settings, and other shop APIs continue to require an active session. Do not configure a client business as the public shop unless its owner has agreed that its dashboard data can be shared publicly. Account registration, verification, Google OAuth, and password recovery remain implemented in the API, but the public browser view does not show account pages. The authenticated workspace includes the shop-scoped product catalog, stock adjustments, checkout, sales history, analytics, audit logs, and settings. The browser interface adapts its navigation and catalog/cart layout for phone-sized screens.

The online edition is still being brought toward desktop feature parity. Its authenticated workspace uses the standalone app's desktop shell, brand asset, and dashboard layout as the reference; the responsive browser navigation adapts that shell on smaller screens. The current workspace includes the dashboard, point of sale, products, sales history, receipts, stock levels, daily/weekly/monthly profit analytics, Business Intelligence, an Audit Logs activity view for completed sales and recorded stock changes, and shop settings for business identity, currency, receipt details, and owner-managed staff accounts. Product management includes shop-scoped category creation, renaming, and deletion, product category assignment, editing, stock adjustment, stock-status and category filters. The online point of sale supports list/grid product selection, barcode/SKU search, cash tender/change calculation, and fixed-amount discounts for owners and managers. Discounts are validated and recorded by the API, shown on receipts and sales exports, and allocated proportionally when calculating product/category profit. Sales history and receipts load the shop's complete transaction history and support date/text filtering, CSV export, and print/PDF. Profit Analytics and Business Intelligence support period and date filters. Owners can create manager and cashier logins and activate or deactivate staff; owners cannot deactivate their own account or the shop's last active owner. Reports include category and product profitability, peak sales hours, market basket pairings, and CSV/print exports, all calculated from shop-scoped PostgreSQL data. Net profit currently equals gross profit because online operating expenses are not tracked. The online activity view is intentionally read-only and does not claim to record account/login events or product edits, which are not yet captured in the online audit trail. Desktop-specific printer, device, licensing, and local backup settings are not available online. The standalone app and its local database remain unchanged.

Products, sales, and inventory movements are always queried and written with the authenticated shop ID. Checkout obtains prices from PostgreSQL, records the sale and stock movements in one transaction, and refuses sales that exceed available stock.

Earlier local validation against PostgreSQL 18 verified schema migration and retail workflows. Email verification, sign-in blocking for unverified accounts, and password reset have automated API tests; actual SMTP delivery still requires valid mail credentials. Google OAuth state, nonce, PKCE, account-linking, and setup configuration have automated tests; live Google sign-in still requires OAuth credentials and a Google Cloud redirect URI.

This remains a development foundation, not a public-ready service until it is deployed and production settings are validated. Before public launch, add repeatable PostgreSQL integration tests, request rate limiting, hosting configuration, managed backups, monitoring, and an operational security review. No automatic synchronization with the offline desktop edition is provided.
