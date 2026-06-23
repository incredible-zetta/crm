-- 0010_multi_tenancy: optional multi-tenant data scoping.
--
-- Every tenant-owned table gains a tenant_id column defaulting to 'default'
-- so existing single-tenant rows keep working unchanged. Name/natural-key
-- uniqueness becomes per-tenant (composite) so two tenants may reuse the same
-- email, template name, mailbox, chat, or external object id.
--
-- Public-facing random codes (tracking_links.code, exports.id,
-- contacts.unsub_code) stay GLOBALLY unique on purpose: the unauthenticated
-- HTTP routes (/t /o /export /u) carry no session header and resolve the
-- owning tenant from the matched row.

CREATE TABLE IF NOT EXISTS tenants (
  id VARCHAR(64) PRIMARY KEY,
  api_key_hash CHAR(64) NOT NULL,
  session_id VARCHAR(255) NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  last_seen_at TIMESTAMP NULL,
  UNIQUE KEY uq_tenants_key_session (api_key_hash, session_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO tenants (id, api_key_hash, session_id)
VALUES ('default', '', '')
ON DUPLICATE KEY UPDATE id = id;

-- contacts: per-tenant email; unsub_code stays globally unique (public route).
ALTER TABLE contacts
  ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default' AFTER id,
  DROP INDEX email,
  ADD UNIQUE KEY uq_contacts_tenant_email (tenant_id, email),
  ADD INDEX idx_contacts_tenant (tenant_id);

-- email_templates: per-tenant name.
ALTER TABLE email_templates
  ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default' AFTER id,
  DROP INDEX name,
  ADD UNIQUE KEY uq_templates_tenant_name (tenant_id, name),
  ADD INDEX idx_templates_tenant (tenant_id);

ALTER TABLE campaigns
  ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default' AFTER id,
  ADD INDEX idx_campaigns_tenant (tenant_id);

ALTER TABLE tracking_links
  ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default' AFTER id,
  ADD INDEX idx_links_tenant (tenant_id);

ALTER TABLE email_events
  ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default' AFTER id,
  ADD INDEX idx_events_tenant (tenant_id);

ALTER TABLE scheduled_tasks
  ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default' AFTER id,
  ADD INDEX idx_tasks_tenant (tenant_id);

ALTER TABLE exports
  ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default' AFTER id,
  ADD INDEX idx_exports_tenant (tenant_id);

-- inbox_cursors: per-tenant mailbox.
ALTER TABLE inbox_cursors
  ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default' AFTER id,
  DROP INDEX mailbox,
  ADD UNIQUE KEY uq_inbox_cursors_tenant_mailbox (tenant_id, mailbox);

-- inbound_messages: per-tenant (mailbox,uid) and (message_id).
ALTER TABLE inbound_messages
  ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default' AFTER id,
  DROP INDEX uq_inbound_mailbox_uid,
  DROP INDEX uq_inbound_message_id,
  ADD UNIQUE KEY uq_inbound_tenant_mailbox_uid (tenant_id, mailbox, uid),
  ADD UNIQUE KEY uq_inbound_tenant_message_id (tenant_id, message_id),
  ADD INDEX idx_inbound_tenant (tenant_id);

-- wa_messages: per-tenant message_id.
ALTER TABLE wa_messages
  ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default' AFTER id,
  DROP INDEX uq_wa_message_id,
  ADD UNIQUE KEY uq_wa_tenant_message_id (tenant_id, message_id),
  ADD INDEX idx_wa_tenant (tenant_id);

-- threads_posts: per-tenant threads_id.
ALTER TABLE threads_posts
  ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default' AFTER id,
  DROP INDEX uq_threads_posts_threads_id,
  ADD UNIQUE KEY uq_threads_posts_tenant_threads_id (tenant_id, threads_id),
  ADD INDEX idx_threads_posts_tenant (tenant_id);

-- threads_replies: per-tenant reply_id.
ALTER TABLE threads_replies
  ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default' AFTER id,
  DROP INDEX uq_threads_replies_reply_id,
  ADD UNIQUE KEY uq_threads_replies_tenant_reply_id (tenant_id, reply_id),
  ADD INDEX idx_threads_replies_tenant (tenant_id);

-- threads_mentions: per-tenant mention_id.
ALTER TABLE threads_mentions
  ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default' AFTER id,
  DROP INDEX uq_threads_mentions_mention_id,
  ADD UNIQUE KEY uq_threads_mentions_tenant_mention_id (tenant_id, mention_id),
  ADD INDEX idx_threads_mentions_tenant (tenant_id);

ALTER TABLE threads_audit_events
  ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default' AFTER id,
  ADD INDEX idx_threads_audit_tenant (tenant_id);

-- wa_listeners: per-tenant chat_jid.
ALTER TABLE wa_listeners
  ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT 'default' AFTER id,
  DROP INDEX uq_wa_listener_chat_jid,
  ADD UNIQUE KEY uq_wa_listeners_tenant_chat_jid (tenant_id, chat_jid),
  ADD INDEX idx_wa_listeners_tenant (tenant_id);
