DECLARE first_user_id AS uuid = gen_random_uuid();
DECLARE first_machine_id AS uuid = gen_random_uuid();

-- Use the UUID variable in the INSERT statement
INSERT INTO intel_machines (
  id,
  machine_id,
  user_id,
  ip_address,
  hostname,
  operating_system,
  operating_system_version,
  cpu_cores,
  memory_mb_total,
  architecture,
  daemon_version,
  git_config_email,
  git_config_name,
  tags
) VALUES (
  gen_random_uuid(),
  first_machine_id,
  first_user_id,
  '1.1.1.1',
  'test-computer'
  )

INSERT INTO
	intel_machines (
		id,
		machine_id,
		user_id,
		ip_address,
		hostname,
		operating_system,
		operating_system_version,
		cpu_cores,
		memory_mb_total,
		architecture,
		daemon_version,
		git_config_email,
		git_config_name,
		tags
	) VALUES (
		gen_random_uuid(),
		'30095c71-380b-457a-8995-97b8ee6e5307',
		gen_random_uuid(),
		'
	)
