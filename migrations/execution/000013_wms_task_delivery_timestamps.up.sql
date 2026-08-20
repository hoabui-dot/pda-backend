ALTER TABLE wms_task_projection
    ADD COLUMN IF NOT EXISTS source_dispatched_at timestamptz NULL,
    ADD COLUMN IF NOT EXISTS backend_received_at timestamptz NOT NULL DEFAULT now();

CREATE INDEX IF NOT EXISTS wms_task_projection_delivery_time_idx
    ON wms_task_projection (backend_received_at DESC);
