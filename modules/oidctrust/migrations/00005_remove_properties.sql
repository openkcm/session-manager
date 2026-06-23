-- +goose Up
-- +goose StatementBegin
ALTER TABLE trust
    DROP COLUMN properties;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE trust
    ADD COLUMN properties JSONB NOT NULL;
-- +goose StatementEnd
