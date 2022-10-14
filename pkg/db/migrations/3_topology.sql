-- +goose Up
-- +goose StatementBegin

CREATE TABLE templates (
  id UUID DEFAULT generate_ulid() PRIMARY KEY,
  name text NOT NULL,
  namespace text NOT NULL,
  labels jsonb null,
  spec jsonb,
  created_at timestamp,
  updated_at timestamp,
  schedule text,
  created_by UUID NULL,
  deleted_at TIMESTAMP DEFAULT NULL,
  UNIQUE (name, namespace)
);

CREATE TABLE components (
  id UUID DEFAULT generate_ulid() PRIMARY KEY,
  system_template_id UUID,
  external_id text NOT NULL,
  parent_id UUID DEFAULT NULL,
  name text NOT NULL, -- Corresponding to .metadata.name
  text text NULL,
  topology_type text,
  namespace text,
  labels jsonb null,
  hidden boolean NOT NULL DEFAULT false,
  silenced boolean NOT NULL DEFAULT false,
  status text NOT NULL,
  description text,
  lifecycle text,
  tooltip text,
  status_reason text,
  schedule text,
  icon text,
  type text NULL,
  owner text,
  selectors jsonb,
  component_checks jsonb,
  configs jsonb,
  properties jsonb,
  path text,
  summary  jsonb,
  is_leaf BOOL DEFAULT false,
  cost_per_minute numeric(16,4) NULL,
  cost_total_1d numeric(16,4) NULL,
  cost_total_7d numeric(16,4) NULL,
  cost_total_30d numeric(16,4) NULL,
  created_by UUID NULL,
  created_at timestamp NOT NULL DEFAULT now(),
  updated_at timestamp NOT NULL DEFAULT now(),
  deleted_at TIMESTAMP DEFAULT NULL,
  FOREIGN KEY (parent_id) REFERENCES components(id),
  FOREIGN KEY (system_template_id) REFERENCES templates(id),
  UNIQUE (system_template_id, type, name, parent_id)
);


CREATE TABLE component_relationships(
  component_id UUID NOT NULL,
  relationship_id UUID NOT NULL, -- parent component id
  created_at timestamp NOT NULL DEFAULT now(),
  updated_at timestamp NOT NULL DEFAULT now(),
  deleted_at TIMESTAMP DEFAULT NULL,
  selector_id text, -- hash of the selector from the components
  relationship_path text,
  FOREIGN KEY(component_id) REFERENCES components(id),
  FOREIGN KEY(relationship_id) REFERENCES components(id),
  UNIQUE(component_id,relationship_id,selector_id)
);

CREATE TABLE check_component_relationships(
  component_id UUID NOT NULL,
  check_id UUID NOT NULL,
  canary_id UUID NOT NULL,
  created_at timestamp NOT NULL DEFAULT now(),
  updated_at timestamp NOT NULL DEFAULT now(),
  deleted_at TIMESTAMP DEFAULT NULL,
  selector_id text, -- hash of the selector from the components
  FOREIGN KEY(canary_id) REFERENCES canaries(id),
  FOREIGN KEY(component_id) REFERENCES components(id),
  FOREIGN KEY(check_id) REFERENCES checks(id),
  UNIQUE (component_id, check_id, canary_id, selector_id)
);

-- +goose StatementEnd

-- For local developemnent; one can run: `goose -dir ./pkg/db/migrations  postgres "postgres://tarun@localhost:5432/canary?sslmode=disable" down-to 0` to remove all the migr
-- +goose Down
DROP TABLE IF EXISTS check_component_relationships;
DROP TABLE component_relationships;
DROP FUNCTION lookup_component_by_property;
DROP TABLE components;
DROP TABLE templates;
