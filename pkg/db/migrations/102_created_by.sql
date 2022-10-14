-- +goose Up

ALTER TABLE canaries ADD COLUMN created_by UUID NULL;
ALTER TABLE templates ADD COLUMN created_by UUID NULL;
ALTER TABLE components ADD COLUMN created_by UUID NULL;

-- +goose Down
DROP TABLE config_component_relationships;
DROP TABLE check_component_relationships;
DROP TABLE component_relationships;
DROP FUNCTION lookup_component_by_property;
DROP TABLE components CASCADE;
DROP TABLE templates;
DROP TABLE check_statuses;
DROP TABLE checks CASCADE;
DROP TABLE canaries;
