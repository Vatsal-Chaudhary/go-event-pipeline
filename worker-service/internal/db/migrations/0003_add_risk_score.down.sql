DROP INDEX IF EXISTS idx_raw_events_risk_score;

ALTER TABLE raw_events 
DROP COLUMN IF EXISTS risk_score;
