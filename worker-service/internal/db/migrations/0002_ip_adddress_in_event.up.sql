ALTER TABLE raw_events 
ADD COLUMN ip_address VARCHAR(45);

CREATE INDEX idx_raw_events_ip_address ON raw_events(ip_address);

