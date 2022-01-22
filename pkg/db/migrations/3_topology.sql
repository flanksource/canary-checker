-- +goose Up
-- +goose StatementBegin


CREATE TABLE system (
  id UUID DEFAULT generate_ulid() PRIMARY KEY,
  external_id text NOT NULL,
  name text NOT NULL,
  status text NOT NULL,
  tooltip text,
  lifecycle text,
  icon text,
  owner text,
  type text,
  properties jsonb,
  created_at timestamp NOT NULL DEFAULT now(),
  updated_at timestamp NOT NULL DEFAULT now(),
  unique (type, external_id)
);


CREATE TABLE component (
  id UUID DEFAULT generate_ulid() PRIMARY KEY,
  external_id text NOT NULL,
  parent_id UUID NULL,
  system_id UUID NULL,
  name text NOT NULL,
  status text NOT NULL,

  description text,
  lifecycle text,
  tooltip text,
  icon text,
  type text NULL,
  owner text,
  properties jsonb,
  created_at timestamp NOT NULL DEFAULT now(),
  updated_at timestamp NOT NULL DEFAULT now(),
  FOREIGN KEY (parent_id) REFERENCES component(id),
  FOREIGN KEY (system_id) REFERENCES system(id),
  UNIQUE (system_id,type, external_id)
);


-- +goose StatementEnd


