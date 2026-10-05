CREATE TABLE IF NOT EXISTS devices (
 id text PRIMARY KEY, name text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS telemetry (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 device_id text NOT NULL, position integer NOT NULL CHECK (position BETWEEN 0 AND 100),
 received_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON telemetry(device_id, received_at DESC);
