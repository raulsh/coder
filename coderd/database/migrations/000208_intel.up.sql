CREATE TABLE intel_cohorts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL,
	created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
	name TEXT NOT NULL,
    icon character varying(256) DEFAULT ''::character varying NOT NULL,
    description TEXT NOT NULL,

    regex_operating_system VARCHAR(255) NOT NULL DEFAULT '.*',
	regex_operating_system_platform VARCHAR(255) NOT NULL DEFAULT '.*',
    regex_operating_system_version VARCHAR(255) NOT NULL DEFAULT '.*',
    regex_architecture VARCHAR(255) NOT NULL DEFAULT '.*',
	regex_instance_id VARCHAR(255) NOT NULL DEFAULT '.*',
    tracked_executables TEXT[] NOT NULL,

	UNIQUE(organization_id, name)
);

CREATE INDEX idx_intel_cohorts_id ON intel_cohorts (id);

CREATE TABLE intel_machines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    instance_id TEXT NOT NULL,
	organization_id UUID NOT NULL,
    user_id UUID NOT NULL,
    ip_address inet DEFAULT '0.0.0.0'::inet NOT NULL,
    hostname TEXT NOT NULL,
    operating_system VARCHAR(255) NOT NULL,
    operating_system_version VARCHAR(255) NOT NULL,
	operating_system_platform VARCHAR(255) NOT NULL,
    cpu_cores INT NOT NULL,
    memory_mb_total INT NOT NULL,
    architecture VARCHAR(255) NOT NULL,
    daemon_version VARCHAR(255) NOT NULL,
	UNIQUE (user_id, instance_id)
);

COMMENT ON COLUMN intel_machines.operating_system IS 'GOOS';
COMMENT ON COLUMN intel_machines.memory_mb_total IS 'in MB';
COMMENT ON COLUMN intel_machines.architecture IS 'GOARCH. e.g. amd64';
COMMENT ON COLUMN intel_machines.daemon_version IS 'Version of the daemon running on the machine';

-- UNLOGGED because it is extremely update-heavy and the data is not valuable enough to justify
-- the overhead of WAL logging.
CREATE UNLOGGED TABLE intel_invocations (
    id uuid NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
    machine_id uuid NOT NULL REFERENCES intel_machines(id) ON DELETE CASCADE,
    user_id uuid NOT NULL,
    binary_hash TEXT NOT NULL,
	binary_name TEXT NOT NULL,
    binary_path TEXT NOT NULL,
    binary_args jsonb NOT NULL,
    binary_version TEXT NOT NULL,
    working_directory TEXT NOT NULL,
    git_remote_url TEXT NOT NULL,
	exit_code INT NOT NULL,
    duration_ms FLOAT NOT NULL
);

CREATE INDEX idx_intel_invocations_id ON intel_invocations (id);
CREATE INDEX idx_intel_invocations_created_at ON intel_invocations (created_at);
CREATE INDEX idx_intel_invocations_machine_id ON intel_invocations (machine_id);
CREATE INDEX idx_intel_invocations_binary_name ON intel_invocations (binary_name);
CREATE INDEX idx_intel_invocations_binary_args ON intel_invocations USING gin (binary_args);

-- Stores summaries for hour intervals of invocations.
-- There are so many invocations that we need to summarize them to make querying them feasible.

-- Summarize invocations by timestamp and cohort.
-- There are so many invocations that we need to summarize them to make querying feasible.
CREATE TABLE intel_invocation_summaries (
  id uuid NOT NULL,
  cohort_id uuid NOT NULL REFERENCES intel_cohorts(id) ON DELETE CASCADE,
  starts_at TIMESTAMPTZ NOT NULL,
  ends_at TIMESTAMPTZ NOT NULL,
  binary_name TEXT NOT NULL,
  binary_args jsonb NOT NULL,
  binary_paths jsonb NOT NULL,
  working_directories jsonb NOT NULL,
  git_remote_urls jsonb NOT NULL,
  exit_codes jsonb NOT NULL,
  unique_machines BIGINT NOT NULL,
  total_invocations BIGINT NOT NULL,
  median_duration_ms FLOAT NOT NULL
);

