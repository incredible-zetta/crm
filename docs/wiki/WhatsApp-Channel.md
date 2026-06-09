# WhatsApp Channel

The WhatsApp channel provides two-way communication with contacts via a self-hosted WhatsApp gateway (go-whatsapp-web-multidevice). It mirrors the email inbox pattern for inbound messages and adds outbound send, capability audit, and delivery/read tracking.

## Architecture

```
WhatsApp Gateway (go-whatsapp-web-multidevice)
  ↓ HTTP REST API
WhatsApp Adapter (internal/adapter/whatsapp)
  ↓ port.WhatsAppGateway
Smart Sender (rate-limit + jitter + daily cap + warmup)
  ↓
WhatsAppService (internal/service/whatsapp_service.go)
  ↓
MCP Tools + HTTP Webhook
```

## Features

### Outbound
- **Send text messages** with WhatsApp markdown formatting
- **Smart-send policy** to prevent account bans:
  - Token-bucket rate limiting
  - Randomized jitter between messages
  - Per-recipient daily cap
  - Global warmup ramp
  - Block sends to numbers verified not on WhatsApp
- **Delivery tracking**: sent → delivered → read

### Inbound (Webhook)
- **Receive messages** from contacts (text + media)
- **Auto-link to known contacts** by phone number
- **Media retrieval**: images, videos, documents, audio, stickers
- **Reply threading**: link outbound replies to inbound messages
- **Read receipts**: mark messages as read on the gateway

### Capability Audit
- **Check phone registration**: verify if a number is on WhatsApp
- **Batch audit**: check a segment of contacts and persist verdicts
- **Contact field**: `whatsapp_status` (unknown/registered/not_registered)

## Configuration

### Required Environment Variables

```bash
# Gateway connection
WA_BASE_URL=https://notification.dev.lazyindra.online
WA_BASIC_AUTH=<base64-encoded "user:pass">
WA_DEVICE_ID=cds

# Webhook secret (HMAC-SHA256 validation, optional but recommended)
WA_WEBHOOK_SECRET=your-secret-here
```

### Smart-Send Policy (Optional)

```bash
# Token-bucket rate limiting
WA_SEND_MAX=10          # max sends per window
WA_SEND_WINDOW_SEC=60   # window in seconds

# Per-recipient daily cap
WA_SEND_DAILY_CAP=5     # max messages per contact per 24h

# Global warmup ramp
WA_WARMUP_PER_DAY=50    # max total sends per 24h (increase as account ages)

# Human-like jitter
WA_SEND_JITTER_MIN_MS=1000  # min delay before send (ms)
WA_SEND_JITTER_MAX_MS=3000  # max delay before send (ms)

# Safety checks
WA_BLOCK_UNREGISTERED_SEND=true  # refuse sends to numbers verified not on WA
```

## Webhook Setup

The gateway must be configured to POST webhook events to your CRM:

```
POST /wa/webhook
```

### Event Types

1. **`message`** - Inbound message from a contact
   - Includes text body, media attachments, quoted message ID
   - Auto-linked to known contacts by phone number

2. **`message.ack`** - Delivery/read receipt
   - `type: "delivered"` - message reached recipient device
   - `type: "read"` - recipient opened the message

### Webhook Validation

If `WA_WEBHOOK_SECRET` is set, the handler validates the `X-Webhook-Signature` header (HMAC-SHA256).

## WhatsApp Markdown

WhatsApp uses simplified markdown. The system auto-converts common GitHub markdown:

| GitHub MD | WhatsApp MD | Renders As |
|-----------|-------------|------------|
| `**bold**` | `*bold*` | **bold** |
| `*italic*` | `_italic_` | _italic_ |
| `~~strike~~` | `~strike~` | ~~strike~~ |
| `# Heading` | `*Heading*` | **Heading** |
| `[label](url)` | `label (url)` | label (url) |

**Not supported**: headings, links, images, tables.

See the `whatsapp://formatting` MCP resource for the full guide.

## MCP Tools

### Check & Audit
- **`whatsapp_check`** - Check if a phone is registered on WhatsApp (persist to contact)
- **`whatsapp_audit`** - Batch-check a segment of contacts

### Send & Inbox
- **`whatsapp_send`** - Send a text message to a contact or phone
- **`whatsapp_list`** - List WhatsApp messages (inbound + outbound)
- **`whatsapp_get`** - Get a single message by ID
- **`whatsapp_reply`** - Reply to an inbound message
- **`whatsapp_mark_read`** - Mark an inbound message as read
- **`whatsapp_get_media`** - Download media URL for a message with attachment

### Resource
- **`whatsapp://formatting`** - WhatsApp markdown formatting guide

## Database Schema

### `wa_messages` Table

Stores both inbound and outbound messages with delivery lifecycle:

- `message_id` - Gateway-assigned WhatsApp message ID (wamid)
- `direction` - `in` (received) or `out` (sent)
- `phone` - Normalized E.164 (no +), e.g. `628xxx`
- `contact_id` - Linked to known contact (if matched)
- `body` - WhatsApp-formatted text
- `media_type` - `image`, `video`, `audio`, `document`, `sticker`, or empty
- `media_url` - Gateway-served URL for media
- `status` - `sent`, `delivered`, `read`, `failed`, `received`
- `sent_at`, `delivered_at`, `read_at`, `received_at` - Lifecycle timestamps
- `replied_to` - Links outbound reply to inbound message ID

### Contact Fields

- `whatsapp_status` - `unknown`, `registered`, `not_registered`
- `whatsapp_checked_at` - Last audit timestamp

## Ban Prevention

WhatsApp aggressively bans accounts that send bulk messages. The smart-send policy applies several safeguards:

1. **Rate limiting** - Token-bucket throttle (e.g., 10 messages per 60 seconds)
2. **Jitter** - Random delay between messages to avoid perfect cadence
3. **Daily cap** - Limit messages per recipient per 24h
4. **Warmup ramp** - Global ceiling that increases as the account ages
5. **Block unregistered** - Refuse sends to numbers verified not on WhatsApp

### Recommended Settings for New Accounts

```bash
WA_SEND_MAX=5
WA_SEND_WINDOW_SEC=60
WA_SEND_DAILY_CAP=3
WA_WARMUP_PER_DAY=20
WA_SEND_JITTER_MIN_MS=2000
WA_SEND_JITTER_MAX_MS=5000
WA_BLOCK_UNREGISTERED_SEND=true
```

Increase `WA_WARMUP_PER_DAY` gradually over weeks as the account establishes trust.

## Phone Number Normalization

The system normalizes phone numbers to E.164 digits-only format:

- Strips `+`, spaces, dashes, parentheses
- Replaces leading `0` with `62` (Indonesian national prefix)
- Replaces leading `00` with nothing (international dialing prefix)
- Passes through full JIDs (`628xxx@s.whatsapp.net`) unchanged

Examples:
- `0899-692-6184` → `628996926184`
- `+62 899 6926 184` → `628996926184`
- `00628996926184` → `628996926184`

## Troubleshooting

### "whatsapp disabled" Error

The WhatsApp service is not configured. Set `WA_BASE_URL` and `WA_DEVICE_ID`.

### "contact verified not on WhatsApp" Error

The contact's `whatsapp_status` is `not_registered`. Either:
- Remove the contact from WhatsApp campaigns
- Set `WA_BLOCK_UNREGISTERED_SEND=false` to override (not recommended)

### Webhook Not Receiving Events

1. Verify the gateway is configured with `WHATSAPP_WEBHOOK=<your-crm-url>/wa/webhook`
2. Check that your CRM is publicly accessible (not localhost)
3. If using `WA_WEBHOOK_SECRET`, verify the gateway is signing requests with the same secret

### Messages Not Marked as Read

The `whatsapp_mark_read` tool calls the gateway's `/message/{id}/read` endpoint. Ensure:
- The message ID exists in the gateway's chat history
- The device is connected and authenticated

## Migration

Run migration `0005_whatsapp.up.sql` to create the `wa_messages` table and add `whatsapp_status`/`whatsapp_checked_at` columns to `contacts`.

```bash
mysql -u root -p your_database < migrations/0005_whatsapp.up.sql
```

To rollback:

```bash
mysql -u root -p your_database < migrations/0005_whatsapp.down.sql
```

## Testing

Unit tests cover:
- Phone normalization (`internal/adapter/whatsapp/phone_test.go`)
- Markdown conversion (`internal/adapter/whatsapp/markdown_test.go`)
- Smart-send policy (`internal/adapter/whatsapp/smart_send_test.go`)
- HTTP client (`internal/adapter/whatsapp/client_test.go`)

Run all tests:

```bash
go test ./...
```

## Security

- **Never log `WA_BASIC_AUTH`** - it's a base64-encoded credential
- **Use `WA_WEBHOOK_SECRET`** - validates webhook authenticity via HMAC-SHA256
- **Webhook endpoint is public** - anyone can POST to `/wa/webhook`, so always validate signatures
- **Media URLs are temporary** - download and cache if you need long-term storage
