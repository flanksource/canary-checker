-- +goose Up
-- +goose StatementBegin

ALTER TABLE component_relationships ADD UNIQUE (component_id,relationship_id,selector_id);


-- +goose StatementEnd