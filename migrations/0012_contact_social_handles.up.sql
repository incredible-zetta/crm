ALTER TABLE contacts
  ADD COLUMN x_username VARCHAR(255) AFTER whatsapp_checked_at,
  ADD COLUMN threads_username VARCHAR(255) AFTER x_username,
  ADD INDEX idx_contacts_x_username (x_username),
  ADD INDEX idx_contacts_threads_username (threads_username);
