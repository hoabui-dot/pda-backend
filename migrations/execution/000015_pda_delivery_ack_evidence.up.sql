ALTER TABLE pda_event_delivery_ack
  ADD COLUMN IF NOT EXISTS device_received_at timestamptz NULL,
  ADD COLUMN IF NOT EXISTS backend_recorded_at timestamptz NULL;

CREATE INDEX IF NOT EXISTS pda_event_delivery_ack_device_time_idx
  ON pda_event_delivery_ack (device_received_at DESC);
