CREATE TABLE IF NOT EXISTS inbox_cursors (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  mailbox VARCHAR(255) NOT NULL UNIQUE,
  last_uid BIGINT UNSIGNED NOT NULL DEFAULT 0,
  last_message_date TIMESTAMP NULL,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS inbound_messages (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  mailbox VARCHAR(255) NOT NULL,
  uid BIGINT UNSIGNED NOT NULL,
  message_id VARCHAR(512) NULL,
  in_reply_to VARCHAR(512) NULL,
  references_header TEXT NULL,
  from_email VARCHAR(320) NOT NULL,
  from_name VARCHAR(255) NULL,
  to_email VARCHAR(320) NULL,
  subject VARCHAR(998) NULL,
  received_at TIMESTAMP NOT NULL,
  text_body MEDIUMTEXT,
  html_body MEDIUMTEXT,
  snippet VARCHAR(500),
  contact_id BIGINT NULL,
  campaign_id BIGINT NULL,
  read_at TIMESTAMP NULL,
  replied_at TIMESTAMP NULL,
  deleted_at TIMESTAMP NULL,
  notified_at TIMESTAMP NULL,
  raw_headers_json JSON,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_inbound_mailbox_uid (mailbox, uid),
  UNIQUE KEY uq_inbound_message_id (message_id),
  INDEX idx_inbound_from_email (from_email),
  INDEX idx_inbound_contact_received (contact_id, received_at),
  INDEX idx_inbound_read (read_at),
  INDEX idx_inbound_deleted (deleted_at),
  INDEX idx_inbound_notified (notified_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
