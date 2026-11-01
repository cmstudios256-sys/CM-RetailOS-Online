# CM RetailOS Online

CM RetailOS Online is a free, open-source retail workspace. The Vue browser client can be used without creating an account; Google is the only sign-in flow presented by the app.

## Guest and demo mode

- First visits open a seeded **Demo Shop** with obviously fictional products, sales, and amounts. Public API demo routes return static fixtures and never query a shop or tenant table.
- The complete workspace remains usable while disconnected. Guest products, categories, sales, stock movements, and settings are stored in browser `localStorage` under a stable random guest-device UUID.
- The header shows a **Demo** badge and **Not connected** status while using the seeded demo. The **Connect Google** button and **Save to cloud** menu action start the same OAuth flow.
- On the OAuth return, the app restores the active page, silently checks the session cookie, and posts the guest bundle to the authenticated claim endpoint. After a successful claim, the local transactional bundle is cleared so a later sign-out does not show stale cloud data.
- Sign out clears the session cookie only. Cloud records are not deleted; the browser returns to its local guest workspace.
- The browser does not show email/password sign-in or signup forms. Legacy email/password API routes remain in the backend but are not used by the browser app.

## Project layout and local development

- `web/` - Vue 3 + TypeScript client.
- `api/` - Go HTTP API and PostgreSQL schema.
- `api/internal/httpapi/schema.sql` - idempotent database schema applied at API startup.

Requirements for cloud features: Go 1.25+, Node.js 20+, npm, PostgreSQL 15+, and a Google OAuth web client.

1. For guest-only browsing, run `npm install` and `npm run dev` in `web/`. The demo and guest writes work even when the API is unavailable.
2. For cloud features, run `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\api\configure-local.ps1` from the project root to configure a local PostgreSQL connection.
3. Configure Google OAuth with `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\api\configure-google.ps1`. For local development, the callback is `http://localhost:8080/api/v1/auth/google/callback` and the app origin is `http://localhost:5180`.
4. Start the API with `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\api\run-local.ps1`, then start the web client with `npm run dev` in `web/`.

The Vite server proxies `/api` requests to `http://127.0.0.1:8080`. OAuth app and callback URLs must use the same hostname so the first-party session cookie is returned to the browser. Do not commit `.env.local`; production secrets belong in the API host's secret manager. Production requires HTTPS, secure cookies, database backups, rate limiting, and monitoring.

## Guest claim contract

`POST /api/v1/account/claim` requires a valid owner session cookie. The browser sends the following JSON bundle (the request is limited to 1 MiB):

```json
{
  "schemaVersion": 1,
  "guestDeviceId": "UUID",
  "idempotencyKey": "UUID",
  "products": [
    {
      "id": 1,
      "sku": "ITEM-1",
      "name": "Sample Item",
      "category": "Sample Category",
      "barcode": "",
      "unit": "piece",
      "sellingPrice": 10,
      "costPrice": 5,
      "quantity": 3,
      "minStock": 1,
      "image": ""
    }
  ],
  "categories": ["Sample Category"],
  "sales": [
    {
      "id": "guest-sale-id",
      "cashierName": "Shop Owner",
      "customerName": "",
      "subtotal": 10,
      "discount": 0,
      "total": 10,
      "paymentMethod": "cash",
      "completedAt": "2026-01-01T10:00:00Z",
      "items": [
        {
          "productId": 1,
          "productName": "Sample Item",
          "quantity": 1,
          "unitPrice": 10,
          "unitCost": 5,
          "lineTotal": 10
        }
      ]
    }
  ],
  "stockMovements": [
    {
      "id": "guest-movement-id",
      "productId": 1,
      "quantityDelta": -1,
      "reason": "Sale",
      "saleId": "guest-sale-id",
      "createdAt": "2026-01-01T10:00:00Z"
    }
  ],
  "settings": {
    "shopName": "My Shop",
    "currency": "UGX",
    "address": "",
    "phone": "",
    "receiptHeader": "",
    "receiptFooter": "Thank you for shopping with us!",
    "receiptShowPhone": false,
    "preferences": {}
  }
}
```

A new import returns `201 Created`; an exact retry with the same device UUID, idempotency UUID, and payload returns `200 OK`. Both return:

```json
{
  "imported": {
    "products": 1,
    "categories": 1,
    "sales": 1,
    "stockMovements": 1
  },
  "productIds": { "1": 123 },
  "saleIds": { "guest-sale-id": "cloud-sale-uuid" }
}
```

Each claim is one database transaction. A stored SHA-256 payload hash prevents reuse of an idempotency key with changed data; such a request returns `409 Conflict`. New cloud IDs are returned for local product and sale IDs. Duplicate category names are ignored. Duplicate product SKU/barcode conflicts fail and roll back the entire claim. Products are imported with their current quantities; stock movements are imported as history and do not adjust quantity again. The owner who completed OAuth is recorded as cashier/user for migrated records. Team-member accounts and printer jobs are not part of this claim bundle.

## Verify the full local flow

1. Start PostgreSQL, the API, and Vite using the steps above. Open the app; it should show **Demo Shop**, **Demo**, and **Not connected** without an auth page.
2. Add a product, adjust its stock, create a category, and complete a sale. Reload the page and confirm the local records remain.
3. Select **Connect Google** or **Save to cloud** and complete Google consent. The browser returns to the page you were using; the status changes to **Connected as <email>** after the guest claim succeeds.
4. Inspect the shop using the authenticated API/database and confirm the products, category, sale, movement history, and settings were imported. Sign out; the cookie is cleared, cloud data remains, and the browser opens local guest mode.

## Data isolation

The app's public demo routes serve only static fake fixtures. User shop APIs require the signed, HTTP-only session cookie and scope queries by its `shop_id`. The browser never connects directly to PostgreSQL or Supabase. Google OAuth identifies the user; the API creates/links the account and shop and stores tenant data in the configured PostgreSQL database. The shared database is not a separate database per Google account.
