CREATE TABLE movement_task (
    task_id text PRIMARY KEY REFERENCES warehouse_task(task_id),
    workflow text NOT NULL CHECK (workflow IN ('PUTAWAY','PICKING','REPLENISHMENT')),
    status text NOT NULL,
    warehouse_id text NOT NULL,
    operator_id text NULL,
    version bigint NOT NULL CHECK (version > 0),
    source_location text NOT NULL,
    destination_location text NOT NULL,
    item_id text NOT NULL,
    barcode text NOT NULL,
    required_quantity bigint NOT NULL CHECK (required_quantity > 0),
    completed_quantity bigint NOT NULL DEFAULT 0 CHECK (completed_quantity >= 0),
    source_validated boolean NOT NULL DEFAULT false,
    destination_validated boolean NOT NULL DEFAULT false,
    item_validated boolean NOT NULL DEFAULT false,
    updated_at timestamptz NOT NULL
);
CREATE INDEX movement_task_list_idx ON movement_task(workflow,warehouse_id,operator_id,status,task_id);
CREATE TABLE warehouse_location (
    warehouse_id text NOT NULL, location_id text NOT NULL, zone text NOT NULL,
    capacity bigint NOT NULL CHECK (capacity >= 0), used_capacity bigint NOT NULL DEFAULT 0 CHECK (used_capacity >= 0),
    compatible_item_id text NULL, PRIMARY KEY(warehouse_id,location_id)
);
CREATE TABLE inventory_location_balance (
    warehouse_id text NOT NULL, location_id text NOT NULL, item_id text NOT NULL,
    quantity bigint NOT NULL CHECK (quantity >= 0), version bigint NOT NULL DEFAULT 1, updated_at timestamptz NOT NULL,
    PRIMARY KEY(warehouse_id,location_id,item_id)
);
CREATE TABLE inventory_movement (
    movement_id uuid PRIMARY KEY, task_id text NOT NULL, workflow text NOT NULL, warehouse_id text NOT NULL,
    item_id text NOT NULL, source_location text NOT NULL, destination_location text NOT NULL,
    quantity bigint NOT NULL CHECK (quantity > 0), aggregate_version bigint NOT NULL, occurred_at timestamptz NOT NULL,
    UNIQUE(task_id,aggregate_version)
);
CREATE TABLE movement_command_status (
    command_id uuid PRIMARY KEY, idempotency_key text NOT NULL UNIQUE, workflow text NOT NULL,
    command_type text NOT NULL, warehouse_id text NOT NULL, operator_id text NOT NULL,
    result_json jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
