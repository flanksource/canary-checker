-- +goose Up

ALTER TABLE components ADD COLUMN is_leaf BOOL DEFAULT false;