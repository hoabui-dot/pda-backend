-- I-08 (outbound) — operator execution of a WMS-dispatched warehouse task.
--
-- pda-backend owns the operator session, the scan workflow, and the resulting
-- fact. It never decides inventory availability (prompt section 14.3): every
-- confirmed result is published back to WMS, and WMS applies it.

CREATE TABLE pda_task_execution (
    warehouse_task_id text PRIMARY KEY
        REFERENCES wms_task_projection(warehouse_task_id) ON DELETE CASCADE,
    pda_task_id uuid NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    task_version bigint NOT NULL,
    state text NOT NULL
        CHECK (state IN ('DISPATCHED','STARTED','COMPLETED','FAILED')),
    operator_id text NULL,
    device_id text NULL,
    confirmed_quantity numeric(18,6) NOT NULL DEFAULT 0,
    reason_code text NULL,
    started_at timestamptz NULL,
    finished_at timestamptz NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX pda_task_execution_state_idx ON pda_task_execution (state, updated_at DESC);

-- Scan evidence. The unique key makes a repeated identical scan a no-op rather
-- than a second row, so a flaky device radio cannot inflate the evidence trail.
CREATE TABLE pda_task_scan (
    scan_id bigserial PRIMARY KEY,
    warehouse_task_id text NOT NULL REFERENCES pda_task_execution(warehouse_task_id) ON DELETE CASCADE,
    scan_type text NOT NULL,
    scan_value text NOT NULL,
    scan_hash char(64) NOT NULL,
    accepted boolean NOT NULL,
    rejection_reason text NULL,
    operator_id text NULL,
    device_id text NULL,
    scanned_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (warehouse_task_id, scan_type, scan_hash)
);

CREATE INDEX pda_task_scan_task_idx ON pda_task_scan (warehouse_task_id, scanned_at);

-- Command-level idempotency for the operator API. A device that retries after a
-- timeout must not produce a second business effect.
CREATE TABLE pda_task_command (
    command_id uuid PRIMARY KEY,
    warehouse_task_id text NOT NULL,
    command_type text NOT NULL CHECK (command_type IN ('START','SCAN','COMPLETE','FAIL')),
    request_hash char(64) NOT NULL,
    result_json jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX pda_task_command_task_idx ON pda_task_command (warehouse_task_id, created_at);

-- Cross-system outbox. It is deliberately separate from domain_outbox: that
-- table carries pda-backend's internal camelCase envelope on pda.* topics,
-- whereas these rows carry the canonical snake_case integration envelope on
-- absolute topics owned by the contract, and must not be topic-prefixed.
CREATE TABLE integration_outbox (
    event_id uuid PRIMARY KEY,
    topic text NOT NULL,
    event_type text NOT NULL,
    aggregate_id text NOT NULL,
    aggregate_version bigint NOT NULL,
    partition_key text NOT NULL,
    envelope_json jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    available_at timestamptz NOT NULL DEFAULT now(),
    attempts integer NOT NULL DEFAULT 0,
    last_error text NULL,
    published_at timestamptz NULL
);

CREATE INDEX integration_outbox_pending_idx
    ON integration_outbox (available_at, created_at) WHERE published_at IS NULL;

CREATE TABLE integration_outbox_dead_letters (
    dead_letter_id bigserial PRIMARY KEY,
    event_id uuid NOT NULL,
    topic text NOT NULL,
    event_type text NOT NULL,
    envelope_json jsonb NOT NULL,
    attempts integer NOT NULL,
    last_error text NOT NULL,
    failed_at timestamptz NOT NULL DEFAULT now()
);
