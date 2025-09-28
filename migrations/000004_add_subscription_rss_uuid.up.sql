-- Add RSS UUID to subscriptions table for individual feed URLs
ALTER TABLE subscriptions ADD COLUMN rss_uuid UUID NOT NULL UNIQUE DEFAULT gen_random_uuid();
