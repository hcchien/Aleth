# Caching Architecture

Aleth uses Redis as an optional cache layer in two services: **content** (single-post lookups) and **feed** (personalized and explore feeds). Both layers are disabled automatically when the corresponding `REDIS_URL` env var is not set, so the application runs without Redis in local development.

---

## Overview

```
Client → Gateway → Content Service ──▶ Redis (post cache, 3 min TTL)
                                   └──▶ Postgres (on miss)

Client → Gateway → Feed Service ──▶ Redis (feed/explore cache, 60-90 s TTL)
                                └──▶ Postgres (on miss)
```

The cache-aside pattern is used throughout:

1. **Read**: check cache → hit returns immediately; miss falls through to DB
2. **Write-back**: after a DB read, result is stored in Redis **asynchronously** (goroutine) so the caller is never delayed by a slow Redis write
3. **Invalidation**: writes that change observable state evict the relevant keys synchronously before returning

---

## Content Service — Post Cache

### Key schema

| Key pattern | Scope | Example |
|---|---|---|
| `post:{postID}:anon` | anonymous viewer | `post:abc123…:anon` |
| `post:{postID}:{viewerID}` | authenticated viewer | `post:abc123…:user456…` |

Anonymous and authenticated keys are **separate** because `IsLiked` and `ViewerEmotion` are viewer-specific fields returned by `GetPostByID`. Sharing a single key would serve stale reaction data to the wrong viewer.

### TTL

| Cache | TTL | Rationale |
|---|---|---|
| Post (anon) | 3 minutes | Public content rarely changes within this window |
| Post (viewer) | 3 minutes | Viewer emotion/like state is eventually consistent |

### Invalidation triggers

| Operation | Keys evicted |
|---|---|
| `DeletePost(id)` | `post:{id}:anon` (immediate; viewer keys age out on TTL) |
| `LikePost(postID, userID)` | `post:{postID}:{userID}` |
| `ReactPost(postID, userID, …)` | `post:{postID}:{userID}` |
| `UnlikePost(postID, userID)` | `post:{postID}:{userID}` |

> **Why not evict all viewers on delete?**
> The `posts` table uses soft-delete (`deleted_at`). After deletion the owner's key is evicted (empty response visible immediately) but other per-viewer keys are not enumerated — they expire naturally within 3 minutes. This avoids an expensive `SCAN` or secondary index of all cached viewer keys.

### Configuration

```
CONTENT_REDIS_URL=redis://localhost:6379/0
```

Leave unset (or empty) to disable caching. The service starts and operates normally without Redis — all cache calls become no-ops.

---

## Feed Service — Feed and Explore Cache

### Key schema

| Key pattern | Scope | Example |
|---|---|---|
| `feed:{viewerID}:` | personalized feed, first page | `feed:user123…:` |
| `feed:{viewerID}:{cursor}` | personalized feed, page N | not cached (see below) |
| `explore:{viewerID}` | explore feed, per viewer | `explore:user123…` |
| `explore:anon` | explore feed, anonymous | `explore:anon` |

### What is and is not cached

| Request type | Cached? | Reason |
|---|---|---|
| Authenticated feed, **first page** (cursor = `""`) | ✅ Yes | Most frequent request; cursor = "" is stable |
| Authenticated feed, **subsequent pages** (cursor set) | ❌ No | Page N is rarely re-requested; caching all cursors would waste memory |
| Anonymous feed (falls through to explore) | ❌ No (via GetFeed) | AnonymouS users always get explore; use `GetExploreFeed` directly |
| Explore feed (any viewer) | ✅ Yes | Expensive ranking query; result shared per viewer |
| Explore feed (anonymous) | ✅ Yes (key `explore:anon`) | All anonymous viewers share one key |

### TTLs

| Cache | TTL | Rationale |
|---|---|---|
| Personalized feed (first page) | 60 seconds | Acceptable lag; new posts appear within 1 minute |
| Explore feed | 90 seconds | HN-style score changes continuously; slightly longer TTL reduces DB load |

### Invalidation

The feed cache has **no active invalidation** — entries expire naturally on TTL. This is acceptable because:

- Feed staleness of 60–90 seconds is typical for social feeds
- The write path (post creation) does not know which viewers have cached feeds
- On-demand invalidation would require a secondary data structure (set of viewer IDs per followee)

Future improvement: on `CreatePost`, publish a Pub/Sub message that a subscriber (or the feed service itself) uses to evict `feed:{authorFollowers}:` keys.

### Configuration

```
FEED_REDIS_URL=redis://localhost:6379/1
```

Using DB index 1 (`/1`) keeps feed keys separate from content keys if both services share a Redis instance. This is optional but keeps key spaces clean.

---

## Nil-safe Design

Both `cache.Client` (concrete) and the `cacher` interface (used in tests) are designed so that a **nil / no-op instance never panics**:

- `*cache.Client` methods guard against `nil` receiver
- `nopCache{}` is the default `cacher` injected into services at construction time
- `SetCache(nil)` replaces the cache with `nopCache{}` rather than a nil interface

This means the service code never needs to check `if s.cache != nil` — all paths are safe.

---

## Testing

The cache layer is exercised by unit tests using an in-memory `fakeCache`:

| Test file | What is tested |
|---|---|
| `services/content/internal/service/cache_test.go` | GetPost hit/miss, async write-back, viewer-scoped keys, eviction on delete/like/react/unlike |
| `services/feed/internal/service/feed_test.go` | GetFeed hit/miss, first-page-only caching, GetExploreFeed hit/miss, anon key, nil cache |

The `fakeCache` stores JSON-serialised blobs in a thread-safe `map[string][]byte`, mirroring the JSON round-trip of the real Redis client. This catches any type that fails to round-trip through `encoding/json`.

To run cache tests:

```bash
cd services/content
go test ./internal/service/ -run TestGetPost -v
go test ./internal/service/ -run TestLikePost -v
go test ./internal/service/ -run TestDeletePost -v

cd services/feed
go test ./internal/service/ -run TestGetFeed -v
go test ./internal/service/ -run TestGetExploreFeed -v
```

---

## Local Development

No Redis required. Simply omit `CONTENT_REDIS_URL` and `FEED_REDIS_URL` from your `.env` file (or `docker-compose.yml`). The services log:

```
feed cache disabled (FEED_REDIS_URL not set)
post cache disabled (CONTENT_REDIS_URL not set)
```

To enable caching locally:

```bash
docker run -d -p 6379:6379 redis:7-alpine
export CONTENT_REDIS_URL=redis://localhost:6379/0
export FEED_REDIS_URL=redis://localhost:6379/1
```

---

## Production (Cloud Run + GCP)

In production, use **Redis Memorystore** (GCP's managed Redis):

1. Create a Memorystore instance in the same VPC as your Cloud Run services
2. Set `CONTENT_REDIS_URL` and `FEED_REDIS_URL` to the Memorystore IP:

```
CONTENT_REDIS_URL=redis://10.x.x.x:6379/0
FEED_REDIS_URL=redis://10.x.x.x:6379/1
```

3. Configure Cloud Run VPC connector to allow private IP access to Memorystore
4. Store the Redis URL in **Secret Manager** and reference it in `cloudbuild.yaml` under `availableSecrets`

Both content and feed services can share a single Memorystore instance with different DB indices.
