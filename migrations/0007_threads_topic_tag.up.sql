ALTER TABLE threads_posts ADD COLUMN topic_tag VARCHAR(50) NULL AFTER username;
CREATE INDEX idx_threads_posts_topic_tag ON threads_posts (topic_tag);
