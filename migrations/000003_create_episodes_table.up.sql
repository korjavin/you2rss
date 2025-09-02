CREATE TABLE episodes (
    id SERIAL PRIMARY KEY,
    subscription_id INTEGER NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    youtube_video_id VARCHAR(255) NOT NULL UNIQUE,
    title TEXT,
    description TEXT,
    published_at TIMESTAMPTZ,
    audio_uuid UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    audio_path VARCHAR(1024),
    audio_size_bytes BIGINT,
    duration_seconds INTEGER,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING', -- PENDING, PROCESSING, COMPLETED, FAILED
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
