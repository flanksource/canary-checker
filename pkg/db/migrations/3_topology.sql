-- +goose Up
-- +goose StatementBegin


CREATE TABLE IF NOT EXISTS systems (
  id UUID DEFAULT generate_ulid() PRIMARY KEY,
  external_id text NOT NULL,
  name text NOT NULL, -- Corresponding to .metadata.name
  text text NULL,
  status text NOT NULL,
  hidden boolean NOT NULL DEFAULT false,
  silenced boolean NOT NULL DEFAULT false,
  labels jsonb null,
  tooltip text,
  lifecycle text,
  icon text,
  owner text,
  type text,
  properties jsonb,
  spec jsonb,
  created_at timestamp NOT NULL DEFAULT now(),
  updated_at timestamp NOT NULL DEFAULT now(),
  unique (type, external_id)
);


CREATE TABLE IF NOT EXISTS components (
  id UUID DEFAULT generate_ulid() PRIMARY KEY,
  external_id text NOT NULL,
  parent_id UUID NULL,
  system_id UUID NULL,
  name text NOT NULL, -- Corresponding to .metadata.name
    text text NULL,
  labels jsonb null,
  hidden boolean NOT NULL DEFAULT false,
  silenced boolean NOT NULL DEFAULT false,
  status text NOT NULL,
  description text,
  lifecycle text,
  tooltip text,
  icon text,
  type text NULL,
  owner text,
  spec jsonb,
  properties jsonb,
  created_at timestamp NOT NULL DEFAULT now(),
  updated_at timestamp NOT NULL DEFAULT now(),
  FOREIGN KEY (parent_id) REFERENCES component(id),
  FOREIGN KEY (system_id) REFERENCES system(id),
  UNIQUE (system_id,type, external_id)
);


CREATE TABLE IF NOT EXISTS SYSTEM_TEMPLATES (
  id UUID DEFAULT generate_ulid() PRIMARY KEY,
  name text NOT NULL,
  namespace text NOT NULL,
  labels jsonb null,
  spec jsonb,
  created_at timestamp,
  updated_at timestamp,
  schedule text,
  deleted_at TIMESTAMP NULL,
  UNIQUE (name, namespace)
);

-- +goose StatementEnd


