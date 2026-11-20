-- +goose Up
CREATE TABLE otp_codes (
  id BIGSERIAL PRIMARY KEY,
  customer_id BIGINT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  code_hash BYTEA NOT NULL,           -- HMAC(secret, customer_id|kode); kode asli tidak disimpan
  attempts INT NOT NULL DEFAULT 0,
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX otp_codes_customer_idx ON otp_codes(customer_id, created_at DESC);

-- +goose Down
DROP TABLE otp_codes;
