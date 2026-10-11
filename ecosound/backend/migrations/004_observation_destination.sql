ALTER TABLE observations ADD COLUMN destination_id TEXT REFERENCES destinations(id);
CREATE INDEX observations_destination ON observations(destination_id);
