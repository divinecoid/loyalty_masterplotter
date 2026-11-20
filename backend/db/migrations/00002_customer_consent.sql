-- +goose Up
ALTER TABLE customers ADD COLUMN consent_at TIMESTAMPTZ; -- persetujuan data pribadi (UU PDP)

-- +goose Down
ALTER TABLE customers DROP COLUMN consent_at;
