ALTER TABLE wms_task_projection
    ADD COLUMN IF NOT EXISTS source_location_code text NULL,
    ADD COLUMN IF NOT EXISTS source_bin_id text NULL,
    ADD COLUMN IF NOT EXISTS source_bin_code text NULL,
    ADD COLUMN IF NOT EXISTS destination_location_code text NULL,
    ADD COLUMN IF NOT EXISTS destination_bin_id text NULL,
    ADD COLUMN IF NOT EXISTS destination_bin_code text NULL,
    ADD COLUMN IF NOT EXISTS lpn_code text NULL,
    ADD COLUMN IF NOT EXISTS allocations jsonb NOT NULL DEFAULT '[]'::jsonb;
