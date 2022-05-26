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
  deleted_at TIMESTAMP DEFAULT NULL,
  UNIQUE (name, namespace)
);

CREATE TABLE systems (
  id UUID DEFAULT generate_ulid() PRIMARY KEY,
  system_template_id UUID,
  external_id text NULL,
  name text NOT NULL, -- Corresponding to .metadata.name
  namespace text,
  text text NULL,
  status text NOT NULL,
  hidden boolean NOT NULL DEFAULT false,
  silenced boolean NOT NULL DEFAULT false,
  label text,
  labels jsonb null,
  tooltip text,
  lifecycle text,
  icon text,
  owner text,
  type text,
  topology_type text,
  properties jsonb,
  summary  jsonb,
  created_at timestamp NOT NULL DEFAULT now(),
  updated_at timestamp NOT NULL DEFAULT now(),
  deleted_at TIMESTAMP DEFAULT NULL,
  FOREIGN KEY (system_template_id) REFERENCES templates(id),
  unique (type, external_id)
);


CREATE TABLE components (
  id UUID DEFAULT generate_ulid() PRIMARY KEY,
  external_id text NOT NULL,
  parent_id UUID DEFAULT NULL,
  system_id UUID NULL,
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
  icon text,
  type text NULL,
  owner text,
  selectors jsonb,
  properties jsonb,
  summary  jsonb,
  created_at timestamp NOT NULL DEFAULT now(),
  updated_at timestamp NOT NULL DEFAULT now(),
  deleted_at TIMESTAMP DEFAULT NULL,
  FOREIGN KEY (parent_id) REFERENCES components(id),
  FOREIGN KEY (system_id) REFERENCES systems(id),
  UNIQUE (system_id, type, name, parent_id)
);


CREATE TABLE component_relationships(
  component_id UUID NOT NULL,
  relationship_id UUID NOT NULL, -- parent component id
  created_at timestamp NOT NULL DEFAULT now(),
  updated_at timestamp NOT NULL DEFAULT now(),
  deleted_at TIMESTAMP DEFAULT NULL,
  selector_id text, -- hash of the selector from the components
  FOREIGN KEY(component_id) REFERENCES components(id), 
  FOREIGN KEY(relationship_id) REFERENCES components(id)
);


create OR REPLACE function lookup_component_by_property(text, text)
returns setof components
as
$$
begin
  return query
    select * from components where properties != 'null' and name in (select name  from components,jsonb_array_elements(properties) property where properties != 'null' and  property is not null and  property->>'name' = $1 and property->>'text' = $2);
end;
$$
language plpgsql;


-- +goose StatementEnd



-- For local developemnent; one can run: `goose -dir ./pkg/db/migrations  postgres "postgres://tarun@localhost:5432/canary?sslmode=disable" down-to 0` to remove all the migr
-- +goose Down
DROP TABLE component_relationships;
DROP FUNCTION GetComponentsWithProperties;
DROP TABLE components;
DROP TABLE systems;
DROP TABLE templates;