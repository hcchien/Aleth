-- Upgrade reaction_count INT → reaction_counts JSONB.
--
-- The INT counter (added in 00010) tracked only a grand total, so the feed
-- service had to fall back to showing all reactions as "like". The JSONB
-- column stores per-emotion counts {"like":5,"love":2,...} and lets the feed
-- service return a proper breakdown without any extra query.
--
-- The Counter Service will switch from ±1 INT ops to a full recompute of the
-- JSONB on every reaction event, keeping eventual-consistency guarantees.

-- 1. Add the new column (default empty object = no reactions yet).
ALTER TABLE posts
    ADD COLUMN reaction_counts JSONB NOT NULL DEFAULT '{}';

-- 2. Back-fill from the source of truth (post_likes).
UPDATE posts p
SET reaction_counts = (
    SELECT COALESCE(jsonb_object_agg(emotion, cnt), '{}')
    FROM (
        SELECT emotion, COUNT(*) AS cnt
        FROM post_likes
        WHERE post_id = p.id
        GROUP BY emotion
    ) t
);

-- 3. Drop the old INT counter (no longer needed).
ALTER TABLE posts
    DROP COLUMN reaction_count;
