-- Durable acknowledgement that an authenticated device persisted a server
-- event. SSE remains at-least-once; this table is the server-side proof that
-- the device received a particular delivery, scoped to operator and device.
CREATE TABLE IF NOT EXISTS pda_event_delivery_ack (
    event_id text NOT NULL,
    operator_id text NOT NULL,
    device_id text NOT NULL,
    warehouse_id text NOT NULL,
    first_ack_at timestamptz NOT NULL DEFAULT now(),
    last_ack_at timestamptz NOT NULL DEFAULT now(),
    ack_count integer NOT NULL DEFAULT 1,
    last_correlation_id text NULL,
    PRIMARY KEY (event_id, operator_id, device_id)
);

CREATE INDEX IF NOT EXISTS pda_event_delivery_ack_operator_idx
    ON pda_event_delivery_ack (operator_id, last_ack_at DESC);
