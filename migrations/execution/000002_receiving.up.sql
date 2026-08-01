CREATE TABLE receiving_task (
    task_id text PRIMARY KEY REFERENCES warehouse_task(task_id),
    po_number text NOT NULL,
    status text NOT NULL,
    warehouse_id text NOT NULL,
    operator_id text NULL,
    version bigint NOT NULL CHECK (version > 0),
    allow_over_receipt boolean NOT NULL DEFAULT false,
    require_remark_on_variance boolean NOT NULL DEFAULT true,
    updated_at timestamptz NOT NULL
);
CREATE TABLE receiving_line (
    line_id text PRIMARY KEY,
    task_id text NOT NULL REFERENCES receiving_task(task_id) ON DELETE CASCADE,
    item_id text NOT NULL,
    item_name text NOT NULL,
    barcode text NOT NULL,
    expected_quantity bigint NOT NULL CHECK (expected_quantity > 0),
    received_quantity bigint NOT NULL DEFAULT 0 CHECK (received_quantity >= 0),
    UNIQUE(task_id, barcode)
);
CREATE INDEX receiving_task_list_idx ON receiving_task(warehouse_id, operator_id, status, task_id);
CREATE INDEX receiving_line_barcode_idx ON receiving_line(barcode);

CREATE TABLE inventory_balance (
    warehouse_id text NOT NULL,
    item_id text NOT NULL,
    quantity bigint NOT NULL DEFAULT 0,
    version bigint NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY(warehouse_id, item_id)
);
CREATE TABLE receiving_command_status (
    command_id uuid PRIMARY KEY,
    idempotency_key text NOT NULL UNIQUE,
    command_type text NOT NULL,
    status text NOT NULL,
    warehouse_id text NOT NULL,
    operator_id text NOT NULL,
    result_json jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE audit_record (
    audit_id uuid PRIMARY KEY,
    action text NOT NULL,
    outcome text NOT NULL,
    aggregate_id text NOT NULL,
    operator_id text NOT NULL,
    device_id text NOT NULL,
    warehouse_id text NOT NULL,
    correlation_id text NOT NULL,
    occurred_at timestamptz NOT NULL,
    details_json jsonb NOT NULL
);
