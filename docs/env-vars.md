# Environment Variables Reference

Complete reference for all environment variables required to deploy Aleth.

---

## Auth Service

| Variable | Required | Default | Description |
|---|---|---|---|
| `AUTH_DATABASE_URL` | ✅ | — | PostgreSQL connection string, e.g. `postgres://user:pass@host:5432/auth_db` |
| `AUTH_ACCESS_TOKEN_SECRET` | ✅ | — | Secret for signing JWT access tokens. Use a 64-char random hex string. |
| `AUTH_REFRESH_TOKEN_SECRET` | ✅ | — | Secret for signing JWT refresh tokens. Must differ from access token secret. |
| `AUTH_PORT` | | `8081` | Listening port |
| `AUTH_ACCESS_TOKEN_TTL` | | `15m` | Access token lifetime |
| `AUTH_REFRESH_TOKEN_TTL` | | `168h` | Refresh token lifetime (7 days) |
| `AUTH_PASSKEY_RP_ID` | | `localhost` | WebAuthn relying party ID — must match your public domain, e.g. `aleth.social` |
| `AUTH_OAUTH_CALLBACK_BASE` | | `http://localhost:8081` | Public base URL of the auth service, used for OAuth redirect URIs |
| `AUTH_FRONTEND_URL` | | `http://localhost:3000` | Public base URL of the Next.js app, used for email links (password reset, email verification) |
| `AUTH_GOOGLE_CLIENT_ID` | | — | Google OAuth client ID (for Google login) |
| `AUTH_FACEBOOK_APP_ID` | | — | Facebook App ID (for Facebook login) |
| `AUTH_FACEBOOK_CLIENT_SECRET` | | — | Facebook App Secret |
| `AUTH_TWITTER_CLIENT_ID` | | — | Twitter OAuth 2.0 client ID (reputation stamps) |
| `AUTH_TWITTER_CLIENT_SECRET` | | — | Twitter OAuth 2.0 client secret |
| `AUTH_INSTAGRAM_CLIENT_ID` | | — | Instagram Basic Display API client ID |
| `AUTH_INSTAGRAM_CLIENT_SECRET` | | — | Instagram client secret |
| `AUTH_LINKEDIN_CLIENT_ID` | | — | LinkedIn OAuth 2.0 client ID |
| `AUTH_LINKEDIN_CLIENT_SECRET` | | — | LinkedIn client secret |
| `AUTH_SMTP_HOST` | | `localhost` | SMTP relay hostname, e.g. `smtp.mailgun.org` |
| `AUTH_SMTP_PORT` | | `587` | SMTP port (587 = STARTTLS, 465 = TLS) |
| `AUTH_SMTP_USER` | | — | SMTP username / API key |
| `AUTH_SMTP_PASSWORD` | | — | SMTP password |
| `AUTH_SMTP_FROM` | | `noreply@aleth.social` | "From" address for transactional emails |

> **Tip — generate secrets:**
> ```bash
> openssl rand -hex 32   # 64-char hex, good for token secrets
> ```

---

## Content Service

| Variable | Required | Default | Description |
|---|---|---|---|
| `CONTENT_DATABASE_URL` | ✅ | — | PostgreSQL connection string for content DB |
| `CONTENT_ACCESS_TOKEN_SECRET` | ✅ | — | Must match `AUTH_ACCESS_TOKEN_SECRET` |
| `CONTENT_PORT` | | `8082` | Listening port |
| `CONTENT_SIGNING_SECRET` | | _(same as access secret)_ | Secret for content signature verification. Set if you want a separate key. |
| `CONTENT_FEDERATION_URL` | | — | URL of the federation service. Set to enable ActivityPub fan-out on new posts. |
| `CONTENT_PUBSUB_ENABLED` | | `false` | Set to `true` to publish content events to GCP Pub/Sub |
| `CONTENT_PUBSUB_PROJECT_ID` | | — | GCP project ID (required when `CONTENT_PUBSUB_ENABLED=true`) |
| `CONTENT_PUBSUB_TOPIC` | | `content-events` | Pub/Sub topic name |

---

## Feed Service

| Variable | Required | Default | Description |
|---|---|---|---|
| `FEED_CONTENT_DATABASE_URL` | ✅ | — | PostgreSQL connection string for content DB (read queries) |
| `FEED_AUTH_DATABASE_URL` | ✅ | — | PostgreSQL connection string for auth DB (follow graph) |
| `FEED_ACCESS_TOKEN_SECRET` | ✅ | — | Must match `AUTH_ACCESS_TOKEN_SECRET` |
| `FEED_PORT` | | `8083` | Listening port |

---

## Gateway Service

| Variable | Required | Default | Description |
|---|---|---|---|
| `GATEWAY_PORT` | | `8080` | Listening port (public-facing) |
| `GATEWAY_AUTH_SERVICE_URL` | | `http://localhost:8081` | Internal URL of the auth service |
| `GATEWAY_CONTENT_SERVICE_URL` | | `http://localhost:8082` | Internal URL of the content service |
| `GATEWAY_FEED_SERVICE_URL` | | `http://localhost:8083` | Internal URL of the feed service |
| `GATEWAY_NOTIFICATION_URL` | | `http://localhost:8086` | Internal URL of the notification service |
| `GATEWAY_FEDERATION_URL` | | `http://localhost:8087` | Internal URL of the federation service |
| `GATEWAY_ACCESS_TOKEN_SECRET` | | — | Optional — set to validate tokens at the gateway level |
| `GATEWAY_ALLOWED_ORIGIN` | | `*` | CORS allowed origin. In production set to your frontend URL, e.g. `https://aleth.social` |

---

## Notification Service

| Variable | Required | Default | Description |
|---|---|---|---|
| `NOTIFICATION_DATABASE_URL` | ✅ | — | PostgreSQL connection string for notifications DB |
| `NOTIFICATION_ACCESS_TOKEN_SECRET` | ✅ | — | Must match `AUTH_ACCESS_TOKEN_SECRET` |
| `NOTIFICATION_PORT` | | `8086` | Listening port |
| `NOTIFICATION_CONTENT_DATABASE_URL` | | — | PostgreSQL connection string for content DB. Required to enable page-follower fan-out. |
| `NOTIFICATION_PUBSUB_PROJECT_ID` | | — | GCP project ID. Set to enable Pub/Sub subscriber for content events. |
| `NOTIFICATION_PUBSUB_SUBSCRIPTION` | | `notification-content-events` | Pub/Sub subscription name |

---

## Federation Service (ActivityPub)

| Variable | Required | Default | Description |
|---|---|---|---|
| `FEDERATION_DATABASE_URL` | ✅ | — | PostgreSQL connection string for federation DB |
| `FEDERATION_DOMAIN` | ✅ | — | Your bare domain, e.g. `aleth.social` (no scheme, no trailing slash) |
| `FEDERATION_PLATFORM_KEY_SECRET` | ✅ | — | 32-byte hex string for AES-256-GCM encryption of actor private keys. Generate with `openssl rand -hex 32` |
| `FEDERATION_PORT` | | `8084` | Listening port |
| `FEDERATION_AUTH_URL` | | `http://localhost:8081` | Internal URL of the auth service |
| `FEDERATION_CONTENT_URL` | | `http://localhost:8082` | Internal URL of the content service |
| `FEDERATION_SKIP_SIG_VERIFY` | | `false` | Skip HTTP Signature verification on inbox requests. **Dev only — never set in production.** |

---

## Counter Service

| Variable | Required | Default | Description |
|---|---|---|---|
| `COUNTER_PORT` | | `8085` | Listening port |
| `COUNTER_DATABASE_URL` | | — | PostgreSQL connection string for counter DB |
| `COUNTER_PUBSUB_PROJECT_ID` | | — | GCP project ID for Pub/Sub |
| `COUNTER_PUBSUB_SUBSCRIPTION` | | — | Pub/Sub subscription name |

---

## Next.js Web App (`apps/web`)

### Server-side (not exposed to browser)

| Variable | Required | Default | Description |
|---|---|---|---|
| `GATEWAY_URL` | ✅ | `http://localhost:4000` | Internal URL of the gateway service — used for SSR GraphQL rewrites |
| `NOTIFICATION_URL` | | `http://localhost:8086` | Internal URL of the notification service — used by the `/api/notifications` proxy route |
| `GCS_BUCKET` | | — | Google Cloud Storage bucket name for image uploads. Falls back to local disk if unset. |
| `GCS_KEY_FILE` | | — | Path to a GCS service account JSON key file. Uses Application Default Credentials (ADC) if unset. |

### Public (exposed to browser, must be prefixed `NEXT_PUBLIC_`)

| Variable | Required | Default | Description |
|---|---|---|---|
| `NEXT_PUBLIC_API_URL` | | `/graphql` | Client-side GraphQL endpoint. Override for cross-origin setups. |
| `NEXT_PUBLIC_GOOGLE_CLIENT_ID` | | — | Google OAuth client ID (same as `AUTH_GOOGLE_CLIENT_ID`) |
| `NEXT_PUBLIC_FACEBOOK_APP_ID` | | — | Facebook App ID (same as `AUTH_FACEBOOK_APP_ID`) |

---

## Shared Secrets Across Services

Several secrets must be **identical** across services:

| Secret | Services that share it |
|---|---|
| `AUTH_ACCESS_TOKEN_SECRET` | Auth, Content, Feed, Notification, Gateway |

Set it once in a secrets manager and inject it into each service's environment.

---

## GCP Pub/Sub — Event Flow

When Pub/Sub is enabled, events flow like this:

```
Content Service  →  [content-events topic]  →  Notification Service (subscription)
                                             →  Counter Service (subscription)
```

All three need the same GCP project and Pub/Sub must be pre-created:

```bash
# Create topic
gcloud pubsub topics create content-events

# Create subscriptions
gcloud pubsub subscriptions create notification-content-events \
  --topic=content-events
gcloud pubsub subscriptions create counter-content-events \
  --topic=content-events
```

---

## Minimal Production Checklist

These are the absolute minimum variables needed to run a working deployment (no social OAuth, no federation):

```dotenv
# Auth
AUTH_DATABASE_URL=postgres://...
AUTH_ACCESS_TOKEN_SECRET=<64-char hex>
AUTH_REFRESH_TOKEN_SECRET=<64-char hex>
AUTH_PASSKEY_RP_ID=aleth.social
AUTH_FRONTEND_URL=https://aleth.social
AUTH_OAUTH_CALLBACK_BASE=https://auth.aleth.social
AUTH_SMTP_HOST=smtp.mailgun.org
AUTH_SMTP_PORT=587
AUTH_SMTP_USER=postmaster@mg.aleth.social
AUTH_SMTP_PASSWORD=<mailgun smtp key>
AUTH_SMTP_FROM=noreply@aleth.social

# Content
CONTENT_DATABASE_URL=postgres://...
CONTENT_ACCESS_TOKEN_SECRET=<same as AUTH_ACCESS_TOKEN_SECRET>

# Feed
FEED_CONTENT_DATABASE_URL=postgres://...
FEED_AUTH_DATABASE_URL=postgres://...
FEED_ACCESS_TOKEN_SECRET=<same as AUTH_ACCESS_TOKEN_SECRET>

# Gateway
GATEWAY_AUTH_SERVICE_URL=http://auth:8081
GATEWAY_CONTENT_SERVICE_URL=http://content:8082
GATEWAY_FEED_SERVICE_URL=http://feed:8083
GATEWAY_NOTIFICATION_URL=http://notification:8086
GATEWAY_ALLOWED_ORIGIN=https://aleth.social

# Notification
NOTIFICATION_DATABASE_URL=postgres://...
NOTIFICATION_ACCESS_TOKEN_SECRET=<same as AUTH_ACCESS_TOKEN_SECRET>

# Next.js
GATEWAY_URL=http://gateway:8080
NEXT_PUBLIC_API_URL=/graphql
GCS_BUCKET=aleth-uploads
```
