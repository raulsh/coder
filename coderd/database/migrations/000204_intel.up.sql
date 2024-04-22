CREATE TABLE intel_cohorts (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_by UUID NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	display_name TEXT NOT NULL,
	description TEXT NOT NULL,

	filter_operating_system TEXT NOT NULL,
	filter_operating_system_version TEXT NOT NULL,
	filter_architecture TEXT NOT NULL,
	filter_repositories TEXT NOT NULL,

	tracked_executables []TEXT NOT NULL
);

CREATE TABLE intel_machines (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	machine_id UUID NOT NULL,
	user_id UUID NOT NULL,
	ip_address TEXT NOT NULL,
	hostname TEXT NOT NULL,
	-- GOOS
	operating_system TEXT NOT NULL,
	operating_system_version TEXT NOT NULL,
	cpu_cores INT NOT NULL,
	memory_mb_total INT NOT NULL COMMENT 'in MB',
	architecture TEXT NOT NULL COMMENT 'GOARCH. e.g. amd64',
	daemon_version TEXT NOT NULL COMMENT 'Version of the daemon running on the machine',
	git_config_email TEXT NOT NULL COMMENT 'git config --get user.email',
	git_config_name TEXT NOT NULL COMMENT 'git config --get user.name',

	tags varchar(64)[] COMMENT 'Arbitrary user-defined tags. e.g. "coder-v1" or "coder-v2"'
);

COMMENT ON TABLE intel_machines IS 'Stores';

CREATE TABLE intel_invocations (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	machine_id UUID NOT NULL,
	user_id UUID NOT NULL,
	binary_hash TEXT NOT NULL,
	binary_path TEXT NOT NULL,
	binary_args TEXT NOT NULL,
	binary_version TEXT NOT NULL,
	working_directory TEXT NOT NULL,
	-- `git config --get remote.origin.url`
	git_remote_url TEXT NOT NULL,
	started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	ended_at TIMESTAMPTZ
);

CREATE TABLE intel_git_commits (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	invocation_id UUID NOT NULL,
	commit_hash TEXT NOT NULL,
	commit_message TEXT NOT NULL,
	commit_author TEXT NOT NULL,
	commit_author_email TEXT NOT NULL,
	commit_author_date TIMESTAMPTZ NOT NULL,
	commit_committer TEXT NOT NULL,
	commit_committer_email TEXT NOT NULL,
	commit_committer_date TIMESTAMPTZ NOT NULL
);

CREATE TABLE intel_path_executables (
	machine_id UUID PRIMARY KEY,
	user_id UUID NOT NULL,
	id uuid NOT NULL,
	hash TEXT NOT NULL,
	basename TEXT NOT NULL,
	version TEXT NOT NULL,
);
