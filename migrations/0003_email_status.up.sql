ALTER TABLE contacts
  ADD COLUMN email_status ENUM('unknown','valid','invalid','risky') NOT NULL DEFAULT 'unknown',
  ADD COLUMN email_reason VARCHAR(120) NULL,
  ADD COLUMN email_checked_at TIMESTAMP NULL,
  ADD INDEX idx_contacts_email_status (email_status);
