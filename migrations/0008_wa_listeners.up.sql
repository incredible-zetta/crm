CREATE TABLE IF NOT EXISTS wa_listeners (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  chat_jid VARCHAR(255) NOT NULL,
  name VARCHAR(255) NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  summary MEDIUMTEXT NULL,
  deleted_at TIMESTAMP NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_wa_listener_chat_jid (chat_jid),
  INDEX idx_wa_listeners_enabled (enabled),
  INDEX idx_wa_listeners_deleted (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE wa_messages
  ADD INDEX idx_wa_chat_created (chat_id, created_at),
  ADD INDEX idx_wa_chat_id (chat_id);
