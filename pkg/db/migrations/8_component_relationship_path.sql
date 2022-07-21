-- +goose Up
-- +goose StatementBegin

ALTER TABLE component_relationships ADD relationship_path text;

-- +goose StatementEnd