-- name: UpsertIntelCohort :one
INSERT INTO intel_cohorts (id, organization_id, created_by, created_at, updated_at, name, display_name, icon, description, filter_regex_operating_system, filter_regex_operating_system_version, filter_regex_architecture, filter_regex_instance_id, tracked_executables)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	ON CONFLICT (id) DO UPDATE SET
		updated_at = $5,
		name = $6,
		display_name = $7,
		icon = $8,
		description = $9,
		filter_regex_operating_system = $10,
		filter_regex_operating_system_version = $11,
		filter_regex_architecture = $12,
		filter_regex_instance_id = $13,
		tracked_executables = $14
	RETURNING *;

-- name: GetIntelCohortsByOrganizationID :many
SELECT * FROM intel_cohorts WHERE organization_id = $1;

-- name: DeleteIntelCohortsByIDs :exec
DELETE FROM intel_cohorts WHERE id = ANY($1::uuid[]);

-- name: UpsertIntelMachine :one
INSERT INTO intel_machines (id, created_at, updated_at, instance_id, organization_id, user_id, ip_address, hostname, operating_system, operating_system_version, cpu_cores, memory_mb_total, architecture, daemon_version)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	ON CONFLICT (user_id, instance_id) DO UPDATE SET
		updated_at = $3,
		ip_address = $7,
		hostname = $8,
		operating_system = $9,
		operating_system_version = $10,
		cpu_cores = $11,
		memory_mb_total = $12,
		architecture = $13,
		daemon_version = $14
	RETURNING *;

-- name: InsertIntelInvocations :exec
-- Insert many invocations using unnest
INSERT INTO intel_invocations (
	created_at, machine_id, user_id, id, binary_hash, binary_path, binary_args,
	binary_version, working_directory, git_remote_url, exit_code, duration_ms)
SELECT
	@created_at :: timestamptz as created_at,
	@machine_id :: uuid as machine_id,
	@user_id :: uuid as user_id,
	unnest(@id :: uuid[ ]) as id,
	unnest(@binary_hash :: text[ ]) as binary_hash,
	unnest(@binary_path :: text[ ]) as binary_path,
	-- This has to be jsonb because PostgreSQL does not support parsing
	-- multi-dimensional multi-length arrays!
	jsonb_array_elements(@binary_args :: jsonb) as binary_args,
	unnest(@binary_version :: text[ ]) as binary_version,
	unnest(@working_directory :: text[ ]) as working_directory,
	unnest(@git_remote_url :: text[ ]) as git_remote_url,
	unnest(@exit_code :: int [ ]) as exit_code,
	unnest(@duration_ms :: int[ ]) as duration_ms;

-- name: GetIntelCohortsMatchedByMachineIDs :many
-- Obtains a list of cohorts that a user can track invocations for.
WITH machines AS (
    SELECT * FROM intel_machines WHERE id = ANY(@ids::uuid [])
),
matches AS (
    SELECT
		m.id machine_id,
		c.id,
        c.tracked_executables,
        (c.filter_regex_operating_system ~ m.operating_system)::boolean AS operating_system_match,
        (c.filter_regex_operating_system_version ~ m.operating_system_version)::boolean AS operating_system_version_match,
        (c.filter_regex_architecture ~ m.architecture)::boolean AS architecture_match,
        (c.filter_regex_instance_id ~ m.instance_id)::boolean AS instance_id_match
    FROM intel_cohorts c
    CROSS JOIN machines m
)
SELECT
    *
FROM matches
WHERE
    operating_system_match AND
    operating_system_version_match AND
    architecture_match AND
	instance_id_match;

-- name: GetIntelMachinesMatchingFilters :many
WITH filtered_machines AS (
	SELECT
		*
	FROM intel_machines WHERE organization_id = @organization_id
	    AND operating_system ~ @filter_operating_system
		AND (operating_system_version IS NULL OR operating_system_version ~ @filter_operating_system_version::text)
		AND architecture ~ @filter_architecture
		AND instance_id ~ @filter_instance_id
), total_machines AS (
	SELECT COUNT(*) as count FROM filtered_machines
), paginated_machines AS (
	SELECT * FROM filtered_machines ORDER BY created_at DESC LIMIT NULLIF(@limit_opt :: int, 0) OFFSET NULLIF(@offset_opt :: int, 0)
)
SELECT tm.count, sqlc.embed(intel_machines) FROM paginated_machines AS intel_machines CROSS JOIN total_machines as tm;

-- name: GetConsistencyByIntelCohort :many
SELECT
    binary_path,
    binary_args,
    PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY duration_ms) AS median_duration
FROM
    intel_invocations
GROUP BY
    binary_path, binary_args
ORDER BY
    median_duration DESC;
