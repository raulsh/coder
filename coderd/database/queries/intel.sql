-- name: UpsertIntelCohort :one
INSERT INTO intel_cohorts (id, organization_id, created_by, created_at, updated_at, name, icon, description, regex_operating_system, regex_operating_system_platform, regex_operating_system_version, regex_architecture, regex_instance_id, tracked_executables)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	ON CONFLICT (id) DO UPDATE SET
		updated_at = $5,
		name = $6,
		icon = $7,
		description = $8,
		regex_operating_system = $9,
		regex_operating_system_platform = $10,
		regex_operating_system_version = $11,
		regex_architecture = $12,
		regex_instance_id = $13,
		tracked_executables = $14
	RETURNING *;

-- name: GetIntelCohortsByOrganizationID :many
SELECT * FROM intel_cohorts WHERE organization_id = $1 AND (@name IS NULL OR name = @name);

-- name: DeleteIntelCohortsByIDs :exec
DELETE FROM intel_cohorts WHERE id = ANY(@cohort_ids::uuid[]);

-- name: UpsertIntelMachine :one
INSERT INTO intel_machines (id, created_at, updated_at, instance_id, organization_id, user_id, ip_address, hostname, operating_system, operating_system_platform, operating_system_version, cpu_cores, memory_mb_total, architecture, daemon_version)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	ON CONFLICT (user_id, instance_id) DO UPDATE SET
		updated_at = $3,
		ip_address = $7,
		hostname = $8,
		operating_system = $9,
		operating_system_platform = $10,
		operating_system_version = $11,
		cpu_cores = $12,
		memory_mb_total = $13,
		architecture = $14,
		daemon_version = $15
	RETURNING *;

-- name: InsertIntelInvocations :exec
-- Insert many invocations using unnest
INSERT INTO intel_invocations (
	created_at, machine_id, user_id, id, binary_name, binary_hash, binary_path, binary_args,
	binary_version, working_directory, git_remote_url, exit_code, duration_ms)
SELECT
	@created_at :: timestamptz as created_at,
	@machine_id :: uuid as machine_id,
	@user_id :: uuid as user_id,
	unnest(@id :: uuid[ ]) as id,
	unnest(@binary_name :: text[ ]) as binary_name,
	unnest(@binary_hash :: text[ ]) as binary_hash,
	unnest(@binary_path :: text[ ]) as binary_path,
	-- This has to be jsonb because PostgreSQL does not support parsing
	-- multi-dimensional multi-length arrays!
	jsonb_array_elements(@binary_args :: jsonb) as binary_args,
	unnest(@binary_version :: text[ ]) as binary_version,
	unnest(@working_directory :: text[ ]) as working_directory,
	unnest(@git_remote_url :: text[ ]) as git_remote_url,
	unnest(@exit_code :: int [ ]) as exit_code,
	unnest(@duration_ms :: float[ ]) as duration_ms;

-- name: GetIntelCohortsMatchedByMachineIDs :many
-- Obtains a list of cohorts that a user can track invocations for.
WITH machines AS (
    SELECT * FROM intel_machines WHERE id = ANY(@ids::uuid [])
) SELECT
	m.id machine_id,
	c.id,
    c.tracked_executables
  FROM intel_cohorts c
  CROSS JOIN machines m
	WHERE c.regex_operating_system ~ m.operating_system
	AND c.regex_operating_system_platform ~ m.operating_system_platform
	AND c.regex_operating_system_version ~ m.operating_system_version
	AND c.regex_architecture ~ m.architecture
	AND c.regex_instance_id ~ m.instance_id;

-- name: GetIntelMachinesMatchingFilters :many
WITH filtered_machines AS (
	SELECT
		*
	FROM intel_machines WHERE organization_id = @organization_id
	    AND operating_system ~ @regex_operating_system
		AND operating_system_platform ~ @regex_operating_system_platform
		AND operating_system_version ~ @regex_operating_system_version
		AND architecture ~ @regex_architecture
		AND instance_id ~ @regex_instance_id
), total_machines AS (
	SELECT COUNT(*) as count FROM filtered_machines
), paginated_machines AS (
	SELECT * FROM filtered_machines ORDER BY created_at DESC LIMIT NULLIF(@limit_opt :: int, 0) OFFSET NULLIF(@offset_opt :: int, 0)
)
SELECT tm.count, sqlc.embed(intel_machines) FROM paginated_machines AS intel_machines CROSS JOIN total_machines as tm;

-- name: UpsertIntelInvocationSummaries :exec
WITH machine_cohorts AS (
    SELECT
        m.id AS machine_id,
        c.id AS cohort_id
    FROM intel_machines m
    LEFT JOIN intel_cohorts c ON
        m.operating_system ~ c.regex_operating_system AND
        m.operating_system_platform ~ c.regex_operating_system_platform AND
        m.operating_system_version ~ c.regex_operating_system_version AND
        m.architecture ~ c.regex_architecture AND
        m.instance_id ~ c.regex_instance_id
),
invocations_with_cohorts AS (
    SELECT
		i.*,
		-- Truncate the created_at timestamp to the nearest 15 minute interval
        date_trunc('minute', i.created_at)
			- INTERVAL '1 minute' * (EXTRACT(MINUTE FROM i.created_at)::integer % 15) AS truncated_created_at,
        mc.cohort_id
    FROM intel_invocations i
    JOIN machine_cohorts mc ON i.machine_id = mc.machine_id
),
invocation_working_dirs AS (
	SELECT
		truncated_created_at,
		cohort_id,
		binary_name,
		binary_args,
		working_directory,
		COUNT(*) as count
	FROM invocations_with_cohorts
	GROUP BY truncated_created_at, cohort_id, binary_name, binary_args, working_directory
),
invocation_binary_paths AS (
	SELECT
		truncated_created_at,
		cohort_id,
		binary_name,
		binary_args,
		binary_path,
		COUNT(*) as count
	FROM invocations_with_cohorts
	GROUP BY truncated_created_at, cohort_id, binary_name, binary_args, binary_path
),
invocation_git_remote_urls AS (
	SELECT
		truncated_created_at,
		cohort_id,
		binary_name,
		binary_args,
		git_remote_url,
		COUNT(*) as count
	FROM invocations_with_cohorts
	GROUP BY truncated_created_at, cohort_id, binary_name, binary_args, git_remote_url
),
invocation_exit_codes AS (
	SELECT
		truncated_created_at,
		cohort_id,
		binary_name,
		binary_args,
		exit_code,
		COUNT(*) as count
	FROM invocations_with_cohorts
	GROUP BY truncated_created_at, cohort_id, binary_name, binary_args, exit_code
),
aggregated AS (
	SELECT
		invocations_with_cohorts.truncated_created_at,
		invocations_with_cohorts.cohort_id,
		invocations_with_cohorts.binary_name,
		invocations_with_cohorts.binary_args,
		jsonb_object_agg(invocation_working_dirs.working_directory, invocation_working_dirs.count) AS working_directories,
		jsonb_object_agg(invocation_git_remote_urls.git_remote_url, invocation_git_remote_urls.count) AS git_remote_urls,
		jsonb_object_agg(invocation_exit_codes.exit_code, invocation_exit_codes.count) AS exit_codes,
		jsonb_object_agg(invocation_binary_paths.binary_path, invocation_binary_paths.count) AS binary_paths,
		COUNT(DISTINCT machine_id) as unique_machines,
		COUNT(DISTINCT id) as total_invocations,
		PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY duration_ms) AS median_duration_ms
	FROM invocations_with_cohorts
	JOIN invocation_working_dirs USING (truncated_created_at, cohort_id, binary_name, binary_args)
	JOIN invocation_git_remote_urls USING (truncated_created_at, cohort_id, binary_name, binary_args)
	JOIN invocation_exit_codes USING (truncated_created_at, cohort_id, binary_name, binary_args)
	JOIN invocation_binary_paths USING (truncated_created_at, cohort_id, binary_name, binary_args)
	GROUP BY
		invocations_with_cohorts.truncated_created_at,
		invocations_with_cohorts.cohort_id,
		invocations_with_cohorts.binary_name,
		invocations_with_cohorts.binary_args
),
saved AS (
    INSERT INTO intel_invocation_summaries (id, cohort_id, starts_at, ends_at, binary_name, binary_args, binary_paths, working_directories, git_remote_urls, exit_codes, unique_machines, total_invocations, median_duration_ms)
    SELECT
        gen_random_uuid(),
        cohort_id,
        truncated_created_at,
		truncated_created_at + INTERVAL '15 minutes' AS ends_at,  -- Add 15 minutes to starts_at
        binary_name,
        binary_args,
        binary_paths,
        working_directories,
        git_remote_urls,
        exit_codes,
		unique_machines,
        total_invocations,
        median_duration_ms
    FROM aggregated
)
-- Delete all invocations after summarizing.
-- If there are invocations that are not in a cohort,
-- they must be purged!
DELETE FROM intel_invocations;

-- name: GetIntelReportGitRemotes :many
-- Get the total amount of time spent invoking commands
-- in the directories of a given git remote URL.
SELECT
  starts_at,
  ends_at,
  cohort_id,
  git_remote_url::text,
  PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY median_duration_ms) AS median_duration_ms,
  SUM(total_invocations) AS total_invocations
FROM
  intel_invocation_summaries,
  LATERAL jsonb_each_text(git_remote_urls) AS git_urls(git_remote_url, invocations)
WHERE
  starts_at >= @starts_at
AND
  (cohort_id = ANY(@cohort_ids :: uuid []))
GROUP BY
  starts_at,
  ends_at,
  cohort_id,
  git_remote_url;

-- name: GetIntelReportCommands :many
SELECT
  starts_at,
  ends_at,
  cohort_id,
  binary_name,
  binary_args,
  SUM(total_invocations) AS total_invocations,
  PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY median_duration_ms) AS median_duration_ms,
  -- We have to convert to text here because Go cannot scan
  -- an array of jsonb.
  array_agg(working_directories):: text [] AS aggregated_working_directories,
  array_agg(binary_paths):: text [] AS aggregated_binary_paths,
  array_agg(git_remote_urls):: text [] AS aggregated_git_remote_urls,
  array_agg(exit_codes):: text [] AS aggregated_exit_codes
FROM
  intel_invocation_summaries
WHERE
	starts_at >= @starts_at
AND
  (cohort_id = ANY(@cohort_ids :: uuid []))
GROUP BY
  starts_at, ends_at, cohort_id, binary_name, binary_args;
