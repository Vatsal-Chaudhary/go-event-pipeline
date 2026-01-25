ALTER TABLE raw_events 
ADD COLUMN risk_score INTEGER DEFAULT 0;

CREATE INDEX idx_raw_events_risk_score ON raw_events(risk_score);
