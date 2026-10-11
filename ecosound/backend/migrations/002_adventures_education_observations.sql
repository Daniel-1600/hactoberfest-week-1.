CREATE TABLE adventures (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  description TEXT NOT NULL,
  duration_minutes INTEGER NOT NULL CHECK(duration_minutes BETWEEN 10 AND 60),
  interest TEXT NOT NULL,
  environment TEXT NOT NULL,
  difficulty TEXT NOT NULL,
  generation_source TEXT NOT NULL CHECK(generation_source IN ('qwen','fallback','destination')),
  destination_id TEXT,
  created_at TEXT NOT NULL
);
CREATE TABLE adventure_tasks (
  id TEXT NOT NULL,
  adventure_id TEXT NOT NULL REFERENCES adventures(id) ON DELETE CASCADE,
  task_type TEXT NOT NULL CHECK(task_type IN ('outdoor_observation','audio_discovery','species_discovery','reflection')),
  title TEXT NOT NULL,
  instructions TEXT NOT NULL,
  xp INTEGER NOT NULL CHECK(xp BETWEEN 0 AND 100),
  ordinal INTEGER NOT NULL,
  PRIMARY KEY(adventure_id,id), UNIQUE(adventure_id,ordinal)
);
CREATE TABLE adventure_progress (
  adventure_id TEXT PRIMARY KEY REFERENCES adventures(id) ON DELETE CASCADE,
  status TEXT NOT NULL CHECK(status IN ('ready','active','completed')) DEFAULT 'ready',
  started_at TEXT,
  completed_at TEXT,
  self_reported_minutes INTEGER CHECK(self_reported_minutes IS NULL OR self_reported_minutes BETWEEN 0 AND 240),
  reflection_discovered TEXT,
  reflection_surprised TEXT,
  reflection_again INTEGER CHECK(reflection_again IS NULL OR reflection_again IN (0,1))
);
CREATE TABLE adventure_task_progress (
  adventure_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  completed_at TEXT NOT NULL,
  completion_source TEXT NOT NULL CHECK(completion_source IN ('self_reported','saved_discovery')),
  discovery_id INTEGER REFERENCES discoveries(id),
  PRIMARY KEY(adventure_id,task_id),
  FOREIGN KEY(adventure_id,task_id) REFERENCES adventure_tasks(adventure_id,id)
);
CREATE UNIQUE INDEX adventure_discovery_credit ON adventure_task_progress(discovery_id) WHERE discovery_id IS NOT NULL;
CREATE TABLE xp_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  event_key TEXT NOT NULL UNIQUE,
  points INTEGER NOT NULL CHECK(points BETWEEN 0 AND 100),
  created_at TEXT NOT NULL
);
CREATE TABLE badges (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL
);
CREATE TABLE earned_badges (
  badge_id TEXT NOT NULL REFERENCES badges(id),
  earned_at TEXT NOT NULL,
  PRIMARY KEY(badge_id)
);
INSERT INTO badges(id,name,description) VALUES
 ('first-discovery','First Discovery','Saved your first wildlife discovery.'),
 ('bird-listener','Bird Listener','Saved five distinct bird recordings.'),
 ('nature-explorer','Nature Explorer','Completed your first outdoor adventure.'),
 ('forest-adventurer','Forest Adventurer','Completed a forest adventure.'),
 ('five-species','Five Species Discovered','Saved five distinct recognized species.'),
 ('seven-adventures','Seven Adventures Completed','Completed seven adventures.');

CREATE TABLE species_knowledge (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  scientific_name TEXT NOT NULL UNIQUE,
  common_name TEXT NOT NULL,
  habitat TEXT NOT NULL,
  diet TEXT NOT NULL,
  ecological_role TEXT NOT NULL,
  conservation_information TEXT NOT NULL,
  ethical_guidance TEXT NOT NULL,
  source_name TEXT NOT NULL,
  source_url TEXT NOT NULL,
  source_license TEXT NOT NULL,
  sensitive INTEGER NOT NULL DEFAULT 0 CHECK(sensitive IN (0,1)),
  updated_at TEXT NOT NULL
);
CREATE TABLE destinations (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  type TEXT NOT NULL,
  region TEXT NOT NULL,
  country TEXT NOT NULL,
  website TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL CHECK(status IN ('verified','concept','inactive')),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE trails (
  id TEXT PRIMARY KEY,
  destination_id TEXT NOT NULL REFERENCES destinations(id),
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  distance_km REAL,
  estimated_duration_minutes INTEGER,
  difficulty TEXT NOT NULL,
  geometry_geojson TEXT,
  accessibility_notes TEXT NOT NULL,
  status TEXT NOT NULL CHECK(status IN ('verified','unverified','inactive'))
);
CREATE TABLE experiences (
  id TEXT PRIMARY KEY,
  destination_id TEXT NOT NULL REFERENCES destinations(id),
  trail_id TEXT REFERENCES trails(id),
  title TEXT NOT NULL,
  description TEXT NOT NULL,
  duration_minutes INTEGER NOT NULL CHECK(duration_minutes BETWEEN 10 AND 240),
  difficulty TEXT NOT NULL,
  experience_type TEXT NOT NULL,
  approved_content TEXT NOT NULL,
  status TEXT NOT NULL CHECK(status IN ('approved','concept','inactive'))
);
INSERT INTO destinations(id,name,description,type,region,country,website,status,created_at,updated_at)
VALUES('karura-concept','Karura Forest','Concept demonstration for a route-free forest listening experience. Follow only routes and visitor guidance published by the destination manager.','forest','Nairobi','Kenya','https://www.kenyaforestservice.org/','concept',strftime('%Y-%m-%dT%H:%M:%fZ','now'),strftime('%Y-%m-%dT%H:%M:%fZ','now'));
INSERT INTO experiences(id,destination_id,trail_id,title,description,duration_minutes,difficulty,experience_type,approved_content,status)
VALUES('karura-hidden-voices','karura-concept',NULL,'Karura Forest — Hidden Voices Adventure','Concept demonstration — not affiliated with Karura Forest management. Route-free listening prompts only; consult current official visitor information and use permitted trails.','30','beginner','birdwatching_adventure','Begin with a brief quiet listening pause, record wildlife from a safe distance, and review BirdNET suggestions later. Never leave a permitted public route or approach wildlife. No trail is specified.','concept');

CREATE TABLE observations (
  id TEXT PRIMARY KEY,
  discovery_id INTEGER NOT NULL UNIQUE REFERENCES discoveries(id),
  observed_at TEXT NOT NULL,
  latitude REAL,
  longitude REAL,
  public_latitude REAL,
  public_longitude REAL,
  location_accuracy_meters REAL,
  location_source TEXT NOT NULL CHECK(location_source IN ('device','manual','none')),
  verification_status TEXT NOT NULL DEFAULT 'ai_predicted' CHECK(verification_status IN ('ai_predicted','user_confirmed','expert_verified','rejected')),
  visibility TEXT NOT NULL CHECK(visibility IN ('public','private')),
  consent_version TEXT NOT NULL,
  created_at TEXT NOT NULL,
  CHECK((latitude IS NULL AND longitude IS NULL AND public_latitude IS NULL AND public_longitude IS NULL) OR (latitude BETWEEN -90 AND 90 AND longitude BETWEEN -180 AND 180 AND public_latitude BETWEEN -90 AND 90 AND public_longitude BETWEEN -180 AND 180))
);
CREATE TABLE sync_operations (
  operation_id TEXT PRIMARY KEY,
  operation_type TEXT NOT NULL,
  payload_hash TEXT NOT NULL,
  result_json TEXT NOT NULL,
  created_at TEXT NOT NULL
);
