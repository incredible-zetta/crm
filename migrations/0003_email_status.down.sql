ALTER TABLE contacts
  DROP INDEX idx_contacts_email_status,
  DROP COLUMN email_checked_at,
  DROP COLUMN email_reason,
  DROP COLUMN email_status;
