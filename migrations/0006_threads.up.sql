CREATE TABLE IF NOT EXISTS threads_posts (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  threads_id VARCHAR(255) NOT NULL,
  media_product_type VARCHAR(64),
  media_type VARCHAR(64),
  text MEDIUMTEXT,
  permalink TEXT,
  timestamp TIMESTAMP NULL,
  username VARCHAR(255),
  is_quote_post BOOLEAN NOT NULL DEFAULT FALSE,
  raw_json JSON,
  deleted_at TIMESTAMP NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_threads_posts_threads_id (threads_id),
  INDEX idx_threads_posts_username (username),
  INDEX idx_threads_posts_timestamp (timestamp),
  INDEX idx_threads_posts_deleted (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS threads_replies (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  reply_id VARCHAR(255) NOT NULL,
  post_id VARCHAR(255) NOT NULL,
  text MEDIUMTEXT,
  username VARCHAR(255),
  timestamp TIMESTAMP NULL,
  hide_status VARCHAR(64),
  raw_json JSON,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_threads_replies_reply_id (reply_id),
  INDEX idx_threads_replies_post_id (post_id),
  INDEX idx_threads_replies_timestamp (timestamp)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS threads_mentions (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  mention_id VARCHAR(255) NOT NULL,
  text MEDIUMTEXT,
  username VARCHAR(255),
  permalink TEXT,
  timestamp TIMESTAMP NULL,
  raw_json JSON,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_threads_mentions_mention_id (mention_id),
  INDEX idx_threads_mentions_username (username),
  INDEX idx_threads_mentions_timestamp (timestamp)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS threads_audit_events (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  action VARCHAR(80) NOT NULL,
  object_id VARCHAR(255),
  ok BOOLEAN NOT NULL DEFAULT TRUE,
  error TEXT,
  raw_json JSON,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_threads_audit_action (action),
  INDEX idx_threads_audit_created (created_at),
  INDEX idx_threads_audit_object (object_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
