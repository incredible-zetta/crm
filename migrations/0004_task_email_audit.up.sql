ALTER TABLE scheduled_tasks
  MODIFY COLUMN kind ENUM('email','campaign','email_audit') NOT NULL;
