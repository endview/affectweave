CREATE TABLE IF NOT EXISTS aw.meta (key text PRIMARY KEY, value text NOT NULL);
CREATE TABLE IF NOT EXISTS aw.roles (
 id text PRIMARY KEY, revision bigint NOT NULL, persona jsonb NOT NULL,
 spaces jsonb NOT NULL, updated_at timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS aw.states (
 scope text PRIMARY KEY, core_id text NOT NULL, space_id text NOT NULL,
 role_id text NOT NULL REFERENCES aw.roles(id), actor_id text NOT NULL,
 revision bigint NOT NULL, state jsonb NOT NULL,
 UNIQUE(core_id,space_id,role_id,actor_id)
);
CREATE TABLE IF NOT EXISTS aw.inputs (
 scope text NOT NULL REFERENCES aw.states(scope) ON DELETE CASCADE,
 input_id text NOT NULL, source_hash text NOT NULL, run_id text NOT NULL,
 snapshot jsonb NOT NULL, context_snapshot jsonb NOT NULL, occurred_at timestamptz NOT NULL,
 PRIMARY KEY(scope,input_id)
);
CREATE TABLE IF NOT EXISTS aw.memories (
 scope text NOT NULL REFERENCES aw.states(scope) ON DELETE CASCADE,
 id text NOT NULL, body jsonb NOT NULL, created_at timestamptz NOT NULL,
 PRIMARY KEY(scope,id)
);
CREATE TABLE IF NOT EXISTS aw.facts (
 scope text NOT NULL REFERENCES aw.states(scope) ON DELETE CASCADE,
 id text NOT NULL, kind text NOT NULL, body jsonb NOT NULL,
 occurred_at timestamptz NOT NULL, PRIMARY KEY(scope,id)
);
CREATE TABLE IF NOT EXISTS aw.operations (
 id text PRIMARY KEY, digest text NOT NULL, request_hash text NOT NULL,
 request jsonb NOT NULL, result jsonb NOT NULL, applied_at timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS aw.progress (
 core_id text PRIMARY KEY, boundary jsonb NOT NULL
);

ALTER TABLE aw.inputs ADD COLUMN IF NOT EXISTS context_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb;
