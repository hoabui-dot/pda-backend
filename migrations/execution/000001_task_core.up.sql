CREATE TABLE warehouse_task (
    task_id text PRIMARY KEY,
    category text NOT NULL,
    status text NOT NULL,
    priority integer NOT NULL CHECK (priority BETWEEN 0 AND 100),
    warehouse_id text NOT NULL,
    operator_id text NULL,
    version bigint NOT NULL CHECK (version > 0),
    updated_at timestamptz NOT NULL
);
CREATE INDEX warehouse_task_query_idx ON warehouse_task (warehouse_id, category, status, task_id);
CREATE INDEX warehouse_task_operator_idx ON warehouse_task (warehouse_id, operator_id, status);

CREATE TABLE command_idempotency (
    idempotency_key text PRIMARY KEY,
    result_json jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE domain_outbox (
    event_id uuid PRIMARY KEY,
    aggregate_id text NOT NULL,
    aggregate_version bigint NOT NULL,
    event_type text NOT NULL,
    envelope_json jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz NULL
);
