-- Remove RSS UUID from subscriptions table
ALTER TABLE subscriptions DROP COLUMN rss_uuid;
