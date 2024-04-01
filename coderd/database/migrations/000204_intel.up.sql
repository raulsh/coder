CREATE TABLE intel_config_executables ();

CREATE TABLE intel_machines (
	user_id UUID NOT NULL,
	machine_id UUID PRIMARY KEY,
	ip_address TEXT NOT NULL,
	hostname TEXT NOT NULL,
	-- GOOS
	operating_system TEXT NOT NULL,
	-- Likely not a standardized string
	operating_system_version TEXT NOT NULL,
	cpu_cores INT NOT NULL,
	memory_total INT NOT NULL,
	-- GOARCH
	architecture TEXT NOT NULL,
	daemon_version TEXT NOT NULL,
	git_config_email TEXT NOT NULL,
	git_config_name TEXT NOT NULL,
	tags TEXT[] NOT NULL
);

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
