# BlueSend — Technical Architecture

## Brand Identity

**Product Name:** BlueSend  
**Tagline:** "iMessage for business. Finally."  
**Logo Concept:** A bold blue speech bubble with a subtle upward-arrow (send indicator) embedded in the tail, set in SF Pro Display weight. Color: #007AFF (Apple iMessage blue). The bubble has rounded corners matching Apple's HIG. Wordmark in clean sans-serif alongside.  
**Positioning:** Premium, Apple-native CRM infrastructure. Not a marketing blast tool — a relationship layer for high-value sales and support teams.

---

## System Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        CUSTOMER LAYER                           │
│  Browser → Next.js 14 (Vercel / self-hosted)                   │
│  - Marketing site, onboarding, user dashboard, admin panel     │
└──────────────────────────┬──────────────────────────────────────┘
                           │ HTTPS / WSS
┌──────────────────────────▼──────────────────────────────────────┐
│                      API GATEWAY (Nginx)                        │
│  TLS termination, rate limiting, path routing                  │
└──────────┬──────────────────────────────┬───────────────────────┘
           │                              │
┌──────────▼───────────┐    ┌────────────▼────────────────────────┐
│   BlueSend API (Go)  │    │     WebSocket Hub (Go)              │
│   Chi router         │    │  Real-time message/status events    │
│   Port 8080          │    │  Port 8081                          │
└──────────┬───────────┘    └────────────┬────────────────────────┘
           │                              │
┌──────────▼──────────────────────────────▼───────────────────────┐
│                      DATA LAYER                                 │
│  PostgreSQL 16 (primary)    Redis 7 (cache + pub/sub + queues) │
└──────────┬──────────────────────────────────────────────────────┘
           │
┌──────────▼──────────────────────────────────────────────────────┐
│                 PHYSICAL DEVICE LAYER                           │
│  Mac Mini / iPhone running BlueSend Device Agent (Go daemon)   │
│  - Interfaces with Messages.app via AppleScript / SQLite       │
│  - Maintains persistent WebSocket connection to BlueSend API   │
│  - One agent instance per device, handles N phone numbers      │
└──────────┬──────────────────────────────────────────────────────┘
           │ Bidirectional WebSocket (wss://devices.bluesend.io)
┌──────────▼──────────────────────────────────────────────────────┐
│               EXTERNAL INTEGRATIONS                             │
│  ┌──────────────────┐  ┌──────────────┐  ┌───────────────────┐ │
│  │  Go High Level   │  │   Stripe     │  │  Apple Messages   │ │
│  │  API v2 (OAuth)  │  │  Billing     │  │  Infrastructure   │ │
│  └──────────────────┘  └──────────────┘  └───────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

---

## Tech Stack

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| API Backend | Go 1.22 + Chi router | High concurrency, low memory, ideal for real-time device connections |
| Database | PostgreSQL 16 | ACID compliance, JSONB for GHL metadata, full-text search on messages |
| Cache / Pub-Sub / Queue | Redis 7 | Rate limit counters, WebSocket fan-out, async job queues (asynq) |
| Frontend | Next.js 14 (App Router) | Server components, streaming, excellent DX |
| Styling | Tailwind CSS + shadcn/ui | Rapid Apple-inspired UI construction |
| Auth | JWT (RS256) + refresh tokens | Stateless, device agent compatible |
| Payments | Stripe Subscriptions | Industry standard, excellent webhook system |
| Device Agent | Go binary (macOS) | Minimal footprint, cross-compile to arm64/amd64 |
| iMessage Bridge | AppleScript + Messages SQLite DB | Only reliable method without private APIs |
| Job Queue | asynq (Redis-backed) | Reliable task processing with retries |
| Observability | Prometheus + Grafana | Self-hosted metrics |
| Deployment | Docker Compose (dev) / K8s (prod) | |

---

## Database Schema

### Core Tables

```sql
-- Accounts (SaaS customers / businesses)
accounts: id, name, email, stripe_customer_id, stripe_subscription_id,
          plan, status, ghl_location_id, ghl_access_token, ghl_refresh_token,
          setup_complete, created_at, updated_at

-- Users (people who log in to the dashboard)
users: id, account_id, email, password_hash, role (owner/member/admin),
       last_login_at, created_at

-- Phone numbers assigned to accounts
phone_numbers: id, account_id, device_id, number, display_name,
               status (active/provisioning/suspended), daily_new_contact_count,
               daily_reset_at, created_at

-- Physical devices (Mac Minis / iPhones)
devices: id, name, type (mac_mini/iphone), serial_number, device_token,
         last_seen_at, status (online/offline/error), ip_address,
         firmware_version, capacity (max numbers), assigned_count, created_at

-- Contacts
contacts: id, account_id, phone_number_id, address (phone/email for iMessage),
          name, ghl_contact_id, first_message_at, last_message_at,
          message_count, is_new_contact, created_at

-- Conversations
conversations: id, account_id, phone_number_id, contact_id,
               ghl_conversation_id, last_message_at, message_count, created_at

-- Messages
messages: id, conversation_id, account_id, phone_number_id, contact_id,
          direction (inbound/outbound), content, imessage_guid,
          delivered_at, read_at, failed_at, error_message,
          ghl_message_id, created_at

-- Rate limiting
rate_limit_entries: id, phone_number_id, contact_address, date,
                    message_count, is_new_contact, created_at

-- Billing events (Stripe webhook log)
billing_events: id, account_id, stripe_event_id, event_type,
                payload (JSONB), processed_at, created_at
```

---

## iMessage Device Architecture

### How Messages Are Sent

```
BlueSend API → [send job to Redis queue]
             → Device Agent polls queue / receives via WebSocket push
             → Agent calls AppleScript:
               tell application "Messages"
                 send "content" to buddy "recipient" of service "iMessage"
               end tell
             → Agent monitors Messages.app SQLite DB for delivery status
             → Status update pushed back to API via WebSocket
```

### How Messages Are Received

```
Device Agent → polls ~/Library/Messages/chat.db every 500ms
             → detects new rows in message table WHERE is_from_me = 0
             → deduplicates using imessage_guid
             → pushes to BlueSend API via WebSocket
             → API saves to PostgreSQL, fans out to user via WebSocket
             → API triggers GHL sync (async via Redis queue)
```

### Daily Limit Enforcement

- Rate limit counter stored in Redis with 24h TTL keyed by `{phone_number_id}:{YYYY-MM-DD}:{contact_address}`
- New contacts tracked in `rate_limit_entries` table
- On send request: check if contact exists in `contacts` table for this phone_number
  - If NEW contact: check daily counter against limit (50). Reject if at limit.
  - If EXISTING contact (prior conversation): bypass new-contact counter entirely
- Counter resets at midnight in the account's configured timezone

---

## GHL Marketplace App Integration

### OAuth Flow
1. User completes BlueSend signup and payment
2. BlueSend initiates GHL OAuth redirect to `https://marketplace.gohighlevel.com/oauth/chooselocation`
3. GHL redirects back to `https://app.bluesend.io/oauth/ghl/callback?code=xxx&locationId=yyy`
4. BlueSend exchanges code for access + refresh tokens
5. BlueSend stores tokens, creates custom channel in GHL location

### Bidirectional Sync Architecture
- **Outbound (BlueSend → GHL):** After message saved to DB, asynq job calls GHL Conversations API to create/update message in the matching conversation thread
- **Inbound (GHL → BlueSend):** GHL webhook fires on new message → BlueSend API validates, routes to device agent for send
- **Contact sync:** On new contact created in BlueSend, create in GHL Contacts API. On GHL contact webhook, update BlueSend contacts table.

---

## Stripe Billing Integration

| Plan | Price ID | Amount |
|------|----------|--------|
| Setup Fee (one-time) | `price_setup` | $399 |
| Monthly | `price_monthly` | $199/mo |
| Annual | `price_annual` | $2,600/yr |

### Signup Flow
1. Collect customer info → create Stripe Customer
2. Present Stripe Payment Element (embedded)
3. On payment confirmation → POST `/api/webhooks/stripe` fires `payment_intent.succeeded`
4. API provisions: account record, GHL sub-account, phone number assignment
5. User lands on dashboard, sees "Setting Up..." status
6. Background worker completes provisioning, updates status to "Active"

### Webhook Events Handled
- `payment_intent.succeeded` → initial setup + subscription creation
- `invoice.paid` → monthly/annual renewal confirmation
- `invoice.payment_failed` → dunning: email + status → `past_due`
- `customer.subscription.deleted` → status → `cancelled`, soft-lock account
- `customer.subscription.updated` → plan changes

---

## Deployment Architecture

### Development
```
docker-compose up
  - postgres:16
  - redis:7-alpine
  - api (Go, hot reload with Air)
  - web (Next.js dev server)
```

### Production
```
Kubernetes (or Docker Swarm):
  - api (2 replicas minimum)
  - web (Vercel recommended for zero-config SSR)
  - postgres (RDS or Supabase)
  - redis (ElastiCache or Upstash)
  - nginx ingress

Device Layer:
  - Each Mac Mini/iPhone runs device-agent binary
  - Agent auto-starts via launchd on macOS
  - Connects outbound to wss://devices.bluesend.io
  - No inbound ports needed on device (NAT-friendly)
```

---

## Iterative Build Order

**Phase 1 (Foundation) — This Session**
- [x] Project scaffolding + architecture
- [x] Docker + infrastructure setup
- [x] Database migrations (full schema)
- [x] Go API: config, models, auth, core handlers
- [x] Stripe integration: checkout, webhooks, billing portal
- [x] GHL integration: OAuth, contact/message sync
- [x] Message rate limiting service
- [x] WebSocket hub
- [x] Device agent: iMessage bridge, sync
- [x] Next.js: landing page, signup/onboarding, user dashboard, admin panel

**Phase 2 (Hardening)**
- [ ] Email notifications (Resend)
- [ ] Prometheus metrics + Grafana dashboards
- [ ] Full admin device management UI
- [ ] CSV export + message search
- [ ] Dunning email sequences
- [ ] GHL marketplace app submission config
