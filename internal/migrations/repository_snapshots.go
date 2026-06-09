package migrations

import "github.com/shareinto/paas/internal/platform/database"

var repositorySnapshotMigrations = []database.Migration{
	{
		Version: 202605301401,
		Name:    "repository_snapshots",
		Up: `
CREATE TABLE repository_snapshots (
  module VARCHAR(128) NOT NULL PRIMARY KEY,
  payload JSON NOT NULL,
  updated_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`,
		Down: `DROP TABLE IF EXISTS repository_snapshots;`,
	},
	{
		Version: 202606080201,
		Name:    "cleanup_buildspec_static_snapshot_fields",
		Up: `
UPDATE repository_snapshots
SET payload = JSON_SET(payload,
    '$.BuildEnvironments',
    COALESCE((
      SELECT JSON_ARRAYAGG(
        CASE
          WHEN JSON_UNQUOTE(JSON_EXTRACT(item, '$.ID')) = 'build_env_node_static'
            THEN JSON_SET(JSON_REMOVE(item, '$.DefaultBuildCommand', '$.DefaultArtifactPath', '$.default_build_command', '$.default_artifact_path'), '$.Status', 'deleted', '$.IsDefault', false)
          WHEN JSON_UNQUOTE(JSON_EXTRACT(item, '$.id')) = 'build_env_node_static'
            THEN JSON_SET(JSON_REMOVE(item, '$.DefaultBuildCommand', '$.DefaultArtifactPath', '$.default_build_command', '$.default_artifact_path'), '$.status', 'deleted', '$.is_default', false)
          ELSE JSON_REMOVE(item, '$.DefaultBuildCommand', '$.DefaultArtifactPath', '$.default_build_command', '$.default_artifact_path')
        END
      )
      FROM JSON_TABLE(JSON_EXTRACT(payload, '$.BuildEnvironments'), '$[*]' COLUMNS (item JSON PATH '$')) build_envs
    ), JSON_ARRAY()))
WHERE module = 'build' AND JSON_CONTAINS_PATH(payload, 'one', '$.BuildEnvironments');

UPDATE repository_snapshots
SET payload = JSON_SET(payload,
    '$.RuntimeEnvironments',
    COALESCE((
      SELECT JSON_ARRAYAGG(
        CASE
          WHEN JSON_UNQUOTE(JSON_EXTRACT(item, '$.ID')) = 'runtime_env_nginx'
            THEN JSON_SET(JSON_REMOVE(item, '$.StartCommand', '$.start_command'), '$.Status', 'deleted', '$.IsDefault', false)
          WHEN JSON_UNQUOTE(JSON_EXTRACT(item, '$.id')) = 'runtime_env_nginx'
            THEN JSON_SET(JSON_REMOVE(item, '$.StartCommand', '$.start_command'), '$.status', 'deleted', '$.is_default', false)
          ELSE JSON_REMOVE(item, '$.StartCommand', '$.start_command')
        END
      )
      FROM JSON_TABLE(JSON_EXTRACT(payload, '$.RuntimeEnvironments'), '$[*]' COLUMNS (item JSON PATH '$')) runtime_envs
    ), JSON_ARRAY()))
WHERE module = 'build' AND JSON_CONTAINS_PATH(payload, 'one', '$.RuntimeEnvironments');

UPDATE repository_snapshots
SET payload = JSON_SET(payload,
    '$.Sources',
    COALESCE((
      SELECT JSON_ARRAYAGG(
        JSON_SET(item,
          '$.BuildSpec', JSON_REMOVE(COALESCE(JSON_EXTRACT(item, '$.BuildSpec'), JSON_OBJECT()), '$.StartCommand', '$.TargetPath', '$.ArtifactPath'),
          '$.build_spec', JSON_REMOVE(COALESCE(JSON_EXTRACT(item, '$.build_spec'), JSON_OBJECT()), '$.start_command', '$.target_path', '$.artifact_path'))
      )
      FROM JSON_TABLE(JSON_EXTRACT(payload, '$.Sources'), '$[*]' COLUMNS (item JSON PATH '$')) app_sources
    ), JSON_ARRAY())),
    updated_at = UTC_TIMESTAMP(6)
WHERE module = 'application-environment' AND JSON_CONTAINS_PATH(payload, 'one', '$.Sources');

UPDATE repository_snapshots
SET payload = JSON_SET(payload,
    '$.Applications',
    COALESCE((
      SELECT JSON_ARRAYAGG(
        CASE
          WHEN JSON_CONTAINS_PATH(app_item, 'one', '$.RuntimeEnvironments') THEN JSON_SET(app_item,
            '$.RuntimeEnvironments',
            COALESCE((
              SELECT JSON_ARRAYAGG(JSON_REMOVE(runtime_item, '$.StartCommand', '$.start_command'))
              FROM JSON_TABLE(JSON_EXTRACT(app_item, '$.RuntimeEnvironments'), '$[*]' COLUMNS (runtime_item JSON PATH '$')) runtime_envs
            ), JSON_ARRAY()))
          ELSE app_item
        END
      )
      FROM JSON_TABLE(JSON_EXTRACT(payload, '$.Applications'), '$[*]' COLUMNS (app_item JSON PATH '$')) apps
    ), JSON_ARRAY()))
WHERE module = 'application-environment' AND JSON_CONTAINS_PATH(payload, 'one', '$.Applications');
`,
		Down: `SELECT 1;`,
	},
}
