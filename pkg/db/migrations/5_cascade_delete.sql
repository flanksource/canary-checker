-- +goose Up

ALTER TABLE checks DROP CONSTRAINT checks_canary_id_fkey;
ALTER TABLE checks ADD	FOREIGN KEY (canary_id) REFERENCES canaries(id) ON DELETE CASCADE;
ALTER TABLE check_statuses DROP CONSTRAINT check_statuses_check_id_fkey;
ALTER TABLE check_statuses ADD FOREIGN KEY (check_id) REFERENCES checks(id) ON DELETE CASCADE;
