CREATE TABLE IF NOT EXISTS shops (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    currency CHAR(3) NOT NULL DEFAULT 'UGX' CHECK (currency ~ '^[A-Z]{3}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE shops ADD COLUMN IF NOT EXISTS currency CHAR(3) NOT NULL DEFAULT 'UGX';
ALTER TABLE shops ADD COLUMN IF NOT EXISTS address TEXT NOT NULL DEFAULT '';
ALTER TABLE shops ADD COLUMN IF NOT EXISTS phone TEXT NOT NULL DEFAULT '';
ALTER TABLE shops ADD COLUMN IF NOT EXISTS receipt_header TEXT NOT NULL DEFAULT '';
ALTER TABLE shops ADD COLUMN IF NOT EXISTS receipt_footer TEXT NOT NULL DEFAULT '';
ALTER TABLE shops ADD COLUMN IF NOT EXISTS receipt_show_phone BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE shops ADD COLUMN IF NOT EXISTS preferences JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE shops ADD COLUMN IF NOT EXISTS is_demo BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    full_name TEXT NOT NULL CHECK (char_length(full_name) BETWEEN 1 AND 100),
    email TEXT NOT NULL,
    password_hash BYTEA,
    google_subject TEXT,
    google_picture_url TEXT,
    email_verified BOOLEAN NOT NULL DEFAULT TRUE,
    role TEXT NOT NULL CHECK (role IN ('owner', 'manager', 'cashier')),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_subject TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_picture_url TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS permissions JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE users ADD COLUMN IF NOT EXISTS username TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS users_email_unique
    ON users (lower(email));
CREATE UNIQUE INDEX IF NOT EXISTS users_shop_username_unique
    ON users (shop_id, lower(username))
    WHERE username IS NOT NULL AND btrim(username) <> '';
CREATE UNIQUE INDEX IF NOT EXISTS users_google_subject_unique
    ON users (google_subject) WHERE google_subject IS NOT NULL;
CREATE INDEX IF NOT EXISTS users_shop_id_idx ON users (shop_id);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    token_hash BYTEA PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS password_reset_tokens_user_id_idx
    ON password_reset_tokens (user_id);

CREATE TABLE IF NOT EXISTS email_verification_tokens (
    token_hash BYTEA PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS email_verification_tokens_user_id_idx
    ON email_verification_tokens (user_id);

CREATE TABLE IF NOT EXISTS products (
    id BIGSERIAL PRIMARY KEY,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    sku TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 160),
    category TEXT NOT NULL DEFAULT '',
    barcode TEXT NOT NULL DEFAULT '',
    unit TEXT NOT NULL DEFAULT 'piece',
    selling_price NUMERIC(14, 2) NOT NULL CHECK (selling_price > 0),
    cost_price NUMERIC(14, 2) NOT NULL DEFAULT 0 CHECK (cost_price >= 0),
    quantity NUMERIC(14, 3) NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    min_stock NUMERIC(14, 3) NOT NULL DEFAULT 0 CHECK (min_stock >= 0),
    image TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE products ADD COLUMN IF NOT EXISTS image TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS products_shop_active_name_idx
    ON products (shop_id, name) WHERE active = TRUE;
CREATE UNIQUE INDEX IF NOT EXISTS products_shop_sku_unique
    ON products (shop_id, lower(sku)) WHERE sku <> '';
CREATE UNIQUE INDEX IF NOT EXISTS products_shop_barcode_unique
    ON products (shop_id, barcode) WHERE barcode <> '';

CREATE TABLE IF NOT EXISTS categories (
    id BIGSERIAL PRIMARY KEY,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS categories_shop_name_unique
    ON categories (shop_id, lower(name));

CREATE TABLE IF NOT EXISTS sales (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    cashier_id UUID NOT NULL REFERENCES users(id),
    cashier_name TEXT NOT NULL,
    customer_name TEXT NOT NULL DEFAULT '',
    subtotal NUMERIC(14, 2) NOT NULL CHECK (subtotal >= 0),
    discount NUMERIC(14, 2) NOT NULL DEFAULT 0 CHECK (discount >= 0),
    total NUMERIC(14, 2) NOT NULL CHECK (total >= 0),
    payment_method TEXT NOT NULL CHECK (payment_method IN ('cash', 'mobile_money', 'card')),
    status TEXT NOT NULL DEFAULT 'completed' CHECK (status = 'completed'),
    completed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE sales ADD COLUMN IF NOT EXISTS discount NUMERIC(14, 2) NOT NULL DEFAULT 0 CHECK (discount >= 0);

CREATE INDEX IF NOT EXISTS sales_shop_completed_idx
    ON sales (shop_id, completed_at DESC);

CREATE TABLE IF NOT EXISTS sale_items (
    id BIGSERIAL PRIMARY KEY,
    sale_id UUID NOT NULL REFERENCES sales(id) ON DELETE RESTRICT,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    product_name TEXT NOT NULL,
    quantity NUMERIC(14, 3) NOT NULL CHECK (quantity > 0),
    unit_price NUMERIC(14, 2) NOT NULL CHECK (unit_price > 0),
    unit_cost NUMERIC(14, 2) NOT NULL CHECK (unit_cost >= 0),
    line_total NUMERIC(14, 2) NOT NULL CHECK (line_total > 0)
);

CREATE INDEX IF NOT EXISTS sale_items_sale_idx ON sale_items (sale_id);

CREATE TABLE IF NOT EXISTS stock_movements (
    id BIGSERIAL PRIMARY KEY,
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    user_id UUID NOT NULL REFERENCES users(id),
    quantity_delta NUMERIC(14, 3) NOT NULL CHECK (quantity_delta <> 0),
    reason TEXT NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 200),
    sale_id UUID REFERENCES sales(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS stock_movements_shop_created_idx
    ON stock_movements (shop_id, created_at DESC);

CREATE TABLE IF NOT EXISTS guest_imports (
    shop_id UUID NOT NULL REFERENCES shops(id) ON DELETE CASCADE,
    guest_device_id UUID NOT NULL,
    idempotency_key UUID NOT NULL,
    payload_hash BYTEA NOT NULL,
    response JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (shop_id, guest_device_id, idempotency_key)
);
