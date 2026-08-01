CREATE TABLE stock_transfer (
 command_id uuid PRIMARY KEY, idempotency_key text NOT NULL UNIQUE, warehouse_id text NOT NULL, operator_id text NOT NULL,
 source_location text NOT NULL, destination_location text NOT NULL, item_id text NOT NULL, quantity bigint NOT NULL CHECK(quantity>0),
 status text NOT NULL, result_json jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE cycle_count_task (
 task_id text PRIMARY KEY REFERENCES warehouse_task(task_id), warehouse_id text NOT NULL, operator_id text NULL,
 location_id text NOT NULL, status text NOT NULL, version bigint NOT NULL CHECK(version>0), updated_at timestamptz NOT NULL
);
CREATE TABLE cycle_count_line (
 line_id text PRIMARY KEY, task_id text NOT NULL REFERENCES cycle_count_task(task_id) ON DELETE CASCADE,
 item_id text NOT NULL, expected_quantity bigint NOT NULL CHECK(expected_quantity>=0), counted_quantity bigint NULL CHECK(counted_quantity>=0),
 variance bigint NULL, recount_required boolean NOT NULL DEFAULT false, UNIQUE(task_id,item_id)
);
CREATE TABLE inventory_command_status (
 command_id uuid PRIMARY KEY, idempotency_key text NOT NULL UNIQUE, command_type text NOT NULL,
 warehouse_id text NOT NULL, operator_id text NOT NULL, result_json jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX inventory_movement_history_idx ON inventory_movement(warehouse_id,item_id,occurred_at DESC,movement_id);
CREATE INDEX cycle_count_list_idx ON cycle_count_task(warehouse_id,operator_id,status,task_id);
