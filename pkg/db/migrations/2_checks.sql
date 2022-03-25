-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS canaries (
	id UUID DEFAULT generate_ulid() PRIMARY KEY,
	name text NOT NULL,
	namespace text NOT NULL,
	labels jsonb NULL,
	spec jsonb NOT NULL,
	schedule text,
	source text,
	created_at TIMESTAMP,
	updated_at TIMESTAMP,
	deleted_at TIMESTAMP,
	UNIQUE (name, namespace)
);

CREATE TABLE IF NOT EXISTS checks(
	id UUID DEFAULT generate_ulid() PRIMARY KEY,
	canary_id UUID NOT NULL,
	type TEXT NOT NULL,
	name text NOT NULL,
	description TEXT,
	icon TEXT,
	spec jsonb  NULL,
	owner text,
	severity TEXT,
	last_runtime TIMESTAMP,
	next_runtime TIMESTAMP,
	created_at TIMESTAMP,
	updated_at TIMESTAMP NULL,
	FOREIGN KEY (canary_id) REFERENCES canaries(id),
	UNIQUE (canary_id, type, name)
);
---
CREATE TABLE IF NOT EXISTS check_statuses(
	check_id UUID NOT NULL,
	details jsonb,
	duration INT,
	error Text,
	-- The time the check as run, can be earlier than created_at
	time TIMESTAMP,
	-- The time in which the check was added to the database
	created_at TIMESTAMP with time zone NOT NULL,
	invalid boolean,
	message TEXT,
	status boolean,
	FOREIGN KEY (check_id) REFERENCES checks(id),
	PRIMARY KEY (check_id, time)

);
-- +goose StatementEnd
