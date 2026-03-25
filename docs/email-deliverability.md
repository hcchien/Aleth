# Email Deliverability Guide

This guide covers setting up SPF, DKIM, and DMARC for Aleth's sending domain so that transactional emails (registration, email verification, password reset) land in users' inboxes rather than spam folders.

Aleth uses **Mailgun** as the email provider. All DNS steps below assume your sending domain is `mg.aleth.social` and your root domain is `aleth.social`.

---

## Overview

Three DNS records protect your email reputation:

| Record | Purpose | Where to add |
|---|---|---|
| **SPF** | Lists which servers are allowed to send mail for your domain | Root domain (`aleth.social`) or sending subdomain (`mg.aleth.social`) |
| **DKIM** | Cryptographic signature — proves mail was not tampered with in transit | Sending subdomain (`mg.aleth.social`) |
| **DMARC** | Policy that tells receiving servers what to do when SPF/DKIM fail | Root domain (`_dmarc.aleth.social`) |

---

## Step 1 — Add a sending domain in Mailgun

1. Log in to [mailgun.com](https://www.mailgun.com) → **Sending → Domains → Add New Domain**
2. Enter `mg.aleth.social` (a subdomain of your main domain — this avoids SPF conflicts with other mail you send from `aleth.social`)
3. Choose **US** or **EU** region (affects the SMTP hostname and DNS verification records)
4. Mailgun will show you a list of DNS records to add — keep this page open

---

## Step 2 — SPF record

SPF authorises Mailgun's servers to send mail on behalf of `mg.aleth.social`.

**DNS record to add:**

| Type | Host | Value | TTL |
|---|---|---|---|
| `TXT` | `mg.aleth.social` | `v=spf1 include:mailgun.org ~all` | 3600 |

> **`~all`** (softfail) is safer than `-all` (hardfail) during initial setup. Once you've confirmed deliverability, you can switch to `-all`.

**Verify with dig:**
```bash
dig TXT mg.aleth.social +short
# Should return: "v=spf1 include:mailgun.org ~all"
```

---

## Step 3 — DKIM record

DKIM adds a cryptographic signature to every outgoing email. Mailgun generates the key pair; you publish the public key in DNS.

Mailgun will show you a record like this after you add your domain:

| Type | Host | Value |
|---|---|---|
| `TXT` | `pic._domainkey.mg.aleth.social` | `k=rsa; p=MIGfMA0GCSqGSI...` (long base64 key) |

Copy the exact host and value from your Mailgun dashboard — the selector (`pic`) may differ.

**Verify with dig:**
```bash
dig TXT pic._domainkey.mg.aleth.social +short
# Should return the rsa key string
```

---

## Step 4 — MX record (Mailgun inbound, optional)

If you want Mailgun to receive bounce notifications or route inbound mail, add:

| Type | Host | Priority | Value |
|---|---|---|---|
| `MX` | `mg.aleth.social` | 10 | `mxa.mailgun.org` |
| `MX` | `mg.aleth.social` | 10 | `mxb.mailgun.org` |

> Skip this if you only need outbound sending.

---

## Step 5 — Verify domain in Mailgun

After adding all DNS records, return to **Mailgun → Sending → Domains** and click **Verify DNS Settings**. Mailgun checks all records and marks the domain as active.

DNS propagation can take up to 48 hours, but is usually under an hour.

---

## Step 6 — DMARC record

DMARC builds on SPF and DKIM to tell receiving servers what to do with emails that fail both checks. It also enables reporting so you can see who is sending mail using your domain.

**Recommended initial policy (monitoring mode):**

| Type | Host | Value | TTL |
|---|---|---|---|
| `TXT` | `_dmarc.aleth.social` | `v=DMARC1; p=none; rua=mailto:dmarc@aleth.social; ruf=mailto:dmarc@aleth.social; fo=1` | 3600 |

| Tag | Value | Meaning |
|---|---|---|
| `p=none` | Monitor only | Do not reject or quarantine — safe for initial rollout |
| `rua` | Aggregate reports | Daily XML reports of all emails sent from your domain |
| `ruf` | Forensic reports | Per-message failure reports (may contain headers) |
| `fo=1` | Failure options | Send report if either SPF or DKIM fails |

**Verify with dig:**
```bash
dig TXT _dmarc.aleth.social +short
# Should return: "v=DMARC1; p=none; ..."
```

### Tightening DMARC over time

Once you've received DMARC aggregate reports for 2–4 weeks and confirmed only Mailgun is sending mail on your behalf:

1. Move to quarantine: `p=quarantine; pct=50` (quarantine 50% of failing mail)
2. Then full enforcement: `p=reject` (reject all failing mail)

```
v=DMARC1; p=reject; rua=mailto:dmarc@aleth.social; fo=1
```

---

## Step 7 — Configure Aleth to use Mailgun SMTP

Set these env vars on the **Auth service**:

```dotenv
AUTH_SMTP_HOST=smtp.mailgun.org
AUTH_SMTP_PORT=587
AUTH_SMTP_USER=postmaster@mg.aleth.social
AUTH_SMTP_PASSWORD=<your-mailgun-smtp-password>
AUTH_SMTP_FROM=noreply@aleth.social
```

The SMTP password is found in Mailgun under **Sending → Domain settings → SMTP credentials**.

> Use port **587** with STARTTLS (not 465 SSL, not 25). Mailgun recommends 587.

---

## Step 8 — Send a test email

```bash
# Using swaks (Swiss Army Knife for SMTP)
swaks \
  --to your@email.com \
  --from noreply@aleth.social \
  --server smtp.mailgun.org:587 \
  --auth LOGIN \
  --auth-user postmaster@mg.aleth.social \
  --auth-password "<your-mailgun-smtp-password>" \
  --tls \
  --header "Subject: Aleth SMTP test"

# Or trigger a real registration flow in staging:
curl -X POST https://staging.aleth.social/graphql \
  -H 'Content-Type: application/json' \
  -d '{"query":"mutation { register(input:{username:\"testuser\",email:\"your@email.com\",password:\"Test1234!\"}) { user { id } } }"}'
```

Then verify the email:
1. Check it arrived in your inbox (not spam)
2. Inspect the full email headers — look for `Authentication-Results` showing `dkim=pass` and `spf=pass`
3. Use [mail-tester.com](https://www.mail-tester.com) for a deliverability score (aim for ≥ 9/10)

---

## Step 9 — Unsubscribe header (CAN-SPAM / GDPR)

For transactional email Mailgun adds `List-Unsubscribe` headers automatically. Verify this is enabled in your Mailgun domain settings under **Tracking** → **Unsubscribe tracking: On**.

---

## Full DNS summary

Add all of these records to your DNS provider (Cloudflare, Route 53, etc.):

| Type | Host | Value | TTL | Purpose |
|---|---|---|---|---|
| `TXT` | `mg.aleth.social` | `v=spf1 include:mailgun.org ~all` | 3600 | SPF |
| `TXT` | `pic._domainkey.mg.aleth.social` | `k=rsa; p=<key from Mailgun>` | 3600 | DKIM |
| `MX` | `mg.aleth.social` | `mxa.mailgun.org` (priority 10) | 3600 | Inbound |
| `MX` | `mg.aleth.social` | `mxb.mailgun.org` (priority 10) | 3600 | Inbound |
| `TXT` | `_dmarc.aleth.social` | `v=DMARC1; p=none; rua=mailto:dmarc@aleth.social; fo=1` | 3600 | DMARC |

> The exact DKIM host and value come from your Mailgun dashboard — copy them exactly.

---

## Troubleshooting

### Email goes to spam

1. Check `Authentication-Results` in the email headers — all three (`dkim`, `spf`, `dmarc`) should show `pass`
2. Run your domain through [MXToolbox](https://mxtoolbox.com/SuperTool.aspx) to check for blacklist entries
3. Ensure `AUTH_SMTP_FROM` (`noreply@aleth.social`) matches your verified sending domain or a subdomain

### Mailgun "domain not verified" error

- DNS propagation can take up to 48 hours
- Run `dig TXT mg.aleth.social +short` to confirm the SPF record is live before clicking Verify in Mailgun

### DMARC reports not arriving

- Ensure the `rua` email address (`dmarc@aleth.social`) is a real inbox or aliased to one
- Use a DMARC report analyser like [dmarcian.com](https://dmarcian.com) to parse the XML reports

### 587 port blocked

If your Cloud Run environment blocks outbound port 587:
- Use Mailgun's API instead of SMTP (update the auth service's mailer implementation)
- Or request the egress firewall rule allow port 587 to `smtp.mailgun.org`

---

## Deliverability checklist

- [ ] Mailgun domain `mg.aleth.social` added and verified
- [ ] SPF `TXT` record published at `mg.aleth.social`
- [ ] DKIM `TXT` record published (selector from Mailgun dashboard)
- [ ] MX records added (optional, for inbound/bounces)
- [ ] DMARC `TXT` record published at `_dmarc.aleth.social` (start with `p=none`)
- [ ] Auth service env vars set (`AUTH_SMTP_*`)
- [ ] Test email received in inbox (not spam)
- [ ] Email headers confirm `dkim=pass spf=pass`
- [ ] [mail-tester.com](https://www.mail-tester.com) score ≥ 9/10
- [ ] DMARC reports received and reviewed after 1 week
- [ ] DMARC policy tightened to `p=reject` after clean reports
