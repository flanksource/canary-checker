-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS checks(
	canary json,
	canary_name TEXT,
	check_type TEXT NOT NULL,
	description TEXT,
	display_type TEXT,
	endpoint TEXT,
	icon TEXT,
	id TEXT NOT NULL,
	interval int,
	key TEXT NOT NULL,
	labels json,
	name TEXT NOT NULL,
	namespace TEXT NOT NULL,
	owner TEXT,
	runner_labels json,
	runner_name TEXT,
	schedule TEXT,
	severity TEXT,
	updated_at TIMESTAMP with time zone NOT NULL,
	PRIMARY KEY (key)
);
---
CREATE TABLE IF NOT EXISTS check_statuses(
	check_key TEXT NOT NULL,
	details json,
	duration INT,
	error Text,
	inserted_at TIMESTAMP with time zone NOT NULL,
	invalid boolean,
	message TEXT,
	status boolean,
	time TIMESTAMP with time zone,
	PRIMARY KEY (time, check_key)
);
CREATE INDEX if NOT EXISTS idx_check_statuses_time on check_statuses (time);
CREATE INDEX IF NOT EXISTS idx_check_statuses_key on check_statuses(check_key);
-- +goose StatementEnd
