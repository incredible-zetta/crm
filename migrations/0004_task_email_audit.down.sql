ALTER TABLE scheduled_tasks
  MODIFY COLUMN kind ENUM('email','campaign') NOT NULL;
