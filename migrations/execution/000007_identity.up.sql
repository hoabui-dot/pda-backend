CREATE TABLE identity_operators (
    id text PRIMARY KEY,
    username text NOT NULL UNIQUE,
    employee_code text NOT NULL UNIQUE,
    display_name text NOT NULL,
    password_hash text NOT NULL,
    status text NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED','LOCKED')),
    shift_code text NOT NULL DEFAULT '',
    failed_login_count integer NOT NULL DEFAULT 0,
    locked_until timestamptz NULL,
    password_changed_at timestamptz NOT NULL DEFAULT now(),
    last_login_at timestamptz NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1
);
CREATE TABLE identity_roles (
    role_id bigserial PRIMARY KEY,
    role_code text NOT NULL UNIQUE,
    role_name text NOT NULL,
    active boolean NOT NULL DEFAULT true
);
CREATE TABLE identity_permissions (
    permission_id bigserial PRIMARY KEY,
    permission_code text NOT NULL UNIQUE,
    permission_name text NOT NULL
);
CREATE TABLE identity_operator_roles (
    operator_id text NOT NULL REFERENCES identity_operators(id) ON DELETE CASCADE,
    role_id bigint NOT NULL REFERENCES identity_roles(role_id) ON DELETE CASCADE,
    PRIMARY KEY(operator_id, role_id)
);
CREATE TABLE identity_role_permissions (
    role_id bigint NOT NULL REFERENCES identity_roles(role_id) ON DELETE CASCADE,
    permission_id bigint NOT NULL REFERENCES identity_permissions(permission_id) ON DELETE CASCADE,
    PRIMARY KEY(role_id, permission_id)
);
CREATE TABLE identity_warehouses (
    warehouse_id text PRIMARY KEY,
    warehouse_name text NOT NULL
);
CREATE TABLE identity_operator_warehouses (
    operator_id text NOT NULL REFERENCES identity_operators(id) ON DELETE CASCADE,
    warehouse_id text NOT NULL REFERENCES identity_warehouses(warehouse_id) ON DELETE CASCADE,
    is_default boolean NOT NULL DEFAULT false,
    active boolean NOT NULL DEFAULT true,
    PRIMARY KEY(operator_id, warehouse_id)
);
CREATE TABLE identity_devices (
    device_code text PRIMARY KEY,
    device_model text NOT NULL DEFAULT '',
    app_version text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','DISABLED','REVOKED')),
    approved_at timestamptz NULL,
    revoked_at timestamptz NULL,
    last_seen_at timestamptz NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    version bigint NOT NULL DEFAULT 1
);
CREATE TABLE identity_operator_devices (
    operator_id text NOT NULL REFERENCES identity_operators(id) ON DELETE CASCADE,
    device_code text NOT NULL REFERENCES identity_devices(device_code) ON DELETE CASCADE,
    warehouse_id text NOT NULL REFERENCES identity_warehouses(warehouse_id) ON DELETE CASCADE,
    active boolean NOT NULL DEFAULT true,
    PRIMARY KEY(operator_id, device_code, warehouse_id)
);
CREATE TABLE identity_sessions (
    id uuid PRIMARY KEY,
    operator_id text NOT NULL REFERENCES identity_operators(id),
    device_code text NULL,
    warehouse_id text NULL,
    status text NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','REVOKED','EXPIRED')),
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz NULL,
    revocation_reason text NULL,
    version bigint NOT NULL DEFAULT 1
);
CREATE INDEX identity_sessions_operator_idx ON identity_sessions(operator_id, status);
CREATE TABLE identity_refresh_tokens (
    id uuid PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES identity_sessions(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    token_family_id uuid NOT NULL,
    parent_token_id uuid NULL REFERENCES identity_refresh_tokens(id),
    issued_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz NULL,
    revoked_at timestamptz NULL,
    replaced_by_token_id uuid NULL REFERENCES identity_refresh_tokens(id),
    reuse_detected_at timestamptz NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX identity_refresh_session_idx ON identity_refresh_tokens(session_id);
CREATE TABLE identity_security_audit (
    id bigserial PRIMARY KEY,
    event_type text NOT NULL,
    operator_id text NULL,
    session_id uuid NULL,
    device_code text NULL,
    warehouse_id text NULL,
    correlation_id text NULL,
    outcome text NOT NULL,
    safe_error_code text NULL,
    source_ip_hash text NULL,
    metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
