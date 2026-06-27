# Threads Channel

Hybrid Threads integration: live Meta Threads Graph API calls plus MySQL cache/audit.

## Enable

```env
THREADS_ACCESS_TOKEN=...
THREADS_USER_ID=me
THREADS_API_VERSION=v1.0
# cookie-only discovery (optional, separate path — no token):
THREADS_DISCOVERY_BIN=/opt/threads/threads
THREADS_COOKIES_FILE=/opt/threads/www.threads.com_cookies.txt
```

Never commit real tokens. Required scopes for the full tool set:

- `threads_basic`
- `threads_content_publish`
- `threads_delete`
- `threads_keyword_search`
- `threads_manage_insights`
- `threads_manage_mentions`
- `threads_manage_replies`
- `threads_profile_discovery`
- `threads_read_replies`

## Tools

| Tool | Purpose |
|------|---------|
| `threads_profile` | Fetch configured profile |
| `threads_profile_lookup` | Look up any public profile by username (follower_count + engagement counts); needs `threads_profile_discovery` |
| `threads_list` | List live posts and cache them |
| `threads_publish` | Publish text/image/video post with optional `topic_tag` |
| `threads_delete` | Delete live post and soft-delete cache row |
| `threads_insights` | Fetch user-level or media-level insights |
| `threads_daily_summary` | Summarize a day's posts: per-post insights + reply breakdown + engagement and account totals |
| `threads_follower_demographics` | Aggregate follower demographics (country/city/age/gender); needs ≥100 followers |
| `threads_replies` | Read replies for a post |
| `threads_reply_tree` | Nested reply conversation tree (is_mine/needs_reply) |
| `threads_reply` | Publish a reply to a post or comment |
| `threads_reply_hide` | Hide (moderate) a reply/comment on your post |
| `threads_reply_unhide` | Unhide a previously hidden reply on your post |
| `threads_reply_quota` | Check reply quota usage |
| `threads_mentions` | Read mentions |
| `threads_search` | Keyword/topic search with filters |
| `threads_discover` | Cookie-only discovery of PUBLIC posts by topic (no token); `mode=posts`/`viral`/`latest` |
| `threads_token_exchange` | Exchange short-lived token for long-lived token |
| `threads_token_refresh` | Refresh long-lived token before expiry |
| `threads_list_cached` | List cached posts |
| `threads_get_cached` | Get cached post |
| `threads_history` | List audit events |
| `threads_delete_cached` | Soft-delete cache row only |

## MCP examples

### Profile

```json
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"threads_profile","arguments":{}}}
```

### Publish text with topic tag

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "threads_publish",
    "arguments": {
      "text": "Hello from ZettaCRM",
      "topic_tag": "AI Threads"
    }
  }
}
```

`topic_tag` rules:

- length 1-50 characters
- disallow `.` and `&`
- one topic tag per post
- preferred over inline `#topic` in text

### Publish image

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "threads_publish",
    "arguments": {
      "text": "Hello saya ZettaCRM",
      "image_url": "https://example.com/image.png"
    }
  }
}
```

### Reply to a post

`threads_reply` uses Meta's two-step flow internally:

1. `POST /{threads-user-id}/threads` with `reply_to_id`
2. `POST /{threads-user-id}/threads_publish` with `creation_id`

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "threads_reply",
    "arguments": {
      "threads_id": "17848812657686460",
      "text": "Setuju. Vibe coding oke kalau tetap ada taste, review, dan konteks engineering-nya."
    }
  }
}
```

### Check reply quota

```json
{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"threads_reply_quota","arguments":{}}}
```

Expected API shape:

```json
{
  "data": [
    {
      "reply_quota_usage": 2,
      "reply_config": {
        "quota_total": 1000,
        "quota_duration": 86400
      }
    }
  ]
}
```

### Search

```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "tools/call",
  "params": {
    "name": "threads_search",
    "arguments": {
      "query": "CRM",
      "search_type": "RECENT",
      "author_username": "callmelords",
      "limit": 5
    }
  }
}
```

Search parameters:

- `search_type`: `TOP` (default) or `RECENT`
- `search_mode`: `KEYWORD` (default) or `TAG`
- `media_type`: `TEXT`, `IMAGE`, or `VIDEO`
- `author_username`: exact username without `@`
- `since` / `until`: Unix timestamp or parseable date/time
- `fields`: comma-separated response fields; default includes `topic_tag`

Public search may require approved `threads_keyword_search`. Without approval the API can return only the authenticated user's posts, so use `author_username` for reliable owned-content discovery.

### Token exchange

```json
{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"threads_token_exchange","arguments":{}}}
```

Graph-generated tokens can be valid for API calls but not refreshable. Exchange first, store the returned `access_token` as `THREADS_ACCESS_TOKEN`, then refresh before expiry. Requires `THREADS_APP_SECRET`.

### Token refresh

```json
{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"threads_token_refresh","arguments":{}}}
```

Requires an unexpired long-lived token.

### Read replies

```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "method": "tools/call",
  "params": {
    "name": "threads_replies",
    "arguments": {
      "threads_id": "17848812657686460",
      "limit": 10
    }
  }
}
```

### Reply conversation tree

`threads_reply_tree` returns the full nested reply conversation for a post via the
Threads `/{id}/conversation` endpoint. Each node carries `is_mine` (authored by
the configured account), `needs_reply` (another user's comment with no reply from
you beneath it), `depth`, and nested `children`. Top-level fields include
`already_replied`, `needs_reply_count`, and `my_replies`.

```json
{
  "jsonrpc": "2.0",
  "id": 10,
  "method": "tools/call",
  "params": {
    "name": "threads_reply_tree",
    "arguments": {
      "threads_id": "18593061424004322",
      "limit": 50
    }
  }
}
```

Reply targeting: to answer a **comment**, pass that comment's `reply_id` to
`threads_reply`, not the root post id. Passing the post id posts a top-level
reply on your own post instead of answering the commenter.

```json
{
  "jsonrpc": "2.0",
  "id": 11,
  "method": "tools/call",
  "params": {
    "name": "threads_reply",
    "arguments": {
      "reply_id": "<comment reply_id from threads_reply_tree>",
      "text": "Thanks for the question — ..."
    }
  }
}
```

### Daily summary

`threads_daily_summary` reports one local day's posts in a single call. It lists
the day's posts, enriches each with media-level insights, derives a reply
breakdown from the conversation tree, and aggregates account-wide totals.

Arguments:

- `date` — local day as `YYYY-MM-DD` (default: today)
- `timezone` — IANA zone for the day window + date parsing, e.g. `Asia/Jakarta`
  (default: server local timezone)
- `max_posts` — recent posts to scan for the day (default 25, capped at 100)

```json
{
  "jsonrpc": "2.0",
  "id": 12,
  "method": "tools/call",
  "params": {
    "name": "threads_daily_summary",
    "arguments": { "timezone": "Asia/Jakarta" }
  }
}
```

Per-post fields: `views`, `likes`, `reposts`, `quotes`, `replies_metric` (from
the insights API), plus a conversation-tree breakdown `total_replies`,
`my_replies`, `other_replies`, `needs_reply`. `engagement` =
`likes + reposts + quotes + other_replies`; `engagement_rate` =
`engagement / views` as a percentage. Account-wide `totals` and `followers_count`
are included.

Live API is the source of truth. A failure fetching one post's insights or
replies is reported inline as `insights_error` / `replies_error` on that post
instead of failing the whole summary.

`replies_metric` (the insights `replies` counter) can differ from
`total_replies` (distinct comments seen in the conversation tree); both are
returned so you can compare them.

### Edit / delete / moderate limitations

The Threads Graph API has no edit endpoint. You **cannot update** a post or a
reply once published — to change content you delete and re-create.

| Want to | API supports | Tool |
|---------|--------------|------|
| Edit a post | No | — (delete + re-publish) |
| Edit a reply/comment | No | — (delete + re-reply) |
| Delete your own post | Yes | `threads_delete` |
| Delete someone's comment on your post | No (no API) | use hide instead |
| Hide an unwanted reply on your post | Yes | `threads_reply_hide` |
| Unhide it | Yes | `threads_reply_unhide` |
| Hide replies on other people's posts | No | — |

Hiding is the moderation primitive Threads exposes (`hide_status` on a reply).
It requires the `threads_manage_replies` scope and only works on replies under
your own posts.

```json
{
  "jsonrpc": "2.0",
  "id": 12,
  "method": "tools/call",
  "params": {
    "name": "threads_reply_hide",
    "arguments": {
      "reply_id": "<comment reply_id from threads_reply_tree>"
    }
  }
}
```

### Delete live post

```json
{
  "jsonrpc": "2.0",
  "id": 8,
  "method": "tools/call",
  "params": {
    "name": "threads_delete",
    "arguments": {
      "threads_id": "18185104660387893"
    }
  }
}
```

## MCP resource

Read `threads://publishing` for agent-facing guidance on publishing, replies, topic tags, quotas, and known API limits.

## Live verification notes

Verified with real Threads token on 2026-06-14:

- profile read
- post list
- keyword search
- text publish
- text publish with `topic_tag` (`AI Threads`) then delete
- image publish
- live delete
- fetch after delete failure
- reply publish via corrected two-step flow
- read replies on own posts
- owned replies list via `/{user_id}/replies`
- media insights
- user insights
- reply quota
- mentions endpoint returning valid empty list

Not verified: video publishing, because it would create a real video post and waits for media processing.

## Cache/audit

Live API remains source of truth. MySQL cache stores typed columns plus `raw_json` in:

- `threads_posts`
- `threads_replies`
- `threads_mentions`
- `threads_audit_events`

Cached delete is soft delete (`deleted_at`) unless live delete is requested.

## Cookie-only discovery (`threads_discover`)

`threads_discover` is a **separate path** from the Graph API tools. It reads
PUBLIC posts of any account for AI-agent research/training using a logged-in
browser session cookie — no access token. Backed by the
[x-threads-utils](https://github.com/incredible-zetta/x-threads-utils) binary,
which embeds the GraphQL query catalog and scrapes dynamic tokens from
threads.com using only the cookie.

### Setup

1. Provision the `threads` binary one of two ways:
   - **Auto-download (recommended for containers):** set `GH_TOKEN` (a GitHub
     token with read access — the release repo is private) and
     `THREADS_DISCOVERY_BIN` to a writable path. On start, if the binary is
     missing, the server downloads the release archive for its OS/arch and
     extracts the binary to that path. Already present → no download. Optional
     `THREADS_DISCOVERY_REPO` / `THREADS_DISCOVERY_TAG` override the source.
   - **Manual:** download the `threads` binary for your OS/arch from the
     [releases page](https://github.com/incredible-zetta/x-threads-utils/releases),
     extract it, and point `THREADS_DISCOVERY_BIN` at it.
2. Export a Netscape cookie file for a logged-in `www.threads.com` session
   (use a browser "cookies.txt" extension). Needs `sessionid`, `ds_user_id`,
   `csrftoken`, `mid`, `ig_did`, `rur`. Mount it into the container manually
   (it is never auto-downloaded — it holds a live session).
3. Set env:

   ```env
   GH_TOKEN=ghp_xxx              # only needed for auto-download
   THREADS_DISCOVERY_BIN=/data/threads
   THREADS_COOKIES_FILE=/data/www.threads.com_cookies.txt
   ```

The server reads the cookie file at call time and never exposes it to the agent.
The channel is enabled only when **both** vars are set; it is independent of
`THREADS_ACCESS_TOKEN`.

### Modes

| `mode` | Output | Reliability |
|--------|--------|-------------|
| `posts` (default) | structured JSON: `pk`, `code`, `username`, `user_pk`, `full_name`, `caption`, `like_count`, `taken_at` | server-rendered HTML scrape — reliable |
| `viral` | engagement-ranked text (♥ likes + 💬 replies, ranked) | reliable |
| `latest` | newest-first text (by `taken_at`) | reliable |

### Example

```json
{
  "jsonrpc": "2.0",
  "id": 20,
  "method": "tools/call",
  "params": {
    "name": "threads_discover",
    "arguments": { "query": "software engineering", "mode": "posts" }
  }
}
```

Returns:

```json
{
  "mode": "posts",
  "posts": [
    {
      "pk": "3920530655558173937",
      "code": "DZohdv_kwTx",
      "username": "singgihmardianto",
      "user_pk": "77471145058",
      "full_name": "Singgih Mardianto",
      "caption": "My first thread post ...",
      "like_count": 4,
      "taken_at": "2026-06-15T..."
    }
  ]
}
```

### IDs do NOT cross to the Graph API

The `pk`/`code` returned here are **web IDs** and are not valid on the official
Graph API (passing a discovery `pk` as a Graph node returns `code 100 — object
does not exist`). Only the `username` string bridges the two paths.

To enrich a discovered username with official follower/engagement counts you
would call `threads_profile_lookup`, but that needs Meta-approved
`threads_profile_discovery`. As of 2026-06-17, arbitrary handles return
`code 10 — Application does not have permission` (only allow-listed profiles like
`meta` resolve); broader access requires Meta App Review. Until then,
`threads_discover` is the engagement source for arbitrary public accounts.

### Caveats

- `mode=posts` JSON exposes `like_count` only. Reply/repost/quote counts are
  shown in the `viral`/`latest` **text** output (the binary does not emit them
  as JSON fields).
- GraphQL-backed discovery commands (`search-users`, etc.) need a current
  `doc_id` that rotates per Threads build; the reliable cookie-only commands
  (`search-posts`, `viral`, `latest`) are the ones surfaced here.
- The cookie file holds a live authenticated session — gitignored, never commit.
