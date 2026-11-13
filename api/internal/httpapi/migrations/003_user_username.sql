ALTER TABLE users ADD COLUMN IF NOT EXISTS username TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS users_shop_username_unique
    ON users (shop_id, lower(username))
    WHERE username IS NOT NULL AND btrim(username) <> '';
