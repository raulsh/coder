-- name: UpsertIntelCohort :one
INSERT INTO intel_cohorts (id, organization_id, created_by, created_at, updated_at, name, icon, description, tracked_executables, machine_metadata)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	ON CONFLICT (id) DO UPDATE SET
		updated_at = $5,
		name = $6,
		icon = $7,
		description = $8,
		tracked_executables = $9,
		machine_metadata = $10
	RETURNING *;

-- name: GetIntelCohortsByOrganizationID :many
SELECT * FROM intel_cohorts WHERE organization_id = $1 AND (@name IS NULL OR name = @name);

-- name: DeleteIntelCohortsByIDs :exec
DELETE FROM intel_cohorts WHERE id = ANY(@cohort_ids::uuid[]);

-- name: UpsertIntelMachine :one
INSERT INTO intel_machines (id, created_at, updated_at, instance_id, organization_id, user_id, ip_address, daemon_version, metadata)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	ON CONFLICT (user_id, instance_id) DO UPDATE SET
		updated_at = $3,
		ip_address = $7,
		daemon_version = $8,
		metadata = $9
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
  CROSS JOIN machines m WHERE
    c.machine_metadata = '{}' OR EXISTS (
        SELECT 1
        FROM jsonb_each_text(metadata) AS mdata(key, value)
        JOIN jsonb_each_text(c.machine_metadata) AS cdata(key, regex)
        ON mdata.key = cdata.key AND mdata.value ~ regex
    );

-- name: GetIntelMachinesMatchingFilters :many
WITH filtered_machines AS (
	SELECT
		*
	FROM intel_machines WHERE organization_id = @organization_id
	    AND (@metadata :: jsonb = '{}' OR EXISTS (
        SELECT 1
        FROM jsonb_each_text(metadata) AS mdata(key, value)
        JOIN jsonb_each_text(@metadata :: jsonb) AS cdata(key, regex)
        ON mdata.key = cdata.key AND mdata.value ~ regex
    ))
), total_machines AS (
	SELECT COUNT(*) as count FROM filtered_machines
), paginated_machines AS (
	SELECT * FROM filtered_machines ORDER BY created_at DESC LIMIT NULLIF(@limit_opt :: int, 0) OFFSET NULLIF(@offset_opt :: int, 0)
)
SELECT tm.count, sqlc.embed(intel_machines) FROM paginated_machines AS intel_machines CROSS JOIN total_machines as tm;

-- name: UpsertIntelInvocationSummaries :exec
WITH machines_with_metadata AS (
    SELECT
        m.id AS machine_id,
        m.metadata AS machine_metadata
    FROM intel_machines m
),
invocations_with_metadata AS (
    SELECT
        i.*,
        -- Truncate the created_at timestamp to the nearest 15 minute interval
        date_trunc('minute', i.created_at)
            - INTERVAL '1 minute' * (EXTRACT(MINUTE FROM i.created_at)::integer % 15) AS truncated_created_at,
        m.machine_metadata
    FROM intel_invocations i
    JOIN machines_with_metadata m ON i.machine_id = m.machine_id
),
invocation_working_dirs AS (
	SELECT
		truncated_created_at,
		binary_name,
		binary_args,
		working_directory,
		COUNT(*) as count
	FROM invocations_with_metadata
	GROUP BY truncated_created_at, binary_name, binary_args, working_directory
),
invocation_binary_paths AS (
	SELECT
		truncated_created_at,
		binary_name,
		binary_args,
		binary_path,
		COUNT(*) as count
	FROM invocations_with_metadata
	GROUP BY truncated_created_at, binary_name, binary_args, binary_path
),
invocation_git_remote_urls AS (
	SELECT
		truncated_created_at,
		binary_name,
		binary_args,
		git_remote_url,
		COUNT(*) as count
	FROM invocations_with_metadata
	GROUP BY truncated_created_at, binary_name, binary_args, git_remote_url
),
invocation_exit_codes AS (
	SELECT
		truncated_created_at,
		binary_name,
		binary_args,
		exit_code,
		COUNT(*) as count
	FROM invocations_with_metadata
	GROUP BY truncated_created_at, binary_name, binary_args, exit_code
),
metadata_counts AS (
    SELECT
        truncated_created_at,
        binary_name,
        binary_args,
        meta.key,
        meta.value,
        COUNT(*) as count
    FROM invocations_with_metadata,
         jsonb_each_text(machine_metadata) AS meta(key, value)
    GROUP BY truncated_created_at, binary_name, binary_args, meta.key, meta.value
),
metadata_aggregated AS (
    SELECT
        truncated_created_at,
        binary_name,
        binary_args,
        key,
        jsonb_object_agg(value, count) AS counts
    FROM metadata_counts
    GROUP BY truncated_created_at, binary_name, binary_args, key
),
metadata_json AS (
    SELECT
        truncated_created_at,
        binary_name,
        binary_args,
        jsonb_object_agg(key, counts) AS metadata
    FROM metadata_aggregated
    GROUP BY truncated_created_at, binary_name, binary_args
),
aggregated AS (
	SELECT
		invocations_with_metadata.truncated_created_at,
		invocations_with_metadata.binary_name,
		invocations_with_metadata.binary_args,
		jsonb_object_agg(invocation_working_dirs.working_directory, invocation_working_dirs.count) AS working_directories,
		jsonb_object_agg(invocation_git_remote_urls.git_remote_url, invocation_git_remote_urls.count) AS git_remote_urls,
		jsonb_object_agg(invocation_exit_codes.exit_code, invocation_exit_codes.count) AS exit_codes,
		jsonb_object_agg(invocation_binary_paths.binary_path, invocation_binary_paths.count) AS binary_paths,
		COALESCE(mj.metadata, '{}'::jsonb) AS machine_metadata,
		COUNT(DISTINCT machine_id) as unique_machines,
		COUNT(DISTINCT id) as total_invocations,
		PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY duration_ms) AS median_duration_ms
	FROM invocations_with_metadata
	JOIN invocation_working_dirs USING (truncated_created_at, binary_name, binary_args)
	JOIN invocation_git_remote_urls USING (truncated_created_at, binary_name, binary_args)
	JOIN invocation_exit_codes USING (truncated_created_at, binary_name, binary_args)
	JOIN invocation_binary_paths USING (truncated_created_at, binary_name, binary_args)
	LEFT JOIN metadata_json mj USING (truncated_created_at, binary_name, binary_args)
	GROUP BY
		invocations_with_metadata.truncated_created_at,
		invocations_with_metadata.binary_name,
		invocations_with_metadata.binary_args,
		mj.metadata
),
saved AS (
    INSERT INTO intel_invocation_summaries (id, starts_at, ends_at, binary_name, binary_args, binary_paths, working_directories, git_remote_urls, exit_codes, machine_metadata, unique_machines, total_invocations, median_duration_ms)
    SELECT
        gen_random_uuid(),
        truncated_created_at,
		truncated_created_at + INTERVAL '15 minutes' AS ends_at,  -- Add 15 minutes to starts_at
        binary_name,
        binary_args,
        binary_paths,
        working_directories,
        git_remote_urls,
        exit_codes,
		machine_metadata,
		unique_machines,
        total_invocations,
        median_duration_ms
    FROM aggregated
)
-- Delete all invocations after summarizing.
-- If there are invocations that are not in a cohort,
-- they must be purged!
DELETE FROM intel_invocations;

-- name: GetIntelInvocationSummaries :many
SELECT * FROM intel_invocation_summaries WHERE starts_at >= @starts_at AND
	(@metadata :: jsonb = '{}' OR EXISTS (
        SELECT 1
        FROM jsonb_each_text(machine_metadata) AS mdata(key, value)
        JOIN jsonb_each_text(@machine_metadata :: jsonb) AS cdata(key, regex)
        ON mdata.key = cdata.key AND mdata.value ~ regex
    ));
