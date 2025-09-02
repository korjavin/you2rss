CREATE TABLE users (
    id BIGINT PRIMARY KEY, -- Telegram User ID
    telegram_username VARCHAR(255),
    rss_uuid UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
