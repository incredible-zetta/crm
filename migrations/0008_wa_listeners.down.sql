ALTER TABLE wa_messages
  DROP INDEX idx_wa_chat_created,
  DROP INDEX idx_wa_chat_id;

DROP TABLE IF EXISTS wa_listeners;
