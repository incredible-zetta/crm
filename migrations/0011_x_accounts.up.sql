CREATE TABLE IF NOT EXISTS x_accounts (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
  label VARCHAR(255) NOT NULL,
  screen_name VARCHAR(255),
  user_id VARCHAR(64),
  cookies MEDIUMTEXT NOT NULL,
  liveness VARCHAR(16) NOT NULL DEFAULT 'unknown',
  last_checked_at TIMESTAMP NULL,
  last_error TEXT,
  deleted_at TIMESTAMP NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_x_accounts_tenant_label (tenant_id, label),
  INDEX idx_x_accounts_tenant (tenant_id),
  INDEX idx_x_accounts_liveness (liveness),
  INDEX idx_x_accounts_checked (last_checked_at),
  INDEX idx_x_accounts_deleted (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
