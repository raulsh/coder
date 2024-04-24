CREATE TABLE intel_cohorts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL,
	created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
	name TEXT NOT NULL,
    display_name TEXT NOT NULL,
    icon character varying(256) DEFAULT ''::character varying NOT NULL,
    description TEXT NOT NULL,

    regex_operating_system VARCHAR(255) NOT NULL DEFAULT '.*',
	regex_operating_system_platform VARCHAR(255) NOT NULL DEFAULT '.*',
    regex_operating_system_version VARCHAR(255) NOT NULL DEFAULT '.*',
    regex_architecture VARCHAR(255) NOT NULL DEFAULT '.*',
	regex_instance_id VARCHAR(255) NOT NULL DEFAULT '.*',

    tracked_executables TEXT[] NOT NULL
);

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

CREATE TABLE intel_invocations (
    id uuid NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
    machine_id uuid NOT NULL REFERENCES intel_machines(id) ON DELETE CASCADE,
    user_id uuid NOT NULL,
    binary_hash TEXT NOT NULL,
    binary_path TEXT NOT NULL,
    binary_args jsonb NOT NULL,
    binary_version TEXT NOT NULL,
    working_directory TEXT NOT NULL,
    git_remote_url TEXT NOT NULL,
	exit_code INT NOT NULL,
    duration_ms INT NOT NULL
);
