BEGIN;

-- Table 1: raw_events (The Firehose)
CREATE TABLE raw_events (
    event_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(20) NOT NULL CHECK (event_type IN ('impression', 'click', 'conversion')),
    user_id VARCHAR(30) NOT NULL,
    campaign_id VARCHAR(255) NOT NULL,
    geo_country VARCHAR(2) NOT NULL,
    device_type VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    meta_data JSONB
);

-- Indexes for raw_events
CREATE INDEX idx_raw_events_campaign_id ON raw_events(campaign_id);
CREATE INDEX idx_raw_events_created_at ON raw_events(created_at DESC);
CREATE INDEX idx_raw_events_event_type ON raw_events(event_type);
CREATE INDEX idx_raw_events_user_id ON raw_events(user_id);


-- Table 2: campaign_stats (The Aggregated Report)
CREATE TABLE campaign_stats (
    campaign_id VARCHAR(255) NOT NULL,
    hour_bucket TIMESTAMP NOT NULL,
    total_impressions INTEGER NOT NULL DEFAULT 0,
    total_clicks INTEGER NOT NULL DEFAULT 0,
    total_conversions INTEGER NOT NULL DEFAULT 0,
    
    -- Composite primary key ensures only one row per campaign per hour
    PRIMARY KEY (campaign_id, hour_bucket)
);

-- Index for time-based dashboard queries
CREATE INDEX idx_campaign_stats_hour_bucket ON campaign_stats(hour_bucket DESC);

COMMIT;
