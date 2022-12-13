-- +goose Up

ALTER TABLE config_component_relationships ADD FOREIGN KEY (config_id) REFERENCES config_items(id);
