CREATE TABLE IF NOT EXISTS users (
    id BIGINT PRIMARY KEY,          
    username TEXT,
    first_name TEXT,
    last_name TEXT,
    state TEXT NOT NULL DEFAULT 'idle',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS access_keys (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key TEXT NOT NULL UNIQUE,
    payment_method TEXT NOT NULL, 
    is_used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS payments (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    method TEXT NOT NULL,         
    amount NUMERIC,
    currency TEXT,
    status TEXT NOT NULL,       
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);