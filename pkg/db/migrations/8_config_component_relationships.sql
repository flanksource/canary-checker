-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS config_component_relationships(
    component_id UUID NOT NULL,
    config_id UUID NOT NULL,
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now(),
    deleted_at TIMESTAMP DEFAULT NULL,
    selector_id text, -- hash of the selector from the component config
    FOREIGN KEY(component_id) REFERENCES components(id),
    UNIQUE (component_id, config_id)
);



--- This is a dummy view, that will be replaced by config-db if installed
CREATE VIEW config_names AS
  SELECT ;


-- +goose StatementEnd


-- +goose Down

DROP TABLE IF EXISTS config_component_relationships;
