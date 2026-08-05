-- I-08: ingestion of WMS.PDA.WarehouseTask*.v1.
--
-- pda-backend owns task delivery and the operator session; it is never the
-- inventory authority (prompt section 13.1). This projection is therefore a
-- read model of WMS-owned state: it keeps the WMS task identity and version so
-- a stale or duplicate delivery can be rejected without a business effect.

CREATE TABLE wms_task_projection (
    warehouse_task_id text PRIMARY KEY,
    task_type text NOT NULL,
    task_version bigint NOT NULL CHECK (task_version > 0),
    status text NOT NULL,
    priority integer NOT NULL DEFAULT 5,

    site_id text NULL,
    warehouse_id text NOT NULL,
    source_location_id text NULL,
    destination_location_id text NULL,

    work_order_id text NULL,
    work_order_code text NULL,
    material_request_id text NULL,
    material_request_line_id text NULL,
    expected_receipt_id text NULL,

    item_id text NULL,
    item_code text NULL,
    revision_id text NULL,
    revision_code text NULL,

    requested_quantity numeric(18,6) NOT NULL DEFAULT 0,
    confirmed_quantity numeric(18,6) NOT NULL DEFAULT 0,
    remaining_quantity numeric(18,6) NOT NULL DEFAULT 0,
    uom_id text NULL,
    uom_code text NULL,

    lot_id text NULL,
    lot_code text NULL,
    serial_numbers jsonb NOT NULL DEFAULT '[]'::jsonb,
    scan_requirements jsonb NOT NULL DEFAULT '[]'::jsonb,

    correlation_id text NULL,
    causation_id text NULL,
    payload jsonb NOT NULL,
    source_event_id uuid NOT NULL,

    dispatched_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX wms_task_projection_work_order_idx
    ON wms_task_projection (work_order_id) WHERE work_order_id IS NOT NULL;
CREATE INDEX wms_task_projection_material_request_idx
    ON wms_task_projection (material_request_id) WHERE material_request_id IS NOT NULL;
CREATE INDEX wms_task_projection_correlation_idx
    ON wms_task_projection (correlation_id) WHERE correlation_id IS NOT NULL;

-- Records a delivery that was accepted by the inbox but carried a task version
-- at or below the one already projected. Kept for the section 22 UC-13
-- out-of-order assertion rather than silently discarded.
CREATE TABLE wms_task_stale_delivery (
    stale_id bigserial PRIMARY KEY,
    event_id uuid NOT NULL,
    warehouse_task_id text NOT NULL,
    incoming_version bigint NOT NULL,
    projected_version bigint NOT NULL,
    event_type text NOT NULL,
    observed_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX wms_task_stale_delivery_task_idx
    ON wms_task_stale_delivery (warehouse_task_id, observed_at DESC);
