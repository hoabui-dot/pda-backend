ALTER TABLE domain_outbox ADD COLUMN IF NOT EXISTS attempts integer NOT NULL DEFAULT 0;
ALTER TABLE domain_outbox ADD COLUMN IF NOT EXISTS available_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE domain_outbox ADD COLUMN IF NOT EXISTS last_error text NULL;
CREATE INDEX IF NOT EXISTS domain_outbox_pending_idx ON domain_outbox (available_at, created_at) WHERE published_at IS NULL;

CREATE TABLE event_inbox (
    event_id uuid NOT NULL,
    consumer_group text NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now(),
    attempts integer NOT NULL DEFAULT 1,
    last_error text NULL,
    PRIMARY KEY (event_id, consumer_group)
);

CREATE TABLE event_dlq (
    event_id uuid NOT NULL,
    consumer_group text NOT NULL,
    envelope_json jsonb NOT NULL,
    attempts integer NOT NULL,
    last_error text NOT NULL,
    failed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, consumer_group)
);
