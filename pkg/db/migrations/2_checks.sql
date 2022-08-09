-- +goose Up

CREATE TABLE canaries (
	id UUID DEFAULT generate_ulid() PRIMARY KEY,
	name text NOT NULL,
	namespace text NOT NULL,
	labels jsonb NULL,
	spec jsonb NOT NULL,
	schedule text,
	source text,
	created_at TIMESTAMP,
	updated_at TIMESTAMP,
	deleted_at TIMESTAMP DEFAULT NULL,
	UNIQUE (name, namespace, source)
);

CREATE TABLE checks(
	id UUID DEFAULT generate_ulid() PRIMARY KEY,
	canary_id UUID NOT NULL,
	type TEXT NOT NULL,
	name text NOT NULL,
	description TEXT,
	icon TEXT,
	spec jsonb  NULL,
	labels jsonb NULL,
	owner text,
	severity TEXT,
	last_runtime TIMESTAMP,
	next_runtime TIMESTAMP,
	silenced_at TIMESTAMP NULL,
	status TEXT, -- status of last check executed
	created_at TIMESTAMP,
	updated_at TIMESTAMP NULL,
	deleted_at TIMESTAMP DEFAULT NULL,
	FOREIGN KEY (canary_id) REFERENCES canaries(id) ON DELETE CASCADE,
	UNIQUE (canary_id, type, name)
);
---
CREATE TABLE check_statuses(
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
	FOREIGN KEY (check_id) REFERENCES checks(id) ON DELETE CASCADE,
	PRIMARY KEY (check_id, time)

);
-- +goose Down
DROP TABLE check_statuses;
DROP TABLE checks;
DROP TABLE canaries;