# Research: Can we get followers / following on Threads profiles?

Date: 2026-06-17
Branch: `research/threads-followers-following`
Question: Can the Threads channel return **followers** and **following** for a profile?

## TL;DR

| Want | Official Threads Graph API | Verdict |
|------|----------------------------|---------|
| Follower **count** (your own account) | `followers_count` user-insight metric | ✅ supported |
| Follower **demographics** (country/city/age/gender) | `follower_demographics` metric (needs ≥100 followers) | ✅ supported (aggregate only) |
| **List** of followers (who follows you) | none | ❌ not supported |
| **Following** count (accounts you follow) | none | ❌ not supported |
| **List** of accounts you follow | none | ❌ not supported |
| Followers/following for **other** users | none | ❌ not supported |

Bottom line: the official API gives you your own **follower count** (and aggregate
demographics) as an insight metric. There is **no** following count, **no** follower
list, and **no** following list. Other users' follower data is not available either.
Anything that returns a follower list (Apify actors, unofficial clients) is scraping,
not the official API, and is out of scope for this MCP (we are token/Graph-API based).

## Evidence

### followers_count is an insight metric, not a profile field
User insights endpoint `GET /{threads-user-id}/threads_insights` accepts:

```
metric = views,likes,replies,reposts,quotes,clicks,followers_count,follower_demographics
```

- `followers_count` → lifetime total follower count (int).
- `follower_demographics` → breakdown by `country|city|age|gender`; requires the
  profile to have **at least 100 followers**, and is **not** compatible with
  `since`/`until`.

(Source: developers.facebook.com/docs/threads/reference/insights; Threads API
changelog 2024-05-21; Supermetrics Threads insights field list.)

### Profile fields do NOT include follower/following
`GET /me` or `GET /{threads-user-id}` profile fields are:

```
id, username, name, threads_profile_picture_url, threads_biography,
is_eligible_for_geo_gating
```

No `followers_count`, no `follows_count`, no follower/following edge. (Instagram
Graph API exposes `follows_count` on the account; the **Threads** API does not.)

### No follower/following list edge exists
Threads user edges in the official API are limited to:
- `GET /{threads-user-id}/threads` — your posts
- `GET /{threads-user-id}/replies` — your replies
- `GET /{threads-user-id}/mentions` — posts mentioning you

There is no `/followers` or `/following` edge. Meta deliberately does not expose
follower/following lists (same posture as Instagram Graph API).

## What we already have

`internal/adapter/threads/client.go`:
- `Profile()` requests `id,username,name,threads_profile_picture_url,threads_biography`.
- `Insights(mediaID)` user-level path already requests
  `views,likes,replies,reposts,quotes,followers_count`.

So **follower count is already reachable today** via `threads_insights` (user-level,
no media id). We just don't surface it as a first-class field.

## Options

1. **Do nothing / document.** `threads_insights` already returns `followers_count`.
   Tell agents to call `threads_insights` with no `threads_id` and read
   `followers_count`. Cheapest; no code change. (Recommended baseline.)

2. **Add `followers_count` to the profile response.** Make `threads_profile` also
   fetch the `followers_count` insight (one extra Graph call) and return it as a
   typed field, so an agent gets handle + bio + follower count in one tool call.
   Nice DX; small cost (extra request, and insight may fail on brand-new accounts).

3. **Add a `follower_demographics` passthrough** to `threads_insights` (breakdown
   param). Useful for audience analytics; gated behind ≥100 followers.

Following count / follower lists / other users' followers are **not implementable**
with the official API — do not promise them. If that data is ever required it means
scraping (Apify-style), which is a separate, ToS-risky effort and not part of this
Graph-API MCP.

## Recommendation

> **Status: implemented 2026-06-17** — option 2 (followers_count on `threads_profile`)
> and option 3 (`threads_follower_demographics` tool) shipped on branch
> `research/threads-followers-following`. Following count / follower lists remain
> unimplementable on the official API and are documented as unsupported.

- Short term: document option 1 in `threads://publishing` + wiki so agents know
  `followers_count` comes from `threads_insights`, and that following/lists are
  unavailable.
- If we want better DX: implement option 2 (surface `followers_count` on
  `threads_profile`) and optionally option 3 (`follower_demographics` breakdown).
- Explicitly record the hard limits so future work doesn't try to build a
  followers/following list on the official API.

## Live verification TODO (needs real token)
- `threads_insights` (no media id) → confirm `followers_count` present.
- `follower_demographics` with `breakdown=country` on an account with ≥100 followers.
