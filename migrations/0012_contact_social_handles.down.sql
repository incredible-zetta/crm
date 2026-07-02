ALTER TABLE contacts
  DROP INDEX idx_contacts_x_username,
  DROP INDEX idx_contacts_threads_username,
  DROP COLUMN x_username,
  DROP COLUMN threads_username;
