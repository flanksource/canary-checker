-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS config_component_relationships(
    component_id UUID NOT NULL,
    config_id UUID NOT NULL,
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now(),
    deleted_at TIMESTAMP DEFAULT NULL,
    FOREIGN KEY(component_id) REFERENCES components(id), 
    UNIQUE (component_id, config_id)
);

ALTER TABLE components ADD COLUMN IF NOT EXISTS configs jsonb;
-- +goose StatementEnd


-- +goose Down

DROP TABLE IF EXISTS config_component_relationships;
