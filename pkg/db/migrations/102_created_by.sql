-- +goose Up

ALTER TABLE canaries ADD COLUMN created_by UUID NULL;
