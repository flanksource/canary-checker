-- +goose Up
-- +goose StatementBegin

CREATE TABLE check_component_relationships(
    component_id UUID NOT NULL,
    check_id UUID NOT NULL,
    canary_id UUID NOT NULL,
    created_at timestamp NOT NULL DEFAULT now(),
    updated_at timestamp NOT NULL DEFAULT now(),
    deleted_at TIMESTAMP DEFAULT NULL,
    selector_id text, -- hash of the selector from the components
    FOREIGN KEY (canary_id) REFERENCES canaries(id),
    FOREIGN KEY(component_id) REFERENCES components(id), 
    FOREIGN KEY(check_id) REFERENCES checks(id),
    UNIQUE (component_id, check_id, canary_id, selector_id)
)
-- +goose StatementEnd


-- +goose Down

DROP TABLE IF EXISTS check_component_relationships;