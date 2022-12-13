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
    FOREIGN KEY(config_id) REFERENCES configs(id),
    UNIQUE (component_id, config_id)
);


DO $$
BEGIN
IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'config_names')
    THEN
    --- This is a dummy view, that will be replaced by config-db if installed
    CREATE VIEW config_names AS
      SELECT ;

END IF;
END $$;

-- +goose StatementEnd


-- +goose Down

DROP TABLE IF EXISTS config_component_relationships;
