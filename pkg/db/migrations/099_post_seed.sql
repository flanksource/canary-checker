-- +goose Up


INSERT INTO canary_checker_db_version(version_id, tstamp, is_applied) (
  SELECT  version_id, now() as tstamp, true as is_applied
  FROM    generate_series(100, 101) version_id
);

