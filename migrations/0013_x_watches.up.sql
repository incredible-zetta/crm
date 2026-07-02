-- 0013_x_watches: watch x.com signals (mentions/search) per account and
-- deliver new matches to a per-watch webhook and/or persisted event log.
--
-- x_watches      : one row per watch. kind + query define what to poll; the
--                  optional account label picks stored cookies; webhook_url +
--                  webhook_secret define an HMAC-signed delivery target that an
--                  AI agent can set/rotate. last_seen_id is the high-water mark
--                  used to only emit tweets newer than the last poll.
-- x_watch_events : every new matched tweet, deduped per (tenant, watch, tweet).
--                  Stores a delivery status so failed webhooks can be retried
--                  and an AI agent can read the backlog for auto-reply.

CREATE TABLE IF NOT EXISTS x_watches (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
  label VARCHAR(255) NOT NULL,
  kind VARCHAR(32) NOT NULL DEFAULT 'mention',
  query VARCHAR(512) NOT NULL DEFAULT '',
  account_label VARCHAR(255) NULL,
  webhook_url VARCHAR(1024) NULL,
  webhook_secret VARCHAR(255) NULL,
  webhook_headers JSON NULL,
  active TINYINT(1) NOT NULL DEFAULT 1,
  last_seen_id VARCHAR(64) NULL,
  last_polled_at TIMESTAMP NULL,
  last_error VARCHAR(1024) NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP NULL,
  UNIQUE KEY uq_x_watches_tenant_label (tenant_id, label),
  INDEX idx_x_watches_tenant (tenant_id),
  INDEX idx_x_watches_active (active, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS x_watch_events (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
  watch_id BIGINT NOT NULL,
  tweet_id VARCHAR(64) NOT NULL,
  author VARCHAR(255) NOT NULL DEFAULT '',
  text TEXT NOT NULL,
  url VARCHAR(512) NOT NULL DEFAULT '',
  likes INT NOT NULL DEFAULT 0,
  retweets INT NOT NULL DEFAULT 0,
  replies INT NOT NULL DEFAULT 0,
  tweet_created_at VARCHAR(64) NULL,
  delivery VARCHAR(32) NOT NULL DEFAULT 'pending',
  delivery_error VARCHAR(1024) NULL,
  delivered_at TIMESTAMP NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_x_watch_events_tenant_watch_tweet (tenant_id, watch_id, tweet_id),
  INDEX idx_x_watch_events_watch (watch_id),
  INDEX idx_x_watch_events_delivery (delivery, tenant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
