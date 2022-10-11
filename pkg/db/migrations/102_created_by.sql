-- +goose Up

ALTER TABLE canaries ADD COLUMN created_by UUID NULL;
ALTER TABLE templates ADD COLUMN created_by UUID NULL;
ALTER TABLE components ADD COLUMN created_by UUID NULL;
