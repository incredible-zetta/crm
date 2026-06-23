-- 0010_multi_tenancy down: drop tenant scoping and restore original
-- single-tenant unique keys.

ALTER TABLE wa_listeners
  DROP INDEX uq_wa_listeners_tenant_chat_jid,
  DROP INDEX idx_wa_listeners_tenant,
  ADD UNIQUE KEY uq_wa_listener_chat_jid (chat_jid),
  DROP COLUMN tenant_id;

ALTER TABLE threads_audit_events
  DROP INDEX idx_threads_audit_tenant,
  DROP COLUMN tenant_id;

ALTER TABLE threads_mentions
  DROP INDEX uq_threads_mentions_tenant_mention_id,
  DROP INDEX idx_threads_mentions_tenant,
  ADD UNIQUE KEY uq_threads_mentions_mention_id (mention_id),
  DROP COLUMN tenant_id;

ALTER TABLE threads_replies
  DROP INDEX uq_threads_replies_tenant_reply_id,
  DROP INDEX idx_threads_replies_tenant,
  ADD UNIQUE KEY uq_threads_replies_reply_id (reply_id),
  DROP COLUMN tenant_id;

ALTER TABLE threads_posts
  DROP INDEX uq_threads_posts_tenant_threads_id,
  DROP INDEX idx_threads_posts_tenant,
  ADD UNIQUE KEY uq_threads_posts_threads_id (threads_id),
  DROP COLUMN tenant_id;

ALTER TABLE wa_messages
  DROP INDEX uq_wa_tenant_message_id,
  DROP INDEX idx_wa_tenant,
  ADD UNIQUE KEY uq_wa_message_id (message_id),
  DROP COLUMN tenant_id;

ALTER TABLE inbound_messages
  DROP INDEX uq_inbound_tenant_mailbox_uid,
  DROP INDEX uq_inbound_tenant_message_id,
  DROP INDEX idx_inbound_tenant,
  ADD UNIQUE KEY uq_inbound_mailbox_uid (mailbox, uid),
  ADD UNIQUE KEY uq_inbound_message_id (message_id),
  DROP COLUMN tenant_id;

ALTER TABLE inbox_cursors
  DROP INDEX uq_inbox_cursors_tenant_mailbox,
  ADD UNIQUE KEY mailbox (mailbox),
  DROP COLUMN tenant_id;

ALTER TABLE exports
  DROP INDEX idx_exports_tenant,
  DROP COLUMN tenant_id;

ALTER TABLE scheduled_tasks
  DROP INDEX idx_tasks_tenant,
  DROP COLUMN tenant_id;

ALTER TABLE email_events
  DROP INDEX idx_events_tenant,
  DROP COLUMN tenant_id;

ALTER TABLE tracking_links
  DROP INDEX idx_links_tenant,
  DROP COLUMN tenant_id;

ALTER TABLE campaigns
  DROP INDEX idx_campaigns_tenant,
  DROP COLUMN tenant_id;

ALTER TABLE email_templates
  DROP INDEX uq_templates_tenant_name,
  DROP INDEX idx_templates_tenant,
  ADD UNIQUE KEY name (name),
  DROP COLUMN tenant_id;

ALTER TABLE contacts
  DROP INDEX uq_contacts_tenant_email,
  DROP INDEX idx_contacts_tenant,
  ADD UNIQUE KEY email (email),
  DROP COLUMN tenant_id;

DROP TABLE IF EXISTS tenants;
