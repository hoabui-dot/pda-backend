-- Section 8/19: the operator device scopes its queue by warehouse code, while
-- WMS keys the warehouse by UUID. The dispatch carries both; store both.
ALTER TABLE wms_task_projection ADD COLUMN IF NOT EXISTS warehouse_code text;
CREATE INDEX IF NOT EXISTS wms_task_projection_warehouse_code_idx
    ON wms_task_projection (warehouse_code) WHERE warehouse_code IS NOT NULL;
