# Threads Channel

Hybrid Threads integration: live Meta Threads Graph API calls plus MySQL cache/audit.

## Enable

```env
THREADS_ACCESS_TOKEN=...
THREADS_USER_ID=me
THREADS_API_VERSION=v1.0
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
| `threads_list` | List live posts and cache them |
| `threads_publish` | Publish text/image/video post with optional `topic_tag` |
| `threads_delete` | Delete live post and soft-delete cache row |
| `threads_insights` | Fetch user-level or media-level insights |
| `threads_follower_demographics` | Aggregate follower demographics (country/city/age/gender); needs ≥100 followers |
| `threads_replies` | Read replies for a post |
| `threads_reply_tree` | Nested reply conversation tree (is_mine/needs_reply) |
| `threads_reply` | Publish a reply to a post or comment |
| `threads_reply_hide` | Hide (moderate) a reply/comment on your post |
| `threads_reply_unhide` | Unhide a previously hidden reply on your post |
| `threads_reply_quota` | Check reply quota usage |
| `threads_mentions` | Read mentions |
| `threads_search` | Keyword/topic search with filters |
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
