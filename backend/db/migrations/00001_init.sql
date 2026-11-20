-- +goose Up
CREATE TABLE products (
  id BIGSERIAL PRIMARY KEY,
  sku TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  points_per_unit INT NOT NULL CHECK (points_per_unit >= 0),
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE admins (
  id BIGSERIAL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'staff' CHECK (role IN ('owner','staff')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE batches (
  id BIGSERIAL PRIMARY KEY,
  product_id BIGINT NOT NULL REFERENCES products(id),
  label TEXT NOT NULL DEFAULT '',
  qty INT NOT NULL CHECK (qty > 0),
  status TEXT NOT NULL DEFAULT 'generated' CHECK (status IN ('generated','printed','active')),
  created_by BIGINT REFERENCES admins(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  printed_at TIMESTAMPTZ
);

CREATE TABLE customers (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  phone_e164 TEXT NOT NULL UNIQUE,
  points_balance INT NOT NULL DEFAULT 0 CHECK (points_balance >= 0), -- cache; ledger = sumber kebenaran
  phone_verified BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE devices (
  id BIGSERIAL PRIMARY KEY,
  customer_id BIGINT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  token_hash BYTEA NOT NULL UNIQUE,
  last_seen TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE qr_codes (
  code UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- gen_random_uuid() = UUID v4 (CSPRNG)
  batch_id BIGINT NOT NULL REFERENCES batches(id),
  seq INT NOT NULL,                                -- nomor urut dalam batch (label di stiker)
  product_id BIGINT NOT NULL REFERENCES products(id),
  points INT NOT NULL CHECK (points >= 0),
  status TEXT NOT NULL DEFAULT 'generated' CHECK (status IN ('generated','active','claimed','void')),
  claimed_by BIGINT REFERENCES customers(id),
  claimed_at TIMESTAMPTZ,
  claim_ip INET,
  claim_ua TEXT,
  CHECK ((status = 'claimed') = (claimed_by IS NOT NULL))
);
CREATE UNIQUE INDEX qr_codes_batch_seq_idx ON qr_codes(batch_id, seq);
CREATE INDEX qr_codes_status_idx ON qr_codes(status);

CREATE TABLE point_ledger (
  id BIGSERIAL PRIMARY KEY,
  customer_id BIGINT NOT NULL REFERENCES customers(id),
  delta INT NOT NULL,
  reason TEXT NOT NULL CHECK (reason IN ('scan','redeem','adjust','expire')),
  qr_code UUID UNIQUE REFERENCES qr_codes(code), -- satu QR hanya bisa menghasilkan satu entri
  ref_id BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX point_ledger_customer_idx ON point_ledger(customer_id, created_at DESC);

CREATE TABLE rewards (
  id BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  cost_points INT NOT NULL CHECK (cost_points > 0),
  stock INT NOT NULL DEFAULT 0 CHECK (stock >= 0),
  image_url TEXT,
  active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE redemptions (
  id BIGSERIAL PRIMARY KEY,
  customer_id BIGINT NOT NULL REFERENCES customers(id),
  reward_id BIGINT NOT NULL REFERENCES rewards(id),
  cost INT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','fulfilled','cancelled')),
  code TEXT NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  fulfilled_at TIMESTAMPTZ
);

CREATE TABLE audit_logs (
  id BIGSERIAL PRIMARY KEY,
  admin_id BIGINT REFERENCES admins(id),
  action TEXT NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE audit_logs, redemptions, rewards, point_ledger, qr_codes, devices, customers, batches, admins, products;
