BEGIN;

-- Drop indexes associated with the campaign_stats table
DROP INDEX IF EXISTS idx_campaign_stats_hour_bucket;

-- Drop the campaign_stats table
DROP TABLE IF EXISTS campaign_stats;

-- Drop indexes associated with the raw_events table
DROP INDEX IF EXISTS idx_raw_events_user_id;
DROP INDEX IF EXISTS idx_raw_events_event_type;
DROP INDEX IF EXISTS idx_raw_events_created_at;
DROP INDEX IF EXISTS idx_raw_events_campaign_id;

-- Drop the raw_events table
DROP TABLE IF EXISTS raw_events;

COMMIT;

