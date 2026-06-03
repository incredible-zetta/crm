CREATE TABLE IF NOT EXISTS contacts (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  email VARCHAR(320) UNIQUE NOT NULL,
  first_name VARCHAR(120),
  last_name VARCHAR(120),
  company VARCHAR(200),
  phone VARCHAR(40),
  stage ENUM('new','contacted','qualified','proposal','won','lost') NOT NULL DEFAULT 'new',
  tags JSON,
  notes TEXT,
  custom JSON,
  source VARCHAR(80),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_contacts_stage (stage),
  INDEX idx_contacts_company (company)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS email_templates (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(160) UNIQUE NOT NULL,
  subject VARCHAR(400),
  body_html MEDIUMTEXT,
  body_text MEDIUMTEXT,
  variables JSON,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS campaigns (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(200) NOT NULL,
  template_id BIGINT,
  provider ENUM('smtp','mailgun') NOT NULL DEFAULT 'smtp',
  segment JSON,
  status ENUM('draft','scheduled','sending','sent','failed') NOT NULL DEFAULT 'draft',
  scheduled_at TIMESTAMP NULL,
  stats JSON,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS tracking_links (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  code CHAR(12) UNIQUE NOT NULL,
  target_url TEXT NOT NULL,
  campaign_id BIGINT NULL,
  contact_id BIGINT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS email_events (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  contact_id BIGINT,
  campaign_id BIGINT NULL,
  type ENUM('sent','delivered','open','click','bounce','failed') NOT NULL,
  link_code CHAR(12) NULL,
  meta JSON,
  ts TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_events_campaign_type (campaign_id, type),
  INDEX idx_events_contact (contact_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS scheduled_tasks (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  kind ENUM('email','campaign') NOT NULL,
  payload JSON,
  run_at TIMESTAMP NOT NULL,
  status ENUM('pending','running','done','failed') NOT NULL DEFAULT 'pending',
  attempts INT NOT NULL DEFAULT 0,
  last_error TEXT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_tasks_status_runat (status, run_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS exports (
  id CHAR(16) PRIMARY KEY,
  path VARCHAR(300) NOT NULL,
  `rows` INT NOT NULL DEFAULT 0,
  expires_at TIMESTAMP NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
