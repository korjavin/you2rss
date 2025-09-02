CREATE TABLE subscriptions (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    youtube_channel_id VARCHAR(255) NOT NULL,
    youtube_channel_title VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, youtube_channel_id) -- Prevent duplicate subscriptions
);
