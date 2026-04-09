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

### Physical Device Model

Each **Mac Mini** acts as a hub that runs the BlueSend device agent. The Mac Mini's
Messages.app handles iMessage send/receive. Additional phone numbers are provided
by **iPhones** that forward their iMessages to the Mac Mini via Apple's built-in
Text Message Forwarding feature (Settings → Messages → Text Message Forwarding).

```
┌─────────────────────────────────────────────────────────┐
│                    MAC MINI HUB                          │
│  ┌─────────────────────────────────────────────────┐    │
│  │ Messages.app (signed into Apple ID #1)          │    │
│  │ - Number A (Mac Mini's own number)              │    │
│  │ - Number B (forwarded from iPhone #1)           │    │
│  │ - Number C (forwarded from iPhone #2)           │    │
│  │ All messages appear in single chat.db           │    │
│  └─────────────────────────────────────────────────┘    │
│  ┌─────────────────────────────────────────────────┐    │
│  │ BlueSend Device Agent (Go binary)               │    │
│  │ - Reads chat.db, routes by handle_id            │    │
│  │ - Sends via AppleScript                         │    │
│  │ - Reports all handles to API on connect         │    │
│  │ - WebSocket → api.blutexts.com                  │    │
│  └─────────────────────────────────────────────────┘    │
└───────────┬───────────────────┬─────────────────────────┘
            │ Text Msg Fwd      │ Text Msg Fwd
     ┌──────▼──────┐     ┌──────▼──────┐
     │  iPhone #1  │     │  iPhone #2  │
     │  Apple ID B │     │  Apple ID C │
     │  Number B   │     │  Number C   │
     └─────────────┘     └─────────────┘
```

**Key constraints:**
- Messages.app on Mac Mini signs into ONE Apple ID (provides 1 number natively)
- Each additional number requires an iPhone with its own Apple ID + Text Message Forwarding enabled
- Each iPhone with dual eSIM can provide up to 2 numbers
- iPhones must stay powered on, on WiFi, and near the Mac Mini

### How Messages Are Sent

```
BlueSend API → [send job via WebSocket to device agent]
             → Agent identifies which handle/number to send from
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
             → identifies which number received it via handle_id
             → deduplicates using imessage_guid
             → pushes to BlueSend API via WebSocket (includes from + to addresses)
             → API routes to correct account based on phone_number assignment
             → API saves to PostgreSQL, fans out to user via WebSocket
             → API triggers GHL sync (async via Redis queue)
```

### Multi-Number Routing

The device agent reports all iMessage handles (phone numbers and emails) registered
on the Mac Mini when it connects. The API maps these handles to assigned phone numbers
in the database. When a message arrives, the agent includes both `from_address` and
`to_address` (derived from `handle_id` in chat.db), allowing the API to route it
to the correct customer account.

### Daily Limit Enforcement

- Rate limit counter stored in Redis with 24h TTL keyed by `{phone_number_id}:{YYYY-MM-DD}:{contact_address}`
- New contacts tracked in `rate_limit_entries` table
- On send request: check if contact exists in `contacts` table for this phone_number
  - If NEW contact: check daily counter against limit (50). Reject if at limit.
  - If EXISTING contact (prior conversation): bypass new-contact counter entirely
- Counter resets at midnight in the account's configured timezone

---

## Scaling Plan: Zero to 50 Customers

### Hardware Math

| Component | Numbers provided |
|-----------|-----------------|
| 1 Mac Mini (own Apple ID) | 1 number |
| 1 iPhone (single SIM) | 1 number |
| 1 iPhone (dual eSIM) | 2 numbers |

Each customer gets 1 dedicated number. Each Mac Mini hub can realistically manage
**up to 10 iPhones** via Text Message Forwarding (Apple's practical limit before
Messages.app performance degrades).

So: **1 Mac Mini hub = 1 own number + up to 10 iPhones = 11-21 numbers**
(depending on single vs dual SIM iPhones).

### Scaling Tiers

#### Tier 0: Proof of Concept (1-3 customers)
```
Hardware:
  - 1× Mac Mini (M2, 8GB) .............. $599
  - 2× iPhone SE (refurbished) .......... $300 ea
  - 2× prepaid SIM / eSIM plan .......... $15/mo ea

Numbers: 3 (1 Mac Mini + 2 iPhones)
Monthly HW cost: ~$30/mo (SIM plans)
One-time: ~$1,200
```
Setup:
1. Mac Mini signed into Apple ID #1 → provides number #1
2. iPhone A signed into Apple ID #2 → Text Msg Forwarding → Mac Mini → number #2
3. iPhone B signed into Apple ID #3 → Text Msg Forwarding → Mac Mini → number #3
4. One device agent running on Mac Mini handles all 3 numbers

#### Tier 1: Early Traction (4-10 customers)
```
Hardware:
  - 1× Mac Mini ......................... (existing)
  - 7-9× iPhone SE (refurbished) ........ $300 ea
  - Use dual eSIM where possible to double up
  - USB charging hub .................... $50

Numbers: 10 (1 Mac Mini + 9 iPhones, some dual SIM → up to 19)
Monthly HW cost: ~$150/mo (SIM plans)
One-time: ~$3,500 total
```
Still one Mac Mini hub. All iPhones on a charging shelf, connected to WiFi.

#### Tier 2: Growth (11-25 customers)
```
Hardware:
  - 2× Mac Mini ......................... $599 ea
  - 15-20× iPhone SE .................... $300 ea
  - Rack shelf or colocation space
  - Managed switch + UPS

Numbers: 25 (2 Mac Minis + ~20 iPhones with dual SIM)
Monthly HW cost: ~$375/mo (SIM plans) + ~$100/mo (colo/power)
One-time: ~$8,000 total
```
Two Mac Mini hubs, each running a device agent. Split iPhones across hubs
(max 10 per hub). Both agents connect to the same BlueSend API.

#### Tier 3: Scale (26-50 customers)
```
Hardware:
  - 4× Mac Mini ......................... $599 ea
  - 30-40× iPhone SE .................... $300 ea
  - Small rack (1U Mac Mini mounts)
  - Colocation or dedicated closet

Numbers: 50 (4 Mac Minis + ~40 iPhones with dual SIM)
Monthly HW cost: ~$750/mo (SIM plans) + ~$200/mo (colo/power)
One-time: ~$16,000 total
```
Four Mac Mini hubs. Consider Mac Stadium or similar Apple hosting provider
if you don't want to manage physical hardware yourself.

### Cost Summary at Scale

| Customers | Mac Minis | iPhones | One-time HW | Monthly HW | Revenue (at $199/mo) |
|-----------|-----------|---------|-------------|------------|---------------------|
| 3         | 1         | 2       | ~$1,200     | ~$30       | $597/mo             |
| 10        | 1         | 9       | ~$3,500     | ~$150      | $1,990/mo           |
| 25        | 2         | 20      | ~$8,000     | ~$475      | $4,975/mo           |
| 50        | 4         | 40      | ~$16,000    | ~$950      | $9,950/mo           |

Break-even on hardware is achieved within 2-3 months at each tier.

### Operational Notes

- **iPhone management:** iPhones only need WiFi + power + initial Apple ID setup.
  No interaction needed after Text Message Forwarding is enabled. Keep them in
  a drawer or on a charging rack.
- **Monitoring:** The admin panel shows device status. If a Mac Mini goes offline,
  all its numbers go dark. Use a UPS and enable auto-restart after power failure.
- **Redundancy:** At Tier 2+, split high-value customers across different Mac Mini
  hubs so a single hub failure doesn't take out all numbers.
- **Apple ID management:** Each number needs a unique Apple ID. Use a naming convention
  like `blutexts-001@icloud.com`, `blutexts-002@icloud.com`, etc.
- **SIM plans:** Prepaid plans (Mint Mobile, US Mobile) at $15-25/mo per number are
  the most cost-effective. You only need the phone number for iMessage registration —
  actual data goes over WiFi.

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
