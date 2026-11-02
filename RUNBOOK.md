# CM RetailOS Operations Runbook

## Backups (Supabase/PostgreSQL)

Create an encrypted, access-controlled backup:

```sh
pg_dump "$DATABASE_URL" --format=custom --no-owner --file=cm-retailos-$(date -u +%Y%m%d-%H%M%SZ).dump
```

- Schedule automated backups at least daily, plus before migrations and releases.
- Retain 30 daily backups, 12 monthly backups, and at least one backup outside the production region/account.
- Restore into a non-production database and verify row counts before promotion:

```sh
pg_restore --clean --if-exists --no-owner --dbname="$DATABASE_URL" cm-retailos.dump
```

## Secret rotation

Rotate `JWT_SECRET`, Google OAuth credentials, and SMTP credentials through their secret managers. Deploy database credentials first and application secrets without values appearing in logs. After rotating `JWT_SECRET`, explicitly expect all existing browser sessions to be invalidated. Rotate one credential at a time and verify health, Google sign-in, and password reset.

## Monitoring

- Alert on health endpoint non-200 for 2 consecutive checks.
- Monitor database connection failures, migration failures, 5xx and 429 rates, and process restarts.
- Alert on expired certificates, OAuth callback failures, SMTP failures, and backup age over 36 hours.
- Track tenant count, failed logins, claim conflicts, stock-adjustment 403s, and p95 API latency.

## Incidents

1. Preserve timestamps and redact credentials, cookies, personal data, and token values.
2. Rate limit or disable affected routes before investigating code.
3. Use the most recent known-good deployment and restore PostgreSQL from backup only for confirmed data loss.
4. Rotate exposed secrets, revoke OAuth/session tokens where applicable, and invalidate sessions if JWT secrecy may be compromised.
5. Communicate status using a neutral incident channel; document scope, detection, containment, recovery, and follow-up owner.
