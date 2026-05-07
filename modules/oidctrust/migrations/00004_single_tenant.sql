-- +goose Up
-- +goose StatementBegin
ALTER TABLE trust
    ADD COLUMN client_id TEXT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE trust
    DROP COLUMN client_id;
-- +goose StatementEnd
