ALTER TABLE contacts
  DROP INDEX idx_contacts_whatsapp_status,
  DROP COLUMN whatsapp_checked_at,
  DROP COLUMN whatsapp_status;

DROP TABLE IF EXISTS wa_messages;
