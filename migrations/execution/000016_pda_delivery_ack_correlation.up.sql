ALTER TABLE pda_event_delivery_ack
  ADD COLUMN IF NOT EXISTS evidence_event_id uuid;

CREATE UNIQUE INDEX IF NOT EXISTS pda_event_delivery_ack_evidence_event_uidx
  ON pda_event_delivery_ack (evidence_event_id)
  WHERE evidence_event_id IS NOT NULL;
