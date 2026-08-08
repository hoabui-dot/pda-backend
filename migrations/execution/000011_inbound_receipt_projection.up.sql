-- The PDA gateway observes the authoritative Inbound receipt-confirmation fact.
-- This is a read model only; Inventory and Inbound remain the owners of the
-- business mutation.
CREATE TABLE receiving_work_projection (
    receipt_id text PRIMARY KEY,
    warehouse_location_id text NOT NULL,
    status text NOT NULL,
    projection_version bigint NOT NULL DEFAULT 1 CHECK (projection_version > 0),
    source_event_id uuid NOT NULL UNIQUE,
    correlation_id text NULL,
    trace_id text NULL,
    payload jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX receiving_work_projection_status_idx
    ON receiving_work_projection (status, updated_at DESC);
